//go:build ci

// Package stress contains stress tests for the scalable coupon system.
//
// CI-ONLY Scale Stress Tests
// ==========================
//
// This file contains scale stress tests that are only run in CI environments.
// These tests are excluded from local `go test ./...` runs by default.
//
// Build Tag Usage:
// - Without `-tags ci`: Tests in this file are excluded
// - With `-tags ci`: Tests in this file are included
//
// Local Testing:
//   go test ./tests/stress/...                    # Excludes scale tests
//   go test -tags ci ./tests/stress/...           # Includes scale tests
//
// CI Testing:
//   go test -v -race -tags ci ./tests/stress/...  # Full test suite with race detection
//
// These tests require significant resources (100-500 concurrent goroutines)
// and are designed to prove system resilience beyond spec requirements.

package stress

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// TestScaleStress100 tests 100 concurrent goroutines claiming a coupon with stock=10.
//
// AC1: Given the CI pipeline runs the scale stress test job,
//
//	When 100 concurrent goroutines attempt to claim a coupon with stock=10,
//	Then exactly 10 claims succeed (200/201 responses),
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Pool stats before test - Total: %d, Idle: %d, In-Use: %d",
		testPool.Stat().TotalConns(),
		testPool.Stat().IdleConns(),
		testPool.Stat().AcquiredConns())

	// Setup: Create coupon with stock=10
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Launch 100 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(fmt.Sprintf("scale100_user_%d", i))
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
	t.Logf("Pool stats after test - Total: %d, Idle: %d, In-Use: %d",
		testPool.Stat().TotalConns(),
		testPool.Stat().IdleConns(),
		testPool.Stat().AcquiredConns())

	// AC1: Assert exactly 10 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC1: Assert exactly 90 ErrNoStock failures
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

	// AC1: Query database to verify exactly 10 claims exist
	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)
	t.Logf("Pool stats before test - Total: %d, Idle: %d, In-Use: %d",
		testPool.Stat().TotalConns(),
		testPool.Stat().IdleConns(),
		testPool.Stat().AcquiredConns())

	// Setup: Create coupon with stock=20
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Launch 200 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(fmt.Sprintf("scale200_user_%d", i))
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
	t.Logf("Pool stats after test - Total: %d, Idle: %d, In-Use: %d",
		testPool.Stat().TotalConns(),
		testPool.Stat().IdleConns(),
		testPool.Stat().AcquiredConns())

	// AC2: Assert exactly 20 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC2: Assert exactly 180 ErrNoStock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with ErrNoStock", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify database state consistency
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remainingAmount)
	require.NoError(t, err, "Failed to query remaining_amount")
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")

	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")
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
// AC3: Given the CI pipeline runs the scale stress test job,
//
//	When 500 concurrent goroutines attempt to claim a coupon with stock=50,
//	Then exactly 50 claims succeed,
//	And no database connection pool exhaustion occurs,
//	And test is tagged with `//go:build ci` to prevent local execution
func TestScaleStress500(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "SCALE_500_TEST"
		availableStock     = 50
		concurrentRequests = 500
		timeout            = 120 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	t.Logf("Starting scale stress test: %d concurrent requests, %d stock", concurrentRequests, availableStock)

	// Log initial pool stats to monitor for connection pool exhaustion
	stats := testPool.Stat()
	t.Logf("Pool stats before test - Total: %d, Idle: %d, In-Use: %d, MaxConns: %d",
		stats.TotalConns(),
		stats.IdleConns(),
		stats.AcquiredConns(),
		stats.MaxConns())

	// Setup: Create coupon with stock=50
	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		couponName, availableStock, availableStock)
	require.NoError(t, err, "Failed to create test coupon")

	// Setup service layer
	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	// Execute: Launch 500 concurrent goroutines using sync.WaitGroup
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	// Track pool exhaustion metrics using atomic operations for thread safety
	var maxAcquiredConns atomic.Int32
	var poolExhaustionDetected atomic.Bool

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()

			// Check for pool exhaustion before each operation (atomic for thread safety)
			currentStats := testPool.Stat()
			acquired := currentStats.AcquiredConns()
			// Atomically update max if current is greater
			for {
				current := maxAcquiredConns.Load()
				if acquired <= current {
					break
				}
				if maxAcquiredConns.CompareAndSwap(current, acquired) {
					break
				}
			}
			if acquired == currentStats.MaxConns() {
				poolExhaustionDetected.Store(true)
			}

			err := couponService.ClaimCoupon(ctx, userID, couponName)
			results <- err
		}(fmt.Sprintf("scale500_user_%d", i))
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

	// Log final pool stats
	finalStats := testPool.Stat()
	t.Logf("Pool stats after test - Total: %d, Idle: %d, In-Use: %d, MaxConns: %d",
		finalStats.TotalConns(),
		finalStats.IdleConns(),
		finalStats.AcquiredConns(),
		finalStats.MaxConns())
	t.Logf("Max concurrent connections during test: %d", maxAcquiredConns.Load())

	// AC3: Assert exactly 50 successes
	assert.Equal(t, availableStock, successes,
		"Exactly %d claims should succeed", availableStock)

	// AC3: Assert exactly 450 ErrNoStock failures
	assert.Equal(t, concurrentRequests-availableStock, noStocks,
		"Exactly %d claims should fail with ErrNoStock", concurrentRequests-availableStock)

	// Assert 0 other errors
	assert.Equal(t, 0, otherErrors, "No other errors should occur")

	// Verify database state consistency
	var remainingAmount int
	err = testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		couponName).Scan(&remainingAmount)
	require.NoError(t, err, "Failed to query remaining_amount")
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be exactly 0")

	var claimCount int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		couponName).Scan(&claimCount)
	require.NoError(t, err, "Failed to query claim count")
	assert.Equal(t, availableStock, claimCount,
		"Exactly %d claim records should exist", availableStock)

	t.Logf("Database verification - remaining_amount: %d, claim_count: %d",
		remainingAmount, claimCount)

	// AC3: Verify no pool exhaustion occurred
	// Note: Pool exhaustion = connection acquisition failures (blocked/timeout), NOT reaching max capacity
	// Reaching max capacity is expected under high concurrency and is handled by pgxpool's queuing
	// True exhaustion would cause "other errors" above (context deadline exceeded, connection pool exhausted)
	assert.Equal(t, 0, otherErrors,
		"No connection pool exhaustion should occur (otherErrors indicates acquisition failures)")

	// Log pool utilization for observability
	if poolExhaustionDetected.Load() {
		t.Logf("INFO: Connection pool reached maximum capacity (%d) - this is expected under high concurrency",
			finalStats.MaxConns())
	}

	// Verify execution completed within timeout
	assert.Less(t, executionTime, timeout,
		"Test should complete within %v", timeout)
}
