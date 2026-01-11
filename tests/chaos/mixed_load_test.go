//go:build ci

// Package chaos contains CI-only chaos engineering tests.
// This file implements mixed load and chaos testing scenarios:
// - Mixed operation load (CREATE/CLAIM/GET interleaved)
// - Zero-stock stampede (single stock, massive concurrency)
// - Constraint violation storm (duplicate claim attempts)
// - Interleaved create-claim operations
//
// These tests verify system stability under realistic chaotic load patterns.
// Use: go test -v -race -tags ci ./tests/chaos/...
package chaos

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/model"
	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// OperationType represents the type of operation in mixed load tests
type OperationType int

const (
	// OpCreate represents a CREATE coupon operation
	OpCreate OperationType = iota
	// OpClaim represents a CLAIM coupon operation
	OpClaim
	// OpGet represents a GET coupon operation
	OpGet
)

// String returns the string representation of the operation type
func (o OperationType) String() string {
	switch o {
	case OpCreate:
		return "CREATE"
	case OpClaim:
		return "CLAIM"
	case OpGet:
		return "GET"
	default:
		return "UNKNOWN"
	}
}

// intPtr returns a pointer to the given integer value
func intPtr(i int) *int {
	return &i
}

// isServerError checks if an error indicates a server-side (500) error
func isServerError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "internal") ||
		strings.Contains(errStr, "panic")
}

// isRawDatabaseError checks if an error is a raw PostgreSQL error that leaked through
func isRawDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for PostgreSQL-specific error codes or raw constraint names
	return strings.Contains(errStr, "23505") || // unique_violation
		strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "pq:") ||
		strings.Contains(errStr, "SQLSTATE")
}

// TestMixedOperationLoad verifies system stability under mixed CREATE/CLAIM/GET operations
// AC1: All operations complete with appropriate status codes, no race conditions or data corruption
func TestMixedOperationLoad(t *testing.T) {
	cleanupTables(t)

	const (
		concurrentOps = 50
		timeout       = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Seed random for reproducibility (log the seed for debugging)
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	t.Logf("Random seed: %d (use for reproducing failures)", seed)

	// Create service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Pre-create some coupons for GET/CLAIM operations
	baseCoupons := []string{"CHAOS_BASE_1", "CHAOS_BASE_2", "CHAOS_BASE_3"}
	for _, name := range baseCoupons {
		_, err := testPool.Exec(ctx,
			"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
			name, 100, 100)
		require.NoError(t, err)
	}

	// Track results by operation type
	var createSuccess, createFail int32
	var claimSuccess, claimFail int32
	var getSuccess, getFail int32

	// Use mutex to protect rng access since rand.Rand is not thread-safe
	var rngMu sync.Mutex

	var wg sync.WaitGroup

	for i := 0; i < concurrentOps; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()

			opCtx, opCancel := context.WithTimeout(ctx, 10*time.Second)
			defer opCancel()

			// Random operation selection (weighted: 20% CREATE, 50% CLAIM, 30% GET)
			rngMu.Lock()
			roll := rng.Intn(100)
			targetCouponIdx := rng.Intn(len(baseCoupons))
			rngMu.Unlock()

			var op OperationType
			switch {
			case roll < 20:
				op = OpCreate
			case roll < 70:
				op = OpClaim
			default:
				op = OpGet
			}

			switch op {
			case OpCreate:
				couponName := fmt.Sprintf("CHAOS_NEW_%d", opID)
				err := svc.Create(opCtx, &model.CreateCouponRequest{
					Name:   couponName,
					Amount: intPtr(50),
				})
				if err == nil {
					atomic.AddInt32(&createSuccess, 1)
				} else {
					atomic.AddInt32(&createFail, 1)
				}

			case OpClaim:
				// Random coupon from base set
				couponName := baseCoupons[targetCouponIdx]
				userID := fmt.Sprintf("chaos_user_%d", opID)
				err := svc.ClaimCoupon(opCtx, userID, couponName)
				if err == nil {
					atomic.AddInt32(&claimSuccess, 1)
				} else {
					atomic.AddInt32(&claimFail, 1)
				}

			case OpGet:
				couponName := baseCoupons[targetCouponIdx]
				_, err := svc.GetByName(opCtx, couponName)
				if err == nil {
					atomic.AddInt32(&getSuccess, 1)
				} else {
					atomic.AddInt32(&getFail, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Results - CREATE: %d/%d, CLAIM: %d/%d, GET: %d/%d",
		createSuccess, createSuccess+createFail,
		claimSuccess, claimSuccess+claimFail,
		getSuccess, getSuccess+getFail)

	// Verify database consistency
	var couponCount, claimCount int
	err := testPool.QueryRow(ctx, "SELECT COUNT(*) FROM coupons").Scan(&couponCount)
	require.NoError(t, err)
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM claims").Scan(&claimCount)
	require.NoError(t, err)

	t.Logf("Database state - Coupons: %d, Claims: %d", couponCount, claimCount)

	// Verify no orphan claims (all claims reference existing coupons)
	var orphanClaims int
	err = testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM claims c
		LEFT JOIN coupons p ON c.coupon_name = p.name
		WHERE p.name IS NULL
	`).Scan(&orphanClaims)
	require.NoError(t, err)
	assert.Equal(t, 0, orphanClaims, "No orphan claims should exist")

	// Verify stock consistency (remaining_amount >= 0 for all coupons)
	var negativeStock int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM coupons WHERE remaining_amount < 0").Scan(&negativeStock)
	require.NoError(t, err)
	assert.Equal(t, 0, negativeStock, "No coupon should have negative stock")

	// Verify claim counts match stock deductions for base coupons
	for _, couponName := range baseCoupons {
		var remaining, claimsForCoupon int
		err = testPool.QueryRow(ctx,
			"SELECT remaining_amount FROM coupons WHERE name = $1",
			couponName).Scan(&remaining)
		require.NoError(t, err)

		err = testPool.QueryRow(ctx,
			"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
			couponName).Scan(&claimsForCoupon)
		require.NoError(t, err)

		// original amount (100) - claims = remaining
		expectedRemaining := 100 - claimsForCoupon
		assert.Equal(t, expectedRemaining, remaining,
			"Coupon %s: remaining_amount should match 100 - claims", couponName)
	}
}

// TestZeroStockStampede verifies single-stock coupon handling under extreme concurrency
// AC2: Exactly 1 claim succeeds, 99 fail with no stock, no 500 errors
func TestZeroStockStampede(t *testing.T) {
	cleanupTables(t)

	const (
		couponName     = "STAMPEDE_TEST"
		availableStock = 1 // Critical: single stock for stampede
		concurrentReqs = 100
		timeout        = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Setup: Create coupon with stock=1
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err)

	// Create service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Launch stampede
	var wg sync.WaitGroup
	results := make(chan error, concurrentReqs)

	for i := 0; i < concurrentReqs; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := svc.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(fmt.Sprintf("stampede_user_%d", i))
	}

	wg.Wait()
	close(results)

	// Collect results
	var successes, noStock, serverErrors, otherErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, service.ErrNoStock):
			noStock++
		case isServerError(err):
			serverErrors++
			t.Logf("SERVER ERROR (unexpected): %v", err)
		default:
			otherErrors++
			t.Logf("Other error: %v", err)
		}
	}

	t.Logf("Stampede results - Successes: %d, NoStock: %d, ServerErrors: %d, Other: %d",
		successes, noStock, serverErrors, otherErrors)

	// AC2: Exactly 1 success
	assert.Equal(t, 1, successes, "Exactly 1 claim should succeed")

	// AC2: Exactly 99 no-stock failures
	assert.Equal(t, concurrentReqs-1, noStock, "Rest should fail with no stock")

	// AC2: No 500 errors or panics
	assert.Equal(t, 0, serverErrors, "No server errors should occur")

	// Verify database state
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "remaining_amount should be exactly 0")
	assert.GreaterOrEqual(t, remaining, 0, "remaining_amount must never be negative")

	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount, "Exactly 1 claim record should exist")
}

// TestConstraintViolationStorm verifies UNIQUE constraint enforcement under concurrent duplicate claims
// AC3: Exactly 1 claim succeeds, 49 fail with 409 Conflict, no raw DB errors leak
func TestConstraintViolationStorm(t *testing.T) {
	cleanupTables(t)

	const (
		couponName     = "VIOLATION_STORM_TEST"
		availableStock = 100 // High stock to isolate constraint violations
		concurrentReqs = 50
		userID         = "storm_user" // Same user for all requests
		timeout        = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Setup: Create coupon with plenty of stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err)

	// Create service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Launch constraint violation storm
	var wg sync.WaitGroup
	results := make(chan error, concurrentReqs)

	for i := 0; i < concurrentReqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := svc.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Collect results
	var successes, alreadyClaimed, rawDBErrors, otherErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, service.ErrAlreadyClaimed):
			alreadyClaimed++
		case isRawDatabaseError(err):
			rawDBErrors++
			t.Logf("RAW DB ERROR (should be wrapped): %v", err)
		default:
			otherErrors++
			t.Logf("Other error: %v", err)
		}
	}

	t.Logf("Storm results - Successes: %d, AlreadyClaimed: %d, RawDBErrors: %d, Other: %d",
		successes, alreadyClaimed, rawDBErrors, otherErrors)

	// AC3: Exactly 1 success
	assert.Equal(t, 1, successes, "Exactly 1 claim should succeed")

	// AC3: Exactly 49 constraint violations (ErrAlreadyClaimed)
	assert.Equal(t, concurrentReqs-1, alreadyClaimed,
		"Rest should fail with ErrAlreadyClaimed")

	// AC3: No raw database errors leaked
	assert.Equal(t, 0, rawDBErrors, "No raw database errors should leak to client")

	// Verify UNIQUE constraint held: exactly 1 claim record
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		userID, couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount,
		"UNIQUE constraint must hold - exactly 1 claim record")

	// Verify remaining stock (only 1 deducted)
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, availableStock-1, remaining,
		"Only 1 stock should be deducted")
}

// TestInterleavedCreateClaim verifies correct serialization of CREATE and CLAIM operations
// AC4: Operations serialize correctly, claims fail with 404 before coupon exists, no orphan claims
func TestInterleavedCreateClaim(t *testing.T) {
	cleanupTables(t)

	const (
		couponName     = "INTERLEAVE_TEST"
		availableStock = 50
		concurrentOps  = 30
		timeout        = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Launch interleaved CREATE and CLAIM operations
	var wg sync.WaitGroup

	// Track results
	var createSuccess, createFail int32
	var claimSuccess, claimNotFound, claimNoStock, claimAlreadyClaimed, claimOther int32

	// Half try to create, half try to claim
	for i := 0; i < concurrentOps; i++ {
		wg.Add(1)
		if i%2 == 0 {
			// CREATE operation
			go func() {
				defer wg.Done()
				err := svc.Create(ctx, &model.CreateCouponRequest{
					Name:   couponName,
					Amount: intPtr(availableStock),
				})
				if err == nil {
					atomic.AddInt32(&createSuccess, 1)
				} else {
					atomic.AddInt32(&createFail, 1)
				}
			}()
		} else {
			// CLAIM operation
			go func(userID string) {
				defer wg.Done()
				err := svc.ClaimCoupon(ctx, userID, couponName)
				switch {
				case err == nil:
					atomic.AddInt32(&claimSuccess, 1)
				case errors.Is(err, service.ErrCouponNotFound):
					atomic.AddInt32(&claimNotFound, 1)
				case errors.Is(err, service.ErrNoStock):
					atomic.AddInt32(&claimNoStock, 1)
				case errors.Is(err, service.ErrAlreadyClaimed):
					atomic.AddInt32(&claimAlreadyClaimed, 1)
				default:
					atomic.AddInt32(&claimOther, 1)
				}
			}(fmt.Sprintf("interleave_user_%d", i))
		}
	}

	wg.Wait()

	t.Logf("CREATE results - Success: %d, Fail: %d", createSuccess, createFail)
	t.Logf("CLAIM results - Success: %d, NotFound: %d, NoStock: %d, AlreadyClaimed: %d, Other: %d",
		claimSuccess, claimNotFound, claimNoStock, claimAlreadyClaimed, claimOther)

	// AC4: Exactly 1 CREATE should succeed (others get ErrCouponExists)
	assert.Equal(t, int32(1), createSuccess, "Exactly 1 CREATE should succeed")

	// AC4: Claims only succeed after coupon exists
	// Some claims may have failed with NotFound (before create), which is correct

	// AC4: Verify no orphan claims
	var orphanClaims int
	err := testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM claims c
		LEFT JOIN coupons p ON c.coupon_name = p.name
		WHERE p.name IS NULL
	`).Scan(&orphanClaims)
	require.NoError(t, err)
	assert.Equal(t, 0, orphanClaims, "No orphan claims should exist")

	// Verify claims count matches successful claims
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, int(claimSuccess), claimCount,
		"Claim count should match successful claims")

	// Verify remaining stock consistency
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, availableStock-int(claimSuccess), remaining,
		"remaining_amount should reflect successful claims")
}
