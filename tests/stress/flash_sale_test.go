//go:build stress

package stress

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlashSale tests a flash sale attack scenario with 50 concurrent requests
// attempting to claim a coupon with only 5 stock available.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
//
// AC1: Given a coupon "FLASH_TEST" with amount=5
//
//	When 50 concurrent goroutines attempt to claim it simultaneously
//	Then exactly 5 claims succeed (200 responses)
//	And exactly 45 claims fail (400 out of stock)
//	And remaining_amount is exactly 0
//	And claimed_by contains exactly 5 unique user IDs
//
// AC2: Test passes consistently and completes within 30 seconds
// AC3: Uses sync.WaitGroup for coordination, collects response status codes
// AC4: Test is deterministic - passes 10 consecutive runs
func TestFlashSale(t *testing.T) {
	cleanupTables(t)

	// Test configuration
	const (
		couponName         = "FLASH_TEST"
		availableStock     = 5
		concurrentRequests = 50
		timeout            = 30 * time.Second
	)

	startTime := time.Now()
	t.Logf("Starting flash sale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Test server: %s", testServer)

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Execute: Launch 50 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests) // Buffered channel for status codes

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()

			// Hit the REAL HTTP endpoint via net/http
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": couponName,
			})
			if err != nil {
				t.Logf("Request error for %s: %v", userID, err)
				results <- 0 // Use 0 to indicate error
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Collect and count results
	var successes, noStocks, otherErrors int
	for statusCode := range results {
		switch statusCode {
		case http.StatusOK:
			successes++
		case http.StatusBadRequest:
			noStocks++ // 400 = out of stock
		default:
			otherErrors++
			t.Logf("Unexpected status code: %d", statusCode)
		}
	}

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d", successes, noStocks, otherErrors)
	t.Logf("Execution time: %v", executionTime)

	// Verify database state
	remainingAmount, claimCount := getCouponFromDB(t, couponName)
	uniqueUsers := getUniqueClaimers(t, couponName)

	// AC1: Assert exactly 5 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC1: Assert exactly 45 out of stock failures (400 Bad Request)
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with 400 (out of stock)", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// AC1: Verify remaining_amount = 0
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")
	require.GreaterOrEqual(t, remainingAmount, 0, "remaining_amount should never be negative")

	// AC1: Verify exactly 5 claims exist
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	// AC1: Verify 5 unique user_ids in claimed_by
	assert.Equal(t, availableStock, uniqueUsers,
		"Exactly %d unique user IDs should have claims", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d, unique_users: %d",
		remainingAmount, claimCount, uniqueUsers)

	// AC2: Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}
