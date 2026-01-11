// Package stress contains stress tests for concurrency safety validation.
// These tests verify the system handles high-concurrency scenarios correctly,
// specifically the Flash Sale (multiple users) and Double Dip (same user) attack patterns.
package stress

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// TestDoubleDip tests a double dip attack scenario with 10 concurrent requests
// from the SAME user attempting to claim a coupon.
//
// This validates NFR2: System handles 10 concurrent same-user requests with exactly 1 success.
// The UNIQUE(user_id, coupon_name) constraint in the database prevents duplicate claims.
//
// Story Acceptance Criteria (from story 4-4):
//
//	AC #1: Given a coupon "DOUBLE_TEST" with amount=100
//	       And a single user "user_greedy"
//	       When 10 concurrent goroutines attempt to claim for "user_greedy" simultaneously
//	       Then exactly 1 claim succeeds (200/201 response)
//	       And exactly 9 claims fail (409 Conflict - ErrAlreadyClaimed)
//	       And remaining_amount is exactly 99
//	       And claimed_by contains exactly ["user_greedy"]
//
//	AC #2: Test passes consistently via `go test ./tests/stress/... -run TestDoubleDip -v`
//
//	AC #3: Test passes 10 consecutive runs without flakiness
//
//	AC #4: Only one claim record exists for (user_greedy, DOUBLE_TEST) in the database
//
// Design Note: Stock is set to 100 (not 1) to ensure all 9 failures are due to
// ErrAlreadyClaimed (UNIQUE constraint violation), NOT ErrNoStock. This isolates
// the double-dip prevention mechanism from stock exhaustion behavior.
func TestDoubleDip(t *testing.T) {
	cleanupTables(t)

	// Test configuration
	const (
		couponName         = "DOUBLE_TEST"
		availableStock     = 100
		concurrentRequests = 10
		userID             = "user_greedy"
		timeout            = 30 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	t.Logf("Starting double dip stress test: %d concurrent same-user requests", concurrentRequests)

	// Setup: Create coupon "DOUBLE_TEST" with amount=100, remaining_amount=100
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Launch 10 concurrent goroutines using sync.WaitGroup
	// ALL goroutines claim with the SAME user_id "user_greedy"
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests) // Buffered channel for all results

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Collect and count results
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

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, AlreadyClaimed: %d, Other: %d", successes, alreadyClaimed, otherErrors)
	t.Logf("Execution time: %v", executionTime)

	// AC1: Assert exactly 1 success
	assert.Equal(t, 1, successes, "Exactly one claim should succeed")

	// AC1: Assert exactly 9 ErrAlreadyClaimed failures
	assert.Equal(t, concurrentRequests-1, alreadyClaimed,
		"Exactly %d claims should fail with ErrAlreadyClaimed", concurrentRequests-1)

	// Assert 0 other errors (no ErrNoStock expected - plenty of stock available)
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// AC1: Query database to verify remaining_amount = 99 (only 1 successful claim)
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remainingAmount)
	require.NoError(t, err, "Failed to query remaining_amount")
	assert.Equal(t, availableStock-1, remainingAmount,
		"remaining_amount should be %d (original %d minus 1 successful claim)",
		availableStock-1, availableStock)

	// AC4: Query database to verify exactly 1 claim exists for user_greedy
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		userID, couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")
	assert.Equal(t, 1, claimCount,
		"Exactly 1 claim record should exist for %s", userID)

	// AC4: Verify the claim record has the correct user_id
	var claimedUserID string
	err = testPool.QueryRow(ctx,
		"SELECT user_id FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimedUserID)
	require.NoError(t, err, "Failed to query claimed user")
	assert.Equal(t, userID, claimedUserID,
		"Claim record should belong to %s", userID)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d", remainingAmount, claimCount)

	// AC #2: Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)

	// Performance regression check: under normal conditions, 10 concurrent claims
	// should complete well under 5 seconds (typically <100ms with local Docker)
	const performanceThreshold = 5 * time.Second
	assert.Less(t, executionTime, performanceThreshold,
		"Performance regression: test took %v, expected under %v", executionTime, performanceThreshold)

	// AC #1: Verify claimed_by contains exactly ["user_greedy"] via service layer
	// This validates the full API response format matches expectations
	couponResp, err := couponService.GetByName(ctx, couponName)
	require.NoError(t, err, "Failed to get coupon response")
	require.NotNil(t, couponResp, "Coupon response should not be nil")
	assert.Equal(t, []string{userID}, couponResp.ClaimedBy,
		"claimed_by should contain exactly [%s]", userID)

	t.Logf("claimed_by verification: %v", couponResp.ClaimedBy)
}

// TestDoubleDip_ContextCancellation verifies graceful handling when context is
// canceled during concurrent claim operations. This ensures no goroutine leaks
// or resource exhaustion occur under abnormal termination conditions.
func TestDoubleDip_ContextCancellation(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "CANCEL_TEST"
		availableStock     = 100
		concurrentRequests = 10
		userID             = "user_cancel"
	)

	// Create a context that we'll cancel almost immediately
	ctx, cancel := context.WithCancel(context.Background())

	// Setup coupon
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer setupCancel()

	_, err := testPool.Exec(setupCtx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Launch concurrent claims
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}()
	}

	// Cancel context after a tiny delay to ensure some goroutines have started
	time.Sleep(1 * time.Millisecond)
	cancel()

	// Wait for all goroutines to complete (they should exit gracefully)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	// Verify goroutines complete within reasonable time (no leaks/hangs)
	select {
	case <-done:
		t.Log("All goroutines completed after context cancellation")
	case <-time.After(10 * time.Second):
		t.Fatal("Goroutines did not complete within 10 seconds - possible goroutine leak")
	}

	// Count results - we expect a mix of successes, already-claimed, and context errors
	var successes, alreadyClaimed, contextErrors, otherErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, service.ErrAlreadyClaimed):
			alreadyClaimed++
		case errors.Is(err, context.Canceled):
			contextErrors++
		default:
			// Context cancellation may surface as various wrapped errors
			if ctx.Err() != nil {
				contextErrors++
			} else {
				otherErrors++
				t.Logf("Unexpected error: %v", err)
			}
		}
	}

	t.Logf("Results after cancellation - Successes: %d, AlreadyClaimed: %d, ContextErrors: %d, Other: %d",
		successes, alreadyClaimed, contextErrors, otherErrors)

	// Key assertion: at most 1 success (same user can only claim once)
	assert.LessOrEqual(t, successes, 1,
		"At most 1 claim should succeed for the same user")

	// Verify database consistency: if any success, exactly 1 claim record
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verifyCancel()

	var claimCount int
	err = testPool.QueryRow(verifyCtx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		userID, couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")

	if successes > 0 {
		assert.Equal(t, 1, claimCount, "If any success, exactly 1 claim record should exist")
	} else {
		assert.Equal(t, 0, claimCount, "If no success, no claim record should exist")
	}

	t.Logf("Database state after cancellation - claim_count: %d", claimCount)
}
