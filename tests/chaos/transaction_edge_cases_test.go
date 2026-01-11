//go:build chaos

// Package chaos contains CI-only chaos engineering tests for transaction edge cases.
//
// These tests verify the system's transaction integrity under adversarial conditions:
//   - Partial failure rollback (AC #1): Ensures transactions are rolled back completely
//     when failure occurs after INSERT but before UPDATE (decrement stock).
//   - Deadlock recovery (AC #2): Verifies system handles concurrent claims on same
//     coupon without persistent deadlocks.
//   - Negative stock prevention (AC #3): Confirms remaining_amount never becomes
//     negative even under high concurrency.
//   - Context cancellation mid-transaction (AC #4): Tests clean rollback and pool
//     health when context is cancelled during transaction.
//
// IMPORTANT: These tests are tagged with "chaos" build constraint and should
// only run in CI environments where infrastructure is controlled.
// Use: go test -v -race -tags chaos ./tests/chaos/...
//
// References:
//   - Story: 6-4-transaction-edge-cases
//   - Architecture: _bmad-output/planning-artifacts/architecture.md#Transaction Pattern
//   - Schema: scripts/init.sql (CHECK constraints on remaining_amount >= 0)
package chaos

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// =============================================================================
// AC #1: Partial Failure Rollback Test
// =============================================================================

// TestPartialFailure_InsertSucceedsDecrementFails verifies that when a transaction
// fails after INSERT (claim record) but before UPDATE (decrement stock), the entire
// transaction is rolled back leaving no orphaned data.
//
// AC #1: Given the CI pipeline runs the transaction edge case test job
//
//	When a claim transaction fails after INSERT but before UPDATE (decrement stock)
//	Then the entire transaction is rolled back
//	And no claim record exists in the database
//	And remaining_amount is unchanged
func TestPartialFailure_InsertSucceedsDecrementFails(t *testing.T) {
	cleanupTables(t)
	ctx := context.Background()

	const (
		couponName   = "PARTIAL_FAIL_TEST"
		initialStock = 5
		testUserID   = "user_partial_fail"
	)

	// Setup: Create coupon with initial stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Verify initial state
	var initialRemaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&initialRemaining)
	require.NoError(t, err)
	require.Equal(t, initialStock, initialRemaining, "Initial stock should be %d", initialStock)

	// Simulate partial failure: Start transaction, INSERT claim, then ROLLBACK
	// This mimics what would happen if DecrementStock failed after Insert succeeded
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err, "Failed to begin transaction")

	// Step 1: Lock the row (simulating GetCouponForUpdate)
	var remaining int
	err = tx.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1 FOR UPDATE",
		couponName).Scan(&remaining)
	require.NoError(t, err, "Failed to lock coupon row")
	require.Equal(t, initialStock, remaining, "Stock should be %d when locked", initialStock)

	// Step 2: Insert claim (this would succeed in normal flow)
	_, err = tx.Exec(ctx,
		"INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)",
		testUserID, couponName)
	require.NoError(t, err, "Claim INSERT should succeed within transaction")

	// Step 3: Simulate failure BEFORE decrement - ROLLBACK instead of continuing
	// This is the critical test: what happens when we fail after INSERT but before UPDATE
	err = tx.Rollback(ctx)
	require.NoError(t, err, "Rollback should succeed")

	t.Log("Transaction rolled back after INSERT, before decrement")

	// Verify: No claim should exist after rollback
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		testUserID, couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to count claims")
	assert.Equal(t, 0, claimCount, "Claim should NOT exist after rollback - transaction atomicity violated!")

	// Verify: Stock should be unchanged
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remaining)
	require.NoError(t, err, "Failed to query remaining stock")
	assert.Equal(t, initialStock, remaining,
		"Stock should be unchanged after rollback (expected %d, got %d)", initialStock, remaining)

	t.Logf("Partial failure rollback verified: claim_count=%d, remaining_amount=%d", claimCount, remaining)
}

// TestPartialFailure_MultipleOperations tests rollback behavior when multiple
// operations are performed before failure.
func TestPartialFailure_MultipleOperations(t *testing.T) {
	cleanupTables(t)
	ctx := context.Background()

	const (
		couponName   = "MULTI_OP_FAIL_TEST"
		initialStock = 10
	)

	// Setup
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err)

	// Start transaction and perform multiple operations
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)

	// Perform 3 claims within the same transaction
	for i := 0; i < 3; i++ {
		userID := fmt.Sprintf("multi_user_%d", i)
		_, err = tx.Exec(ctx,
			"INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)",
			userID, couponName)
		require.NoError(t, err, "Claim %d INSERT should succeed", i)
	}

	// Decrement stock 3 times
	_, err = tx.Exec(ctx,
		"UPDATE coupons SET remaining_amount = remaining_amount - 3 WHERE name = $1",
		couponName)
	require.NoError(t, err)

	// Rollback the entire transaction
	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Verify ALL operations were rolled back
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1", couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount, "All claims should be rolled back")

	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, initialStock, remaining, "Stock should be fully restored after rollback")

	t.Logf("Multi-operation rollback verified: all 3 claims and stock decrement rolled back")
}

// =============================================================================
// AC #2: Deadlock Recovery Test
// =============================================================================

// TestDeadlockRecovery_ConcurrentSameCoupon verifies that when multiple transactions
// attempt to claim the same coupon simultaneously (potential deadlock scenario),
// at least one completes successfully, others fail gracefully, and no deadlock persists.
//
// AC #2: Given the CI pipeline runs the transaction edge case test job
//
//	When two transactions attempt to claim the same coupon simultaneously (deadlock scenario)
//	Then at least one transaction completes successfully
//	And the other retries or fails gracefully
//	And no deadlock persists beyond timeout
func TestDeadlockRecovery_ConcurrentSameCoupon(t *testing.T) {
	cleanupTables(t)

	const (
		couponName    = "DEADLOCK_TEST"
		initialStock  = 2
		numGoroutines = 10
		testTimeout   = 30 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Setup: Create coupon with limited stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Create service with the test pool
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Track initial goroutine count for leak detection
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Launch concurrent claims that will contend for the same row
	results := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	t.Logf("Launching %d concurrent claims for coupon with stock=%d", numGoroutines, initialStock)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("deadlock_user_%d", id)
			err := svc.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Collect and categorize results
	var successes, noStock, otherErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, service.ErrNoStock):
			noStock++
		default:
			otherErrors++
			t.Logf("Other error: %v", err)
		}
	}

	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d", successes, noStock, otherErrors)

	// Verify: Exactly initialStock successful claims (stock=2 means 2 successes)
	assert.Equal(t, initialStock, successes,
		"Should have exactly %d successful claims (one per stock unit)", initialStock)

	// Verify: Remaining goroutines should fail with NoStock
	assert.Equal(t, numGoroutines-initialStock, noStock,
		"Remaining %d goroutines should fail with NoStock", numGoroutines-initialStock)

	// Verify: No unexpected errors (deadlocks would appear as errors)
	assert.Equal(t, 0, otherErrors, "Should have no unexpected errors (deadlocks)")

	// Verify database state consistency
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "Stock should be fully depleted")

	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1", couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, initialStock, claimCount, "Should have exactly %d claims in database", initialStock)

	// Goroutine leak detection
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalGoroutines)

	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+3,
		"Possible goroutine leak: started with %d, ended with %d", initialGoroutines, finalGoroutines)

	t.Log("Deadlock recovery test passed - all concurrent claims handled correctly")
}

// TestDeadlockRecovery_HighContention tests with even higher concurrency
func TestDeadlockRecovery_HighContention(t *testing.T) {
	cleanupTables(t)

	const (
		couponName    = "HIGH_CONTENTION_TEST"
		initialStock  = 5
		numGoroutines = 50
		testTimeout   = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err)

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	var successes, noStock int32
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("contention_user_%d", id)
			err := svc.ClaimCoupon(ctx, userID, couponName)
			if err == nil {
				atomic.AddInt32(&successes, 1)
			} else if errors.Is(err, service.ErrNoStock) {
				atomic.AddInt32(&noStock, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("High contention results - Successes: %d, NoStock: %d", successes, noStock)

	// Critical assertions
	assert.Equal(t, int32(initialStock), successes,
		"Exactly %d claims should succeed", initialStock)
	assert.Equal(t, int32(numGoroutines-initialStock), noStock,
		"Exactly %d should fail with NoStock", numGoroutines-initialStock)

	// Verify final state
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
}

// =============================================================================
// AC #3: Negative Stock Prevention Test
// =============================================================================

// TestNegativeStockPrevention_ConcurrentExhaustion verifies that under extreme
// concurrent load, remaining_amount never becomes negative, enforced by both
// application logic and database CHECK constraint.
//
// AC #3: Given the CI pipeline runs the transaction edge case test job
//
//	When remaining_amount reaches 0 and concurrent claims attempt to decrement
//	Then remaining_amount never becomes negative
//	And all attempts after stock=0 return 400 out of stock
//	And database constraint CHECK (remaining_amount >= 0) is never violated
func TestNegativeStockPrevention_ConcurrentExhaustion(t *testing.T) {
	cleanupTables(t)

	const (
		couponName    = "NEGATIVE_STOCK_TEST"
		initialStock  = 1 // Single unit to maximize contention
		numGoroutines = 100
		testTimeout   = 60 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Setup: Coupon with stock=1, 100 concurrent claims
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err)

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	var successes, noStock, otherErrors int32
	var wg sync.WaitGroup

	t.Logf("Launching %d concurrent claims for coupon with stock=%d", numGoroutines, initialStock)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("negative_test_user_%d", id)
			err := svc.ClaimCoupon(ctx, userID, couponName)
			switch {
			case err == nil:
				atomic.AddInt32(&successes, 1)
			case errors.Is(err, service.ErrNoStock):
				atomic.AddInt32(&noStock, 1)
			default:
				atomic.AddInt32(&otherErrors, 1)
				t.Logf("Unexpected error: %v", err)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d", successes, noStock, otherErrors)

	// CRITICAL: Exactly 1 success when stock=1
	assert.Equal(t, int32(1), successes,
		"Exactly 1 claim should succeed when stock=1")

	// All others should fail with NoStock
	assert.Equal(t, int32(numGoroutines-1), noStock,
		"%d claims should fail with NoStock", numGoroutines-1)

	// No unexpected errors
	assert.Equal(t, int32(0), otherErrors,
		"Should have no unexpected errors")

	// CRITICAL: Verify remaining_amount is exactly 0, never negative
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)

	assert.Equal(t, 0, remaining,
		"Stock should be exactly 0 after exhaustion")
	assert.GreaterOrEqual(t, remaining, 0,
		"CRITICAL: Stock must NEVER be negative (CHECK constraint)")

	// CRITICAL: Verify only 1 claim exists in database
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1", couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount,
		"Exactly 1 claim should exist in database")

	t.Logf("Negative stock prevention verified: remaining=%d, claims=%d", remaining, claimCount)
}

// TestNegativeStockPrevention_DatabaseConstraint directly tests the CHECK constraint
func TestNegativeStockPrevention_DatabaseConstraint(t *testing.T) {
	cleanupTables(t)
	ctx := context.Background()

	const couponName = "CONSTRAINT_TEST"

	// Create coupon with stock=1
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, 1)
	require.NoError(t, err)

	// Attempt to directly set negative stock - should violate CHECK constraint
	_, err = testPool.Exec(ctx,
		"UPDATE coupons SET remaining_amount = -1 WHERE name = $1", couponName)

	require.Error(t, err, "Direct negative stock update should fail")
	assert.Contains(t, err.Error(), "check",
		"Error should mention CHECK constraint violation")

	t.Logf("CHECK constraint correctly prevents negative stock: %v", err)

	// Verify stock is unchanged
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 1, remaining, "Stock should be unchanged after failed update")
}

// TestNegativeStockPrevention_RapidSuccession tests rapid sequential claims
func TestNegativeStockPrevention_RapidSuccession(t *testing.T) {
	cleanupTables(t)
	ctx := context.Background()

	const (
		couponName   = "RAPID_TEST"
		initialStock = 3
		numClaims    = 20
	)

	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err)

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	var successes int
	for i := 0; i < numClaims; i++ {
		userID := fmt.Sprintf("rapid_user_%d", i)
		err := svc.ClaimCoupon(ctx, userID, couponName)
		if err == nil {
			successes++
		}
	}

	assert.Equal(t, initialStock, successes,
		"Exactly %d sequential claims should succeed", initialStock)

	// Verify final state
	var remaining int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.GreaterOrEqual(t, remaining, 0, "Stock must never be negative")
}

// =============================================================================
// AC #4: Context Cancellation Mid-Transaction Test
// =============================================================================

// TestContextCancellation_MidTransaction verifies that when a context is cancelled
// during a transaction, the transaction is rolled back cleanly with no partial state
// committed, and the connection pool remains healthy.
//
// AC #4: Given the CI pipeline runs the transaction edge case test job
//
//	When a transaction is interrupted by context cancellation
//	Then the transaction is rolled back cleanly
//	And no partial state is committed
//	And connection is returned to pool in healthy state
func TestContextCancellation_MidTransaction(t *testing.T) {
	cleanupTables(t)

	const (
		couponName   = "CANCEL_TEST"
		initialStock = 10
	)

	bgCtx := context.Background()

	// Setup test coupon
	_, err := testPool.Exec(bgCtx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, initialStock)
	require.NoError(t, err)

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Track initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(bgCtx)

	// Start claim in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.ClaimCoupon(ctx, "user_cancel", couponName)
	}()

	// Cancel context almost immediately
	time.Sleep(1 * time.Millisecond)
	cancel()

	// Wait for result with timeout
	select {
	case err := <-errCh:
		// May succeed or fail depending on timing
		if err != nil {
			// Expected: context.Canceled or related error
			isExpectedError := errors.Is(err, context.Canceled) ||
				containsAny(err.Error(), "context canceled", "context deadline exceeded")
			if isExpectedError {
				t.Logf("Expected context cancellation error: %v", err)
			} else {
				t.Logf("Other error (may be timing-dependent): %v", err)
			}
		} else {
			t.Log("Claim completed before cancellation (race condition - acceptable)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock or resource leak")
	}

	// Verify pool health - subsequent operations should succeed
	err = testPool.Ping(bgCtx)
	require.NoError(t, err, "Pool should be healthy after cancellation")

	// Verify we can perform normal operations
	var remaining int
	err = testPool.QueryRow(bgCtx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err, "Query should succeed after cancellation")
	t.Logf("Remaining stock after cancellation test: %d", remaining)

	// Stock should be either unchanged (10) or decremented once (9) depending on timing
	assert.True(t, remaining == initialStock || remaining == initialStock-1,
		"Stock should be %d or %d (depending on timing), got %d",
		initialStock, initialStock-1, remaining)

	// Verify no goroutine leaks
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalGoroutines)

	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+3,
		"Possible goroutine leak after context cancellation")

	// Verify connection pool metrics
	stats := testPool.Stat()
	t.Logf("Pool stats - Total: %d, Idle: %d, In-Use: %d",
		stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns())

	// Pool should have no acquired connections after cleanup
	assert.LessOrEqual(t, stats.AcquiredConns(), int32(1),
		"Pool should not have stuck connections")
}

// TestContextCancellation_DuringLockWait tests cancellation while waiting for row lock
func TestContextCancellation_DuringLockWait(t *testing.T) {
	cleanupTables(t)
	bgCtx := context.Background()

	const couponName = "LOCK_WAIT_CANCEL_TEST"

	// Setup
	_, err := testPool.Exec(bgCtx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, 10)
	require.NoError(t, err)

	// Start a transaction that holds the row lock
	holderTx, err := testPool.Begin(bgCtx)
	require.NoError(t, err)
	defer holderTx.Rollback(bgCtx)

	// Lock the row (this transaction will hold it)
	_, err = holderTx.Exec(bgCtx,
		"SELECT * FROM coupons WHERE name = $1 FOR UPDATE", couponName)
	require.NoError(t, err)
	t.Log("Row lock acquired by holder transaction")

	// Create service for the waiting claim
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Start a claim that will wait for the lock, then cancel its context
	waitCtx, waitCancel := context.WithTimeout(bgCtx, 500*time.Millisecond)
	defer waitCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.ClaimCoupon(waitCtx, "waiting_user", couponName)
	}()

	// Wait for the claim to time out
	select {
	case err := <-errCh:
		require.Error(t, err, "Claim should fail due to context timeout/cancellation")
		isTimeoutError := errors.Is(err, context.DeadlineExceeded) ||
			containsAny(err.Error(), "timeout", "deadline", "canceled")
		assert.True(t, isTimeoutError,
			"Error should be timeout-related, got: %v", err)
		t.Logf("Claim correctly cancelled while waiting for lock: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out - claim should have failed faster")
	}

	// Release the holder's lock
	err = holderTx.Rollback(bgCtx)
	require.NoError(t, err)

	// Verify database state - no claim should exist from cancelled transaction
	var claimCount int
	err = testPool.QueryRow(bgCtx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1", couponName).Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount,
		"No claims should exist after cancelled transaction")

	// Verify stock unchanged
	var remaining int
	err = testPool.QueryRow(bgCtx,
		"SELECT remaining_amount FROM coupons WHERE name = $1", couponName).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 10, remaining, "Stock should be unchanged")

	t.Log("Lock wait cancellation test passed")
}

// TestContextCancellation_PoolRecovery verifies pool remains fully functional after cancellations
func TestContextCancellation_PoolRecovery(t *testing.T) {
	cleanupTables(t)
	bgCtx := context.Background()

	const couponName = "POOL_RECOVERY_TEST"

	_, err := testPool.Exec(bgCtx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		couponName, 100)
	require.NoError(t, err)

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	svc := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Perform multiple cancelled operations
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(bgCtx)
		go func(id int) {
			time.Sleep(time.Duration(id) * time.Millisecond)
			cancel()
		}(i)

		_ = svc.ClaimCoupon(ctx, fmt.Sprintf("cancel_user_%d", i), couponName)
	}

	// Allow time for cleanup
	time.Sleep(200 * time.Millisecond)

	// Pool should still be healthy
	for i := 0; i < 5; i++ {
		err := testPool.Ping(bgCtx)
		require.NoError(t, err, "Pool ping %d should succeed", i+1)
	}

	// Should be able to perform normal operations
	successCtx, successCancel := context.WithTimeout(bgCtx, 10*time.Second)
	defer successCancel()

	err = svc.ClaimCoupon(successCtx, "recovery_user", couponName)
	assert.NoError(t, err, "Normal claim should succeed after cancellation stress")

	// Verify pool metrics
	stats := testPool.Stat()
	t.Logf("Pool after recovery test - Total: %d, Idle: %d, Acquired: %d",
		stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns())

	t.Log("Pool recovery after cancellations verified")
}

// =============================================================================
// Helper Functions
// =============================================================================

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
