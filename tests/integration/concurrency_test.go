//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// TestConcurrentClaimLastStock tests AC4: Race Condition Prevention for Last Stock
// Given two concurrent claim requests for the last available coupon
// When both requests attempt to claim simultaneously
// Then exactly one succeeds with 200/201
// And exactly one fails with 400 (out of stock)
// And remaining_amount is exactly 0 (not negative)
func TestConcurrentClaimLastStock(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create coupon with remaining_amount = 1
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"LAST_STOCK_TEST", 10, 1)
	require.NoError(t, err)

	// Setup service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Two concurrent claims
	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, "LAST_STOCK_TEST")
			results <- err
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: exactly 1 success, exactly 1 ErrNoStock
	var successes, noStocks, otherErrors int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, service.ErrNoStock) {
			noStocks++
		} else {
			otherErrors++
			t.Logf("Unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successes, "Exactly one claim should succeed")
	assert.Equal(t, 1, noStocks, "Exactly one claim should fail with ErrNoStock")
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify database state: remaining_amount = 0 (not negative)
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"LAST_STOCK_TEST").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0, not negative")

	// Verify: exactly 1 claim record created
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		"LAST_STOCK_TEST").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount, "Exactly 1 claim record should exist")
}

// TestConcurrentClaimsSameUser tests AC3: Unique Constraint Violation Handling
// Given the claims table unique constraint on (user_id, coupon_name)
// When a duplicate claim is attempted concurrently
// Then the database constraint violation is caught
// And the transaction is rolled back
// And 409 Conflict is returned
func TestConcurrentClaimsSameUser(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create coupon with enough stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"SAME_USER_TEST", 100, 100)
	require.NoError(t, err)

	// Setup service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: 10 concurrent claims by the SAME user
	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, "same_user", "SAME_USER_TEST")
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Verify: exactly 1 success, 9 ErrAlreadyClaimed
	var successes, alreadyClaimed, otherErrors int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, service.ErrAlreadyClaimed) {
			alreadyClaimed++
		} else {
			otherErrors++
			t.Logf("Unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successes, "Exactly one claim should succeed")
	assert.Equal(t, 9, alreadyClaimed, "Nine claims should fail with ErrAlreadyClaimed")
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify database state: exactly 1 claim
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		"same_user", "SAME_USER_TEST").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount, "Exactly 1 claim record should exist")

	// Verify remaining_amount decremented by 1
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"SAME_USER_TEST").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 99, remainingAmount, "remaining_amount should be 99")
}

// TestSelectForUpdateSerialization tests AC5: SELECT FOR UPDATE Serialization
// Given the SELECT FOR UPDATE implementation
// When multiple transactions attempt to lock the same coupon row
// Then they are serialized (one waits for the other)
// And no deadlocks occur under normal operation
func TestSelectForUpdateSerialization(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create coupon with stock = number of concurrent requests
	concurrentRequests := 5
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"SERIALIZATION_TEST", concurrentRequests, concurrentRequests)
	require.NoError(t, err)

	// Setup service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: N concurrent claims from different users
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, "SERIALIZATION_TEST")
			results <- err
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: all claims succeed (enough stock for all)
	var successes, errs int
	for err := range results {
		if err == nil {
			successes++
		} else {
			errs++
			t.Logf("Unexpected error: %v", err)
		}
	}

	assert.Equal(t, concurrentRequests, successes, "All claims should succeed")
	assert.Equal(t, 0, errs, "No claims should fail")

	// Verify database state: remaining_amount = 0
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"SERIALIZATION_TEST").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be 0")

	// Verify: N claim records created
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		"SERIALIZATION_TEST").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, concurrentRequests, claimCount, "N claim records should exist")
}

// TestFlashSaleScenario tests a realistic flash sale scenario
// with more concurrent requests than available stock
func TestFlashSaleScenario(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Flash sale with 5 coupons, 20 concurrent requests
	availableStock := 5
	concurrentRequests := 20

	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"FLASH_SALE", availableStock, availableStock)
	require.NoError(t, err)

	// Setup service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: 20 concurrent claims from different users
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, "FLASH_SALE")
			results <- err
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: exactly 5 successes, 15 ErrNoStock
	var successes, noStocks, otherErrors int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, service.ErrNoStock) {
			noStocks++
		} else {
			otherErrors++
			t.Logf("Unexpected error: %v", err)
		}
	}

	assert.Equal(t, availableStock, successes, "Exactly %d claims should succeed", availableStock)
	assert.Equal(t, concurrentRequests-availableStock, noStocks, "Exactly %d claims should fail with ErrNoStock", concurrentRequests-availableStock)
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify database state: remaining_amount = 0 (not negative)
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"FLASH_SALE").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")
	assert.GreaterOrEqual(t, remainingAmount, 0, "remaining_amount should never be negative")

	// Verify: exactly 5 claim records created
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		"FLASH_SALE").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, availableStock, claimCount, "Exactly %d claim records should exist", availableStock)
}

// TestTransactionRollbackOnFailure tests AC2: Transaction Rollback on Failure
// Given a claim operation fails at any step
// When an error occurs
// Then the entire transaction is rolled back
// And no partial changes are persisted
func TestTransactionRollbackOnFailure_OutOfStock(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create coupon with 0 stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"ZERO_STOCK", 10, 0)
	require.NoError(t, err)

	// Setup service
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Attempt claim on zero stock
	err = couponService.ClaimCoupon(ctx, "user_001", "ZERO_STOCK")

	// Verify: ErrNoStock returned
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNoStock), "Should return ErrNoStock")

	// Verify: No claim record created (transaction rolled back)
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		"user_001", "ZERO_STOCK").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount, "No claim record should exist after rollback")

	// Verify: remaining_amount unchanged
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"ZERO_STOCK").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be unchanged")
}
