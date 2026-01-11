package stress

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

// TestFlashSale tests a flash sale attack scenario with 50 concurrent requests
// attempting to claim a coupon with only 5 stock available.
//
// AC1: Given a coupon "FLASH_TEST" with amount=5
//
//	When 50 concurrent goroutines attempt to claim it simultaneously
//	Then exactly 5 claims succeed (200/201 responses)
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	t.Logf("Starting flash sale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)

	// Setup: Create coupon "FLASH_TEST" with amount=5, remaining_amount=5
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Launch 50 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests) // Buffered channel for all results

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(fmt.Sprintf("user_%d", i))
	}

	wg.Wait()
	close(results)

	// Collect and count results
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

	executionTime := time.Since(startTime)
	t.Logf("Results - Successes: %d, NoStock: %d, Other: %d", successes, noStocks, otherErrors)
	t.Logf("Execution time: %v", executionTime)

	// AC1: Assert exactly 5 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC1: Assert exactly 45 ErrNoStock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with ErrNoStock", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// AC1: Query database to verify remaining_amount = 0
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remainingAmount)
	require.NoError(t, err, "Failed to query remaining_amount")
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")
	assert.GreaterOrEqual(t, remainingAmount, 0, "remaining_amount should never be negative")

	// AC1: Query database to verify exactly 5 claims exist
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	// AC1: Verify 5 unique user_ids in claimed_by
	var uniqueUsers int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(DISTINCT user_id) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&uniqueUsers)
	require.NoError(t, err, "Failed to query unique users")
	assert.Equal(t, availableStock, uniqueUsers,
		"Exactly %d unique user IDs should have claims", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d, unique_users: %d",
		remainingAmount, claimCount, uniqueUsers)

	// AC2: Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}
