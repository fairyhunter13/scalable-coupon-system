//go:build stress

// Package stress contains stress tests for concurrency safety validation.
// These tests verify the system handles high-concurrency scenarios correctly,
// specifically the Flash Sale (multiple users) and Double Dip (same user) attack patterns.
//
// IMPORTANT: All tests hit the REAL docker-compose server via net/http.
package stress

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoubleDip tests a double dip attack scenario with 10 concurrent requests
// from the SAME user attempting to claim a coupon.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
//
// This validates NFR2: System handles 10 concurrent same-user requests with exactly 1 success.
// The UNIQUE(user_id, coupon_name) constraint in the database prevents duplicate claims.
//
// Story Acceptance Criteria (from story 4-4):
//
//	AC #1: Given a coupon "DOUBLE_TEST" with amount=100
//	       And a single user "user_greedy"
//	       When 10 concurrent goroutines attempt to claim for "user_greedy" simultaneously
//	       Then exactly 1 claim succeeds (200 response)
//	       And exactly 9 claims fail (409 Conflict - already claimed)
//	       And remaining_amount is exactly 99
//	       And claimed_by contains exactly ["user_greedy"]
//
//	AC #2: Test passes consistently via `go test -tags stress ./tests/stress/... -run TestDoubleDip -v`
//
//	AC #3: Test passes 10 consecutive runs without flakiness
//
//	AC #4: Only one claim record exists for (user_greedy, DOUBLE_TEST) in the database
//
// Design Note: Stock is set to 100 (not 1) to ensure all 9 failures are due to
// 409 Conflict (UNIQUE constraint violation), NOT 400 (out of stock). This isolates
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

	startTime := time.Now()
	t.Logf("Starting double dip stress test: %d concurrent same-user requests", concurrentRequests)
	t.Logf("Test server: %s", testServer)

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Execute: Launch 10 concurrent goroutines using sync.WaitGroup
	// ALL goroutines claim with the SAME user_id "user_greedy"
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests) // Buffered channel for status codes

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Hit the REAL HTTP endpoint via net/http - same user_id for all requests
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": couponName,
			})
			if err != nil {
				t.Logf("Request error: %v", err)
				results <- 0 // Use 0 to indicate error
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}()
	}

	wg.Wait()
	close(results)

	// Collect and count results
	var successes, alreadyClaimed, otherErrors int
	for statusCode := range results {
		switch statusCode {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			alreadyClaimed++ // 409 = already claimed
		default:
			otherErrors++
			t.Logf("Unexpected status code: %d", statusCode)
		}
	}

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, AlreadyClaimed: %d, Other: %d", successes, alreadyClaimed, otherErrors)
	t.Logf("Execution time: %v", executionTime)

	// Verify database state
	remainingAmount, claimCount := getCouponFromDB(t, couponName)

	// AC1: Assert exactly 1 success
	assert.Equal(t, 1, successes, "Exactly one claim should succeed")

	// AC1: Assert exactly 9 Conflict failures (409)
	assert.Equal(t, concurrentRequests-1, alreadyClaimed,
		"Exactly %d claims should fail with 409 (already claimed)", concurrentRequests-1)

	// Assert 0 other errors (no 400 out of stock expected - plenty of stock available)
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// AC1: Verify remaining_amount = 99 (only 1 successful claim)
	assert.Equal(t, availableStock-1, remainingAmount,
		"remaining_amount should be %d (original %d minus 1 successful claim)",
		availableStock-1, availableStock)

	// AC4: Verify exactly 1 claim exists for user_greedy
	assert.Equal(t, 1, claimCount,
		"Exactly 1 claim record should exist for %s", userID)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d", remainingAmount, claimCount)

	// AC #2: Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)

	// Performance regression check: under normal conditions, 10 concurrent claims
	// should complete well under 5 seconds (typically <100ms with local Docker)
	const performanceThreshold = 5 * time.Second
	assert.Less(t, executionTime, performanceThreshold,
		"Performance regression: test took %v, expected under %v", executionTime, performanceThreshold)

	// AC #1: Verify claimed_by contains exactly ["user_greedy"] via GET endpoint
	resp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)
	defer resp.Body.Close()

	var couponResp map[string]interface{}
	err = readJSONResponse(resp, &couponResp)
	require.NoError(t, err)

	claimedBy, ok := couponResp["claimed_by"].([]interface{})
	require.True(t, ok, "claimed_by should be an array")
	assert.Len(t, claimedBy, 1, "claimed_by should contain exactly 1 user")
	if len(claimedBy) > 0 {
		assert.Equal(t, userID, claimedBy[0], "claimed_by should contain %s", userID)
	}

	t.Logf("claimed_by verification: %v", claimedBy)
}

// TestDoubleDip_ContextCancellation verifies graceful handling when concurrent
// claim operations complete. This ensures no goroutine leaks or resource
// exhaustion occur under normal termination conditions.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
func TestDoubleDip_ContextCancellation(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "CANCEL_TEST"
		availableStock     = 100
		concurrentRequests = 10
		userID             = "user_cancel"
	)

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Launch concurrent claims
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": couponName,
			})
			if err != nil {
				results <- 0
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}()
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	// Verify goroutines complete within reasonable time (no leaks/hangs)
	select {
	case <-done:
		t.Log("All goroutines completed")
	case <-time.After(10 * time.Second):
		t.Fatal("Goroutines did not complete within 10 seconds - possible goroutine leak")
	}

	// Count results
	var successes, alreadyClaimed, otherErrors int
	for statusCode := range results {
		switch statusCode {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			alreadyClaimed++
		default:
			otherErrors++
		}
	}

	t.Logf("Results - Successes: %d, AlreadyClaimed: %d, Other: %d",
		successes, alreadyClaimed, otherErrors)

	// Key assertion: at most 1 success (same user can only claim once)
	assert.LessOrEqual(t, successes, 1,
		"At most 1 claim should succeed for the same user")

	// Verify database consistency
	_, claimCount := getCouponFromDB(t, couponName)

	if successes > 0 {
		assert.Equal(t, 1, claimCount, "If any success, exactly 1 claim record should exist")
	} else {
		assert.Equal(t, 0, claimCount, "If no success, no claim record should exist")
	}

	t.Logf("Database state - claim_count: %d", claimCount)
}
