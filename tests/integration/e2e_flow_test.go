//go:build integration

// Package integration contains end-to-end API flow tests that verify
// the complete user journey through the coupon system.
//
// These tests run against the real docker-compose infrastructure and
// test the full API flow without any direct database manipulation.
package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_CreateGetClaimFlow tests the complete happy path flow:
// 1. Create a coupon via API
// 2. Get the coupon via API
// 3. Claim the coupon via API
// 4. Verify claim was recorded via GET API
func TestE2E_CreateGetClaimFlow(t *testing.T) {
	cleanupTables(t)

	const (
		couponName = "E2E_TEST_COUPON"
		amount     = 100
		userID     = "test_user_1"
	)

	// Step 1: Create a coupon via API
	t.Log("Step 1: Creating coupon via API")
	createResp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   couponName,
		"amount": amount,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "Should create coupon successfully")
	createResp.Body.Close()

	// Step 2: Get the coupon via API
	t.Log("Step 2: Getting coupon via API")
	getResp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode, "Should get coupon successfully")

	var couponData map[string]interface{}
	body, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	require.NoError(t, json.Unmarshal(body, &couponData))

	assert.Equal(t, couponName, couponData["name"], "Coupon name should match")
	assert.Equal(t, float64(amount), couponData["amount"], "Coupon amount should match")
	assert.Equal(t, float64(amount), couponData["remaining_amount"], "Remaining amount should equal amount initially")
	assert.Empty(t, couponData["claimed_by"], "No claims initially")

	// Step 3: Claim the coupon via API
	t.Log("Step 3: Claiming coupon via API")
	claimResp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     userID,
		"coupon_name": couponName,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, claimResp.StatusCode, "Should claim coupon successfully")
	claimResp.Body.Close()

	// Step 4: Verify claim was recorded via GET API
	t.Log("Step 4: Verifying claim via GET API")
	verifyResp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, verifyResp.StatusCode)

	body, _ = io.ReadAll(verifyResp.Body)
	verifyResp.Body.Close()
	require.NoError(t, json.Unmarshal(body, &couponData))

	assert.Equal(t, float64(amount-1), couponData["remaining_amount"], "Remaining amount should decrease by 1")
	claimedBy, ok := couponData["claimed_by"].([]interface{})
	require.True(t, ok, "claimed_by should be an array")
	assert.Len(t, claimedBy, 1, "Should have 1 claimer")
	if len(claimedBy) > 0 {
		assert.Equal(t, userID, claimedBy[0], "Claimer should be the test user")
	}

	t.Log("E2E flow completed successfully!")
}

// TestE2E_MultipleClaimsFlow tests multiple users claiming the same coupon:
// 1. Create a coupon with amount=5
// 2. 5 different users claim successfully
// 3. 6th user claim fails with out of stock
func TestE2E_MultipleClaimsFlow(t *testing.T) {
	cleanupTables(t)

	const (
		couponName     = "E2E_MULTI_CLAIM"
		initialAmount  = 5
		totalAttempts  = 6
	)

	// Step 1: Create a coupon via API
	t.Log("Step 1: Creating coupon with amount=5")
	createResp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   couponName,
		"amount": initialAmount,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	createResp.Body.Close()

	// Step 2: 6 users attempt to claim
	t.Log("Step 2: 6 users attempting to claim")
	var successCount, failCount int
	for i := 0; i < totalAttempts; i++ {
		userID := fmt.Sprintf("user_%d", i)
		claimResp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
			"user_id":     userID,
			"coupon_name": couponName,
		})
		require.NoError(t, err)

		if claimResp.StatusCode == http.StatusOK {
			successCount++
			t.Logf("  User %s: SUCCESS", userID)
		} else if claimResp.StatusCode == http.StatusBadRequest {
			failCount++
			t.Logf("  User %s: OUT OF STOCK", userID)
		}
		claimResp.Body.Close()
	}

	// Step 3: Verify results
	t.Log("Step 3: Verifying results")
	assert.Equal(t, initialAmount, successCount, "Exactly 5 claims should succeed")
	assert.Equal(t, 1, failCount, "Exactly 1 claim should fail")

	// Verify via GET API
	getResp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var couponData map[string]interface{}
	body, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	require.NoError(t, json.Unmarshal(body, &couponData))

	assert.Equal(t, float64(0), couponData["remaining_amount"], "Remaining amount should be 0")
	claimedBy, _ := couponData["claimed_by"].([]interface{})
	assert.Len(t, claimedBy, initialAmount, "Should have 5 claimers")

	t.Log("E2E multiple claims flow completed successfully!")
}

// TestE2E_DoubleDipPrevention tests that a user cannot claim the same coupon twice:
// 1. Create a coupon
// 2. User claims successfully
// 3. Same user attempts to claim again - should fail with 409 Conflict
func TestE2E_DoubleDipPrevention(t *testing.T) {
	cleanupTables(t)

	const (
		couponName = "E2E_DOUBLE_DIP"
		amount     = 100
		userID     = "greedy_user"
	)

	// Step 1: Create a coupon via API
	t.Log("Step 1: Creating coupon")
	createResp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   couponName,
		"amount": amount,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	createResp.Body.Close()

	// Step 2: First claim should succeed
	t.Log("Step 2: First claim attempt")
	claim1Resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     userID,
		"coupon_name": couponName,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, claim1Resp.StatusCode, "First claim should succeed")
	claim1Resp.Body.Close()

	// Step 3: Second claim should fail with 409 Conflict
	t.Log("Step 3: Second claim attempt (should fail)")
	claim2Resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     userID,
		"coupon_name": couponName,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, claim2Resp.StatusCode, "Second claim should fail with 409")
	claim2Resp.Body.Close()

	// Verify only 1 claim exists
	getResp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)

	var couponData map[string]interface{}
	body, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	require.NoError(t, json.Unmarshal(body, &couponData))

	assert.Equal(t, float64(amount-1), couponData["remaining_amount"], "Only 1 should be claimed")
	claimedBy, _ := couponData["claimed_by"].([]interface{})
	assert.Len(t, claimedBy, 1, "Should have only 1 claimer")

	t.Log("E2E double dip prevention verified!")
}

// TestE2E_ConcurrentClaimsFlow tests concurrent claims with proper race handling:
// 1. Create a coupon with amount=10
// 2. 50 users claim concurrently
// 3. Verify exactly 10 succeed and 40 fail
func TestE2E_ConcurrentClaimsFlow(t *testing.T) {
	cleanupTables(t)

	const (
		couponName         = "E2E_CONCURRENT"
		initialAmount      = 10
		concurrentRequests = 50
	)

	// Step 1: Create a coupon via API
	t.Log("Step 1: Creating coupon with amount=10")
	createResp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   couponName,
		"amount": initialAmount,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	createResp.Body.Close()

	// Step 2: 50 users claim concurrently
	t.Log("Step 2: 50 concurrent claim attempts")
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
		}(fmt.Sprintf("concurrent_user_%d", i))
	}

	wg.Wait()
	close(results)

	// Collect results
	var successCount, failCount, otherCount int
	for status := range results {
		switch status {
		case http.StatusOK:
			successCount++
		case http.StatusBadRequest:
			failCount++
		default:
			otherCount++
		}
	}

	t.Logf("Results: Success=%d, OutOfStock=%d, Other=%d", successCount, failCount, otherCount)

	// Step 3: Verify results
	assert.Equal(t, initialAmount, successCount, "Exactly 10 claims should succeed")
	assert.Equal(t, concurrentRequests-initialAmount, failCount, "Exactly 40 should fail with out of stock")
	assert.Equal(t, 0, otherCount, "No other errors should occur")

	// Verify via GET API
	getResp, err := getJSON(formatURL("/api/coupons/" + couponName))
	require.NoError(t, err)

	var couponData map[string]interface{}
	body, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	require.NoError(t, json.Unmarshal(body, &couponData))

	assert.Equal(t, float64(0), couponData["remaining_amount"], "Remaining amount should be 0")

	t.Log("E2E concurrent claims flow completed successfully!")
}

// TestE2E_NonExistentCoupon tests error handling for non-existent coupon:
// 1. Try to GET a non-existent coupon - should return 404
// 2. Try to claim a non-existent coupon - should return 404
func TestE2E_NonExistentCoupon(t *testing.T) {
	cleanupTables(t)

	const nonExistentCoupon = "DOES_NOT_EXIST"

	// Step 1: Try to GET non-existent coupon
	t.Log("Step 1: Getting non-existent coupon")
	getResp, err := getJSON(formatURL("/api/coupons/" + nonExistentCoupon))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode, "Should return 404 for non-existent coupon")
	getResp.Body.Close()

	// Step 2: Try to claim non-existent coupon
	t.Log("Step 2: Claiming non-existent coupon")
	claimResp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "test_user",
		"coupon_name": nonExistentCoupon,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, claimResp.StatusCode, "Should return 404 for claiming non-existent coupon")
	claimResp.Body.Close()

	t.Log("E2E non-existent coupon handling verified!")
}

// TestE2E_ValidationErrors tests API validation:
// 1. Create coupon with invalid data (missing name, zero amount, etc.)
// 2. Claim with invalid data (missing user_id, etc.)
func TestE2E_ValidationErrors(t *testing.T) {
	cleanupTables(t)

	// Test 1: Create coupon with missing name
	t.Log("Test 1: Create coupon with missing name")
	resp1, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"amount": 100,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp1.StatusCode, "Should reject missing name")
	resp1.Body.Close()

	// Test 2: Create coupon with zero amount
	t.Log("Test 2: Create coupon with zero amount")
	resp2, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "test_coupon",
		"amount": 0,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode, "Should reject zero amount")
	resp2.Body.Close()

	// Test 3: Create coupon with negative amount
	t.Log("Test 3: Create coupon with negative amount")
	resp3, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "test_coupon",
		"amount": -10,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp3.StatusCode, "Should reject negative amount")
	resp3.Body.Close()

	// Create a valid coupon for claim tests
	createResp, _ := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "valid_coupon",
		"amount": 100,
	})
	createResp.Body.Close()

	// Test 4: Claim with missing user_id
	t.Log("Test 4: Claim with missing user_id")
	resp4, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"coupon_name": "valid_coupon",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp4.StatusCode, "Should reject missing user_id")
	resp4.Body.Close()

	// Test 5: Claim with missing coupon_name
	t.Log("Test 5: Claim with missing coupon_name")
	resp5, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id": "test_user",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp5.StatusCode, "Should reject missing coupon_name")
	resp5.Body.Close()

	t.Log("E2E validation errors verified!")
}
