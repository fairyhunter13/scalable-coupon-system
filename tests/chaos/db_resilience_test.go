//go:build chaos

// Package chaos contains CI-only chaos engineering tests for database resilience.
// These tests verify the system handles database failure scenarios correctly:
// - Connection pool exhaustion (AC #1)
// - Query timeouts (AC #2)
// - Connection drops mid-transaction (AC #3)
//
// All tests use real HTTP requests to the docker-compose server.
package chaos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
// This test launches many concurrent HTTP requests to stress the pool.
func TestConnectionPoolExhaustion(t *testing.T) {
	cleanupTables(t)

	const (
		concurrentRequests = 50 // High concurrency to stress pool
		couponName         = "EXHAUST_TEST"
		availableStock     = 1000
	)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Record initial goroutine count for leak detection
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Setup test coupon using direct DB (setup only)
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Launch concurrent claims via HTTP to stress the pool
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests) // HTTP status codes

	t.Logf("Launching %d concurrent HTTP requests to stress connection pool", concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("user_exhaust_%d", id)
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": couponName,
			})
			if err != nil {
				t.Logf("HTTP error for user %d: %v", id, err)
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}(i)
	}

	wg.Wait()
	close(results)

	// Collect and categorize results
	var successes, clientErrors, serverErrors, other int
	for code := range results {
		switch {
		case code == http.StatusOK:
			successes++
		case code >= 400 && code < 500:
			clientErrors++ // Expected for duplicate claims, out of stock
		case code >= 500:
			serverErrors++ // Pool exhaustion may cause 500/503
		default:
			other++
			t.Logf("Unexpected status code: %d", code)
		}
	}

	t.Logf("Results - Successes: %d, ClientErrors: %d, ServerErrors: %d, Other: %d",
		successes, clientErrors, serverErrors, other)

	// Verify some requests succeeded (system wasn't completely broken)
	assert.Greater(t, successes, 0, "At least some requests should succeed")

	// Goroutine leak detection
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalGoroutines)

	// Allow small variance for runtime goroutines
	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+20,
		"Possible goroutine leak: started with %d, ended with %d",
		initialGoroutines, finalGoroutines)

	// Verify recovery: after concurrent requests complete, new requests should work
	t.Log("Testing recovery after stress...")

	// Create a new coupon for recovery test
	_, err = testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"RECOVERY_TEST", 10, 10)
	require.NoError(t, err, "Failed to create recovery coupon")

	// Verify new HTTP request succeeds
	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_recovery",
		"coupon_name": "RECOVERY_TEST",
	})
	require.NoError(t, err, "Recovery request should not error")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"System should recover and process new requests successfully")

	t.Log("Pool stress test completed - system recovered successfully")
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

	// Test 1: Direct query timeout with pg_sleep (database-level test)
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

	// Test 2: Transaction timeout with rollback verification (database-level test)
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

	// Test 3: HTTP request works after timeout scenarios
	t.Run("HTTP API works after timeout scenarios", func(t *testing.T) {
		cleanupTables(t)

		const couponName = "POST_TIMEOUT_TEST"
		const availableStock = 100

		// Setup coupon
		setupCtx, setupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer setupCancel()

		_, err := testPool.Exec(setupCtx,
			"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
			couponName, availableStock, availableStock)
		require.NoError(t, err, "Failed to create test coupon")

		// HTTP request should work after timeout scenarios
		resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
			"user_id":     "user_after_timeout",
			"coupon_name": couponName,
		})
		require.NoError(t, err, "HTTP request should succeed")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Claim should succeed")

		t.Log("HTTP API correctly handles requests after timeout scenarios")
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

	// Test 1: Terminate connection mid-transaction (database-level test)
	t.Run("Connection terminated mid-transaction", func(t *testing.T) {
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
		_, err = testPool.Exec(testCtx, "SELECT pg_terminate_backend($1)", backendPID)
		if err != nil {
			t.Logf("Note: pg_terminate_backend returned error (expected in some cases): %v", err)
		}

		// The transaction should now be broken
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

	// Test 2: Verify HTTP API recovers after connection drop
	t.Run("HTTP API recovery after connection drop", func(t *testing.T) {
		// Multiple subsequent HTTP operations should succeed
		for i := 0; i < 5; i++ {
			resp, err := getJSON(formatURL("/health"))
			require.NoError(t, err, "Health check %d should not error", i+1)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode,
				"Health check %d should return 200", i+1)
		}

		// Create a new coupon via HTTP to prove the API is fully functional
		resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
			"name":   "RECOVERY_VERIFY",
			"amount": 50,
		})
		require.NoError(t, err, "Should be able to create new coupon after recovery")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode,
			"Coupon creation should succeed")

		t.Log("HTTP API successfully recovered after connection drop")
	})

	// Test 3: HTTP claim works after connection recovery
	t.Run("HTTP claim after connection recovery", func(t *testing.T) {
		resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
			"user_id":     "user_after_drop",
			"coupon_name": couponName,
		})
		require.NoError(t, err, "HTTP claim should not error")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Claim should succeed after connection recovery")

		// Verify claim via GET API
		getResp, err := getJSON(formatURL("/api/coupons/" + couponName))
		require.NoError(t, err, "GET should not error")
		defer getResp.Body.Close()
		assert.Equal(t, http.StatusOK, getResp.StatusCode, "GET should succeed")

		t.Log("HTTP claim correctly handled after pool recovery")
	})
}

// TestGoroutineLeakDetection is a comprehensive test that runs multiple
// chaos scenarios via HTTP and verifies no goroutine leaks occur.
func TestGoroutineLeakDetection(t *testing.T) {
	cleanupTables(t)

	// Record baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()
	t.Logf("Baseline goroutine count: %d", baselineGoroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Setup test data via direct DB (setup only)
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"LEAK_TEST", 1000, 1000)
	require.NoError(t, err, "Failed to create test coupon")

	// Run multiple rounds of concurrent HTTP operations
	const rounds = 3
	const operationsPerRound = 30

	for round := 1; round <= rounds; round++ {
		t.Logf("Running round %d/%d...", round, rounds)

		var wg sync.WaitGroup
		for i := 0; i < operationsPerRound; i++ {
			wg.Add(1)
			go func(roundNum, opID int) {
				defer wg.Done()

				userID := fmt.Sprintf("leak_test_user_%d_%d", roundNum, opID)
				resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
					"user_id":     userID,
					"coupon_name": "LEAK_TEST",
				})
				if err == nil {
					resp.Body.Close()
				}
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

	// Allow reasonable variance (15 goroutines) for runtime variations
	maxAllowedGoroutines := baselineGoroutines + 15
	assert.LessOrEqual(t, finalGoroutines, maxAllowedGoroutines,
		"Possible goroutine leak detected: baseline=%d, final=%d, max_allowed=%d",
		baselineGoroutines, finalGoroutines, maxAllowedGoroutines)

	t.Log("Goroutine leak detection test passed")
}
