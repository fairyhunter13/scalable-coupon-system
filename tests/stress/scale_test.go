//go:build stress

// Package stress contains stress tests for the scalable coupon system.
//
// Scale Stress Tests
// ==================
//
// These tests run against the real docker-compose infrastructure.
// They require docker-compose to be running before execution.
//
// Usage:
//   docker-compose up -d                               # Start services
//   go test -v -race -tags stress ./tests/stress/...   # Run tests
//   docker-compose down                                # Cleanup
//
// These tests require significant resources (100-500 concurrent goroutines)
// and are designed to prove system resilience beyond spec requirements.

package stress

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScaleStress100 tests 100 concurrent goroutines claiming a coupon with stock=10.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
//
// AC1: Given the CI pipeline runs the scale stress test job,
//
//	When 100 concurrent goroutines attempt to claim a coupon with stock=10,
//	Then exactly 10 claims succeed (200 responses),
//	And exactly 90 claims fail (400 out of stock),
//	And remaining_amount is exactly 0,
//	And test completes without race conditions (`-race` flag)
func TestScaleStress100(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "SCALE_100_TEST"
		availableStock     = 10
		concurrentRequests = 100
		timeout            = 60 * time.Second
	)

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Test server: %s", testServer)
	logPoolStats(t, "Before test")

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Execute: Launch 100 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

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
				results <- 0
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(fmt.Sprintf("scale100_user_%d", i))
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
	logPoolStats(t, "After test")

	// Verify database state
	remainingAmount, claimCount := getCouponFromDB(t, couponName)

	// AC1: Assert exactly 10 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC1: Assert exactly 90 out of stock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with 400 (out of stock)", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// AC1: Verify remaining_amount = 0
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")

	// AC1: Verify exactly 10 claims exist
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d",
		remainingAmount, claimCount)

	// Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}

// TestScaleStress200 tests 200 concurrent goroutines claiming a coupon with stock=20.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
//
// AC2: Given the CI pipeline runs the scale stress test job,
//
//	When 200 concurrent goroutines attempt to claim a coupon with stock=20,
//	Then exactly 20 claims succeed,
//	And test completes within 60 seconds
func TestScaleStress200(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "SCALE_200_TEST"
		availableStock     = 20
		concurrentRequests = 200
		timeout            = 60 * time.Second
	)

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Test server: %s", testServer)
	logPoolStats(t, "Before test")

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Execute: Launch 200 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
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
		}(fmt.Sprintf("scale200_user_%d", i))
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
			noStocks++
		default:
			otherErrors++
			t.Logf("Unexpected status code: %d", statusCode)
		}
	}

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d", successes, noStocks, otherErrors)
	t.Logf("Execution time: %v", executionTime)
	logPoolStats(t, "After test")

	// Verify database state
	remainingAmount, claimCount := getCouponFromDB(t, couponName)

	// AC2: Assert exactly 20 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC2: Assert exactly 180 out of stock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with 400 (out of stock)", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify remaining_amount = 0
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")

	// Verify exactly 20 claims exist
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d",
		remainingAmount, claimCount)

	// AC2: Verify execution completed within 60 seconds
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}

// TestScaleStress500 tests 500 concurrent goroutines claiming a coupon with stock=50.
//
// IMPORTANT: This test hits the REAL docker-compose server via net/http.
//
// AC3: Given the CI pipeline runs the scale stress test job,
//
//	When 500 concurrent goroutines attempt to claim a coupon with stock=50,
//	Then exactly 50 claims succeed,
//	And no connection errors occur,
//	And test completes within 120 seconds
func TestScaleStress500(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "SCALE_500_TEST"
		availableStock     = 50
		concurrentRequests = 500
		timeout            = 120 * time.Second
	)

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Test server: %s", testServer)
	logPoolStats(t, "Before test")

	// Setup: Create coupon directly in database
	createTestCoupon(t, couponName, availableStock)

	// Execute: Launch 500 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	// Track metrics
	var connectionErrors atomic.Int32

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()

			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": couponName,
			})
			if err != nil {
				connectionErrors.Add(1)
				results <- 0
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(fmt.Sprintf("scale500_user_%d", i))
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
			noStocks++
		default:
			otherErrors++
			t.Logf("Unexpected status code: %d", statusCode)
		}
	}

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d, ConnectionErrors: %d",
		successes, noStocks, otherErrors, connectionErrors.Load())
	t.Logf("Execution time: %v", executionTime)
	logPoolStats(t, "After test")

	// Verify database state
	remainingAmount, claimCount := getCouponFromDB(t, couponName)

	// AC3: Assert exactly 50 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC3: Assert exactly 450 out of stock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with 400 (out of stock)", concurrentRequests-availableStock)

	// Assert 0 other errors and connection errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")
	assert.Equal(t, int32(0), connectionErrors.Load(),
		"No connection errors should occur")

	// Verify remaining_amount = 0
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")
	require.GreaterOrEqual(t, remainingAmount, 0, "remaining_amount should never be negative")

	// Verify exactly 50 claims exist
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d",
		remainingAmount, claimCount)

	// Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}
