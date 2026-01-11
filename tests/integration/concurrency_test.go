//go:build integration

// Package integration contains concurrency tests that run against the real docker-compose infrastructure.
// These tests verify race condition handling using real HTTP requests to the API server.
package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentClaimLastStock tests AC4: Race Condition Prevention for Last Stock
// Given two concurrent claim requests for the last available coupon
// When both requests attempt to claim simultaneously
// Then exactly one succeeds with 200
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

	// Execute: Two concurrent claims via HTTP
	var wg sync.WaitGroup
	results := make(chan int, 2) // HTTP status codes

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": "LAST_STOCK_TEST",
			})
			if err != nil {
				t.Logf("HTTP error for %s: %v", userID, err)
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: exactly 1 success (200), exactly 1 out of stock (400)
	var successes, outOfStock, other int
	for code := range results {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusBadRequest:
			outOfStock++
		default:
			other++
			t.Logf("Unexpected status code: %d", code)
		}
	}

	assert.Equal(t, 1, successes, "Exactly one claim should succeed (200)")
	assert.Equal(t, 1, outOfStock, "Exactly one claim should fail with 400 (out of stock)")
	assert.Equal(t, 0, other, "No other status codes should occur")

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
// Then exactly one succeeds with 200
// And the rest fail with 409 Conflict
func TestConcurrentClaimsSameUser(t *testing.T) {
	cleanupTables(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create coupon with enough stock
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"SAME_USER_TEST", 100, 100)
	require.NoError(t, err)

	// Execute: 10 concurrent claims by the SAME user via HTTP
	var wg sync.WaitGroup
	results := make(chan int, 10) // HTTP status codes

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     "same_user",
				"coupon_name": "SAME_USER_TEST",
			})
			if err != nil {
				t.Logf("HTTP error: %v", err)
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}()
	}

	wg.Wait()
	close(results)

	// Verify: exactly 1 success (200), 9 already claimed (409)
	var successes, alreadyClaimed, other int
	for code := range results {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			alreadyClaimed++
		default:
			other++
			t.Logf("Unexpected status code: %d", code)
		}
	}

	assert.Equal(t, 1, successes, "Exactly one claim should succeed (200)")
	assert.Equal(t, 9, alreadyClaimed, "Nine claims should fail with 409 (already claimed)")
	assert.Equal(t, 0, other, "No other status codes should occur")

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
// And all requests with sufficient stock succeed
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

	// Execute: N concurrent claims from different users via HTTP
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": "SERIALIZATION_TEST",
			})
			if err != nil {
				t.Logf("HTTP error for %s: %v", userID, err)
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: all claims succeed (enough stock for all)
	var successes, failures int
	for code := range results {
		if code == http.StatusOK {
			successes++
		} else {
			failures++
			t.Logf("Unexpected status code: %d", code)
		}
	}

	assert.Equal(t, concurrentRequests, successes, "All claims should succeed")
	assert.Equal(t, 0, failures, "No claims should fail")

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

	// Execute: 20 concurrent claims from different users via HTTP
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
				"user_id":     userID,
				"coupon_name": "FLASH_SALE",
			})
			if err != nil {
				t.Logf("HTTP error for %s: %v", userID, err)
				results <- 0
				return
			}
			defer resp.Body.Close()
			results <- resp.StatusCode
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Verify: exactly 5 successes (200), 15 out of stock (400)
	var successes, outOfStock, other int
	for code := range results {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusBadRequest:
			outOfStock++
		default:
			other++
			t.Logf("Unexpected status code: %d", code)
		}
	}

	assert.Equal(t, availableStock, successes, "Exactly %d claims should succeed (200)", availableStock)
	assert.Equal(t, concurrentRequests-availableStock, outOfStock, "Exactly %d claims should fail with 400 (out of stock)", concurrentRequests-availableStock)
	assert.Equal(t, 0, other, "No other status codes should occur")

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
// When an error occurs (out of stock)
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

	// Execute: Attempt claim on zero stock via HTTP
	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_001",
		"coupon_name": "ZERO_STOCK",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify: 400 Bad Request returned
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request for out of stock")

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
