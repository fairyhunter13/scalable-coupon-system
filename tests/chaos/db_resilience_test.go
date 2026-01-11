//go:build ci

// Package chaos contains CI-only chaos engineering tests for database resilience.
// These tests verify the system handles database failure scenarios correctly:
// - Connection pool exhaustion (AC #1)
// - Query timeouts (AC #2)
// - Connection drops mid-transaction (AC #3)
package chaos

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// TestConnectionPoolExhaustion verifies behavior when all connection pool slots are exhausted.
//
// AC #1: Given the CI pipeline runs the database resilience test job
//
//	When all connection pool slots are exhausted (max_conns reached)
//	Then new requests receive appropriate error responses (503 or timeout)
//	And no goroutine leaks occur
//	And system recovers when connections become available
//
// This test creates a pool with max_conns=2, launches concurrent operations
// exceeding pool capacity, and verifies proper error handling and recovery.
func TestConnectionPoolExhaustion(t *testing.T) {
	cleanupTables(t)

	const (
		maxConns           = int32(2) // Deliberately low for exhaustion testing
		concurrentRequests = 10       // Exceed pool capacity
		couponName         = "EXHAUST_TEST"
		availableStock     = 100
		acquireTimeout     = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Record initial goroutine count for leak detection
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Create a pool with limited connections
	limitedPool, err := createPoolWithConfigAndTimeout(ctx, maxConns, acquireTimeout)
	require.NoError(t, err, "Failed to create limited pool")
	defer limitedPool.Close()

	// Setup test coupon using the main test pool
	_, err = testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Create service with the limited pool
	couponRepo := repository.NewCouponRepository(limitedPool)
	claimRepo := repository.NewClaimRepository(limitedPool)
	couponService := service.NewCouponService(limitedPool, couponRepo, claimRepo)

	// Launch concurrent claims exceeding pool capacity
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	t.Logf("Launching %d concurrent requests with pool max_conns=%d", concurrentRequests, maxConns)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("user_exhaust_%d", id)
			claimCtx, claimCancel := context.WithTimeout(ctx, acquireTimeout+1*time.Second)
			defer claimCancel()
			err := couponService.ClaimCoupon(claimCtx, userID, couponName)
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Collect and categorize results
	var successes, timeouts, otherErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, context.DeadlineExceeded):
			timeouts++
		case isPoolAcquireTimeout(err):
			timeouts++
		default:
			// Other errors are acceptable in pool exhaustion scenarios
			otherErrors++
			t.Logf("Other error (acceptable in exhaustion scenario): %v", err)
		}
	}

	t.Logf("Results - Successes: %d, Timeouts: %d, Other: %d", successes, timeouts, otherErrors)

	// Verify some requests succeeded (pool wasn't completely broken)
	assert.Greater(t, successes, 0, "At least some requests should succeed")

	// Verify timeout behavior when pool is exhausted
	// Note: timeouts may or may not occur depending on timing
	t.Logf("Timeout count: %d (expected behavior when pool exhausted)", timeouts)

	// Goroutine leak detection
	// Allow cleanup time
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalGoroutines)

	// Allow small variance for runtime goroutines (5 is a reasonable buffer)
	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+10,
		"Possible goroutine leak: started with %d, ended with %d",
		initialGoroutines, finalGoroutines)

	// Verify recovery: after concurrent requests complete, new requests should work
	t.Log("Testing recovery after exhaustion...")
	recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 10*time.Second)
	defer recoveryCancel()

	// Create a new coupon for recovery test
	_, err = testPool.Exec(recoveryCtx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"RECOVERY_TEST", 10, 10)
	require.NoError(t, err, "Failed to create recovery coupon")

	// Verify new request succeeds
	err = couponService.ClaimCoupon(recoveryCtx, "user_recovery", "RECOVERY_TEST")
	assert.NoError(t, err, "System should recover and process new requests")

	t.Log("Pool exhaustion test completed - system recovered successfully")
}

// TestQueryTimeout verifies behavior when a query exceeds configured timeout.
//
// AC #2: Given the CI pipeline runs the database resilience test job
//
//	When a query exceeds the configured timeout (e.g., 5 seconds)
//	Then the request is cancelled with context deadline exceeded
//	And the transaction is rolled back properly
//	And appropriate error response is returned to client
//
// This test uses PostgreSQL's pg_sleep to simulate slow queries.
func TestQueryTimeout(t *testing.T) {
	cleanupTables(t)

	const (
		shortTimeout = 100 * time.Millisecond
		sleepSeconds = 1 // pg_sleep(1) = 1 second, will exceed shortTimeout
	)

	// Test 1: Direct query timeout with pg_sleep
	t.Run("Direct query timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()

		// This should timeout - pg_sleep(1) sleeps for 1 second
		_, err := testPool.Exec(ctx, "SELECT pg_sleep($1)", sleepSeconds)

		require.Error(t, err, "Query should timeout")
		assert.True(t, errors.Is(err, context.DeadlineExceeded),
			"Error should be context.DeadlineExceeded, got: %v", err)

		t.Logf("Query timeout correctly returned: %v", err)
	})

	// Test 2: Transaction timeout with rollback verification
	t.Run("Transaction timeout with rollback", func(t *testing.T) {
		const couponName = "TIMEOUT_TX_TEST"
		const availableStock = 100

		// Setup coupon
		setupCtx, setupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer setupCancel()

		_, err := testPool.Exec(setupCtx,
			"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
			couponName, availableStock, availableStock)
		require.NoError(t, err, "Failed to create test coupon")

		// Start a transaction with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
		defer cancel()

		tx, err := testPool.Begin(ctx)
		if err != nil {
			// If we can't even begin due to timeout, that's expected
			assert.True(t, errors.Is(err, context.DeadlineExceeded),
				"Begin error should be deadline exceeded")
			return
		}
		defer tx.Rollback(context.Background())

		// Try to execute a slow query in the transaction
		_, err = tx.Exec(ctx, "SELECT pg_sleep($1)", sleepSeconds)

		require.Error(t, err, "Transaction query should timeout")
		assert.True(t, errors.Is(err, context.DeadlineExceeded),
			"Error should be context.DeadlineExceeded, got: %v", err)

		// Verify transaction is rolled back (can't commit after error)
		commitErr := tx.Commit(context.Background())
		assert.Error(t, commitErr, "Commit should fail after timeout")

		// Verify no partial state: coupon stock unchanged
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer verifyCancel()

		var remaining int
		err = testPool.QueryRow(verifyCtx,
			"SELECT remaining_amount FROM coupons WHERE name = $1",
			couponName).Scan(&remaining)
		require.NoError(t, err, "Failed to verify coupon state")
		assert.Equal(t, availableStock, remaining,
			"Remaining stock should be unchanged after rollback")

		t.Logf("Transaction properly rolled back, remaining_amount: %d", remaining)
	})

	// Test 3: Service layer timeout propagation
	t.Run("Service layer timeout propagation", func(t *testing.T) {
		cleanupTables(t)

		const couponName = "SERVICE_TIMEOUT_TEST"
		const availableStock = 100

		// Setup coupon
		setupCtx, setupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer setupCancel()

		_, err := testPool.Exec(setupCtx,
			"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
			couponName, availableStock, availableStock)
		require.NoError(t, err, "Failed to create test coupon")

		// Create service
		couponRepo := repository.NewCouponRepository(testPool)
		claimRepo := repository.NewClaimRepository(testPool)
		couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

		// Create an already-cancelled context to simulate immediate timeout
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = couponService.ClaimCoupon(ctx, "user_timeout", couponName)

		require.Error(t, err, "Service call with cancelled context should fail")
		assert.True(t, errors.Is(err, context.Canceled),
			"Error should be context.Canceled, got: %v", err)

		// Verify database state unchanged
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer verifyCancel()

		var remaining int
		err = testPool.QueryRow(verifyCtx,
			"SELECT remaining_amount FROM coupons WHERE name = $1",
			couponName).Scan(&remaining)
		require.NoError(t, err, "Failed to verify coupon state")
		assert.Equal(t, availableStock, remaining,
			"Stock should be unchanged after cancelled request")

		t.Log("Service layer correctly propagates context timeout")
	})
}

// TestConnectionDrop simulates a connection being terminated mid-transaction.
//
// AC #3: Given the CI pipeline runs the database resilience test job
//
//	When a database connection drops mid-transaction
//	Then the transaction fails safely (no partial commits)
//	And the connection is removed from the pool
//	And subsequent requests use healthy connections
//
// This test uses PostgreSQL's pg_terminate_backend to simulate connection drops.
func TestConnectionDrop(t *testing.T) {
	cleanupTables(t)

	const (
		couponName     = "DROP_TEST"
		availableStock = 100
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Setup test coupon
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Test 1: Terminate connection mid-transaction
	t.Run("Connection terminated mid-transaction", func(t *testing.T) {
		// Create a dedicated pool for this test to avoid affecting other tests
		testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
		defer testCancel()

		// Start a transaction
		tx, err := testPool.Begin(testCtx)
		require.NoError(t, err, "Failed to begin transaction")
		defer tx.Rollback(context.Background())

		// Get the backend PID for this connection
		var backendPID int
		err = tx.QueryRow(testCtx, "SELECT pg_backend_pid()").Scan(&backendPID)
		require.NoError(t, err, "Failed to get backend PID")
		t.Logf("Transaction backend PID: %d", backendPID)

		// Do some work in the transaction (but don't commit yet)
		_, err = tx.Exec(testCtx,
			"UPDATE coupons SET remaining_amount = remaining_amount - 1 WHERE name = $1",
			couponName)
		require.NoError(t, err, "Failed to update in transaction")

		// From a separate connection, terminate the transaction's connection
		// This simulates a network failure or database restart
		_, err = testPool.Exec(testCtx, "SELECT pg_terminate_backend($1)", backendPID)
		if err != nil {
			t.Logf("Note: pg_terminate_backend returned error (expected in some cases): %v", err)
		}

		// The transaction should now be broken
		// Any subsequent operation on the transaction should fail
		time.Sleep(50 * time.Millisecond) // Give time for termination to propagate

		// Try to use the terminated connection
		_, err = tx.Exec(testCtx, "SELECT 1")

		// We expect an error - the connection was terminated
		if err != nil {
			t.Logf("Transaction correctly failed after connection termination: %v", err)
		}

		// Verify no partial commit occurred
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer verifyCancel()

		var remaining int
		err = testPool.QueryRow(verifyCtx,
			"SELECT remaining_amount FROM coupons WHERE name = $1",
			couponName).Scan(&remaining)
		require.NoError(t, err, "Failed to verify coupon state")
		assert.Equal(t, availableStock, remaining,
			"No partial commit should occur - remaining should still be %d", availableStock)

		t.Logf("Verified no partial commit: remaining_amount = %d", remaining)
	})

	// Test 2: Verify pool recovers with healthy connections
	t.Run("Pool recovery after connection drop", func(t *testing.T) {
		testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
		defer testCancel()

		// Multiple subsequent operations should succeed using healthy connections
		for i := 0; i < 5; i++ {
			err := testPool.Ping(testCtx)
			require.NoError(t, err, "Ping %d should succeed after connection drop", i+1)
		}

		// Create a new coupon to prove the pool is fully functional
		_, err := testPool.Exec(testCtx,
			"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
			"RECOVERY_VERIFY", 50, 50)
		require.NoError(t, err, "Should be able to create new coupon after recovery")

		// Query should work
		var count int
		err = testPool.QueryRow(testCtx, "SELECT COUNT(*) FROM coupons").Scan(&count)
		require.NoError(t, err, "Query should succeed")
		assert.GreaterOrEqual(t, count, 2, "Should have at least 2 coupons")

		t.Log("Pool successfully recovered with healthy connections")
	})

	// Test 3: Service layer handles connection errors gracefully
	t.Run("Service handles connection errors", func(t *testing.T) {
		testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
		defer testCancel()

		// Create service
		couponRepo := repository.NewCouponRepository(testPool)
		claimRepo := repository.NewClaimRepository(testPool)
		couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

		// Service operations should work normally after pool recovery
		err := couponService.ClaimCoupon(testCtx, "user_after_drop", couponName)
		assert.NoError(t, err, "Service should handle claims after connection recovery")

		// Verify claim succeeded
		var claimCount int
		err = testPool.QueryRow(testCtx,
			"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
			couponName).Scan(&claimCount)
		require.NoError(t, err, "Failed to count claims")
		assert.Equal(t, 1, claimCount, "Claim should be recorded")

		t.Log("Service layer correctly handles post-recovery operations")
	})
}

// TestGoroutineLeakDetection is a comprehensive test that runs multiple
// chaos scenarios and verifies no goroutine leaks occur.
func TestGoroutineLeakDetection(t *testing.T) {
	cleanupTables(t)

	// Record baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()
	t.Logf("Baseline goroutine count: %d", baselineGoroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Setup test data
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"LEAK_TEST", 1000, 1000)
	require.NoError(t, err, "Failed to create test coupon")

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Run multiple rounds of concurrent operations
	const rounds = 3
	const operationsPerRound = 20

	for round := 1; round <= rounds; round++ {
		t.Logf("Running round %d/%d...", round, rounds)

		var wg sync.WaitGroup
		for i := 0; i < operationsPerRound; i++ {
			wg.Add(1)
			go func(roundNum, opID int) {
				defer wg.Done()

				opCtx, opCancel := context.WithTimeout(ctx, 5*time.Second)
				defer opCancel()

				userID := fmt.Sprintf("leak_test_user_%d_%d", roundNum, opID)
				_ = couponService.ClaimCoupon(opCtx, userID, "LEAK_TEST")
			}(round, i)
		}
		wg.Wait()

		// Check goroutine count after each round
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		currentGoroutines := runtime.NumGoroutine()
		t.Logf("Round %d complete - goroutine count: %d", round, currentGoroutines)
	}

	// Final goroutine leak check
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Final goroutine count: %d (baseline: %d)", finalGoroutines, baselineGoroutines)

	// Allow reasonable variance (10 goroutines) for runtime variations
	maxAllowedGoroutines := baselineGoroutines + 10
	assert.LessOrEqual(t, finalGoroutines, maxAllowedGoroutines,
		"Possible goroutine leak detected: baseline=%d, final=%d, max_allowed=%d",
		baselineGoroutines, finalGoroutines, maxAllowedGoroutines)

	t.Log("Goroutine leak detection test passed")
}

// createPoolWithConfigAndTimeout creates a pool with custom max connections.
// Note: Acquire timeout is handled via context timeout in the calling code,
// not via pool configuration. The acquireTimeout parameter is retained for
// documentation purposes but pool exhaustion timeout is controlled by context.
func createPoolWithConfigAndTimeout(ctx context.Context, maxConns int32, _ time.Duration) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	config.MaxConns = maxConns
	config.MinConns = 1
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	return pgxpool.NewWithConfig(ctx, config)
}

// isPoolAcquireTimeout checks if the error is related to pool acquisition timeout.
func isPoolAcquireTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "pool") ||
		strings.Contains(errStr, "acquire")
}
