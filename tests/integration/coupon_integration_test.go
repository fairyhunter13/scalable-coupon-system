//go:build integration

// Package integration contains integration tests that run against the real docker-compose infrastructure.
// These tests verify the system's HTTP API behavior end-to-end using real HTTP requests.
//
// All tests use postJSON/getJSON helpers which make real HTTP calls to the docker-compose server.
package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateCoupon_Integration_Success tests POST /api/coupons success via real HTTP
func TestCreateCoupon_Integration_Success(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "PROMO_SUPER",
		"amount": 100,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

	// Verify coupon was actually stored in database
	var name string
	var amount, remainingAmount int
	err = testPool.QueryRow(context.Background(),
		"SELECT name, amount, remaining_amount FROM coupons WHERE name = $1",
		"PROMO_SUPER").Scan(&name, &amount, &remainingAmount)

	require.NoError(t, err, "Coupon should be in database")
	assert.Equal(t, "PROMO_SUPER", name)
	assert.Equal(t, 100, amount)
	assert.Equal(t, 100, remainingAmount, "remaining_amount should equal amount on creation")
}

func TestCreateCoupon_Integration_InvalidInput_MissingName(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"amount": 50,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for missing name")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: name is required", result["error"])
}

func TestCreateCoupon_Integration_InvalidInput_MissingAmount(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name": "TEST_COUPON",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for missing amount")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: amount is required", result["error"])
}

func TestCreateCoupon_Integration_InvalidInput_ZeroAmount(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "ZERO_AMOUNT_TEST",
		"amount": 0,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for zero amount")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "invalid request", "Error should indicate invalid request")
}

func TestCreateCoupon_Integration_InvalidInput_NegativeAmount(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "NEGATIVE_AMOUNT_TEST",
		"amount": -10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for negative amount")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "invalid request", "Error should indicate invalid request")
}

func TestCreateCoupon_Integration_InvalidInput_EmptyBody(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for empty body")
}

func TestCreateCoupon_Integration_DuplicateName(t *testing.T) {
	cleanupTables(t)

	// Create first coupon
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "UNIQUE_COUPON",
		"amount": 50,
	})
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Try to create duplicate
	resp, err = postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "UNIQUE_COUPON",
		"amount": 50,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon already exists", result["error"])
}

// SQL Injection Tests - These verify that parameterized queries prevent injection attacks

func TestCreateCoupon_Integration_SQLInjection_DropTable(t *testing.T) {
	cleanupTables(t)

	// Attempt SQL injection via coupon name
	maliciousName := "'; DROP TABLE coupons;--"
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   maliciousName,
		"amount": 1,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed (coupon created with weird name) OR fail gracefully
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
		"Response should be 201 (created with literal name) or 400 (rejected)")

	// Verify coupons table still exists and is accessible
	var count int
	err = testPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM coupons").Scan(&count)
	require.NoError(t, err, "coupons table should still exist after SQL injection attempt")
}

func TestCreateCoupon_Integration_SQLInjection_UnionSelect(t *testing.T) {
	cleanupTables(t)

	// Attempt SQL injection via UNION SELECT
	maliciousName := "test' UNION SELECT * FROM pg_user--"
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   maliciousName,
		"amount": 10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify database integrity
	var count int
	err = testPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM coupons").Scan(&count)
	require.NoError(t, err, "Database should remain intact after UNION injection attempt")
}

func TestCreateCoupon_Integration_SQLInjection_CommentInjection(t *testing.T) {
	cleanupTables(t)

	// Attempt SQL injection via comment
	maliciousName := "test'/**/OR/**/1=1--"
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   maliciousName,
		"amount": 5,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify only expected data exists (not all rows)
	var count int
	err = testPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM coupons").Scan(&count)
	require.NoError(t, err)
	assert.LessOrEqual(t, count, 1, "SQL injection should not expose multiple rows")
}

func TestCreateCoupon_Integration_SQLInjection_BatchStatement(t *testing.T) {
	cleanupTables(t)

	// Attempt batch statement injection
	maliciousName := "test'; INSERT INTO coupons (name, amount, remaining_amount) VALUES ('HACKED', 999, 999);--"
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   maliciousName,
		"amount": 1,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify that no 'HACKED' coupon was created
	var count int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM coupons WHERE name = 'HACKED'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Batch injection should not create unauthorized rows")
}

func TestCreateCoupon_Integration_SQLInjection_NumericOverflow(t *testing.T) {
	cleanupTables(t)

	// Attempt injection via amount field (tests numeric handling)
	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "OVERFLOW_TEST",
		"amount": 2147483647,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should handle gracefully (either succeed with max int or fail validation)
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
		"Should handle large numbers gracefully")
}

func TestCreateCoupon_Integration_AtomicInsert(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "ATOMIC_TEST",
		"amount": 50,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify all fields were inserted atomically
	var name string
	var amount, remainingAmount int
	err = testPool.QueryRow(context.Background(),
		"SELECT name, amount, remaining_amount FROM coupons WHERE name = $1",
		"ATOMIC_TEST").Scan(&name, &amount, &remainingAmount)

	require.NoError(t, err)
	assert.Equal(t, "ATOMIC_TEST", name)
	assert.Equal(t, 50, amount)
	assert.Equal(t, 50, remainingAmount)
}

func TestCreateCoupon_Integration_EmptyResponseBody(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons"), map[string]interface{}{
		"name":   "EMPTY_BODY_TEST",
		"amount": 25,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// AC #1 requires empty response body
	respBody, _ := io.ReadAll(resp.Body)
	assert.Empty(t, respBody, "Response body should be empty on success per AC #1")
}

// GET /api/coupons/:name Integration Tests

func TestGetCoupon_Integration_WithClaims(t *testing.T) {
	cleanupTables(t)

	// Create coupon directly in DB
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"PROMO_SUPER", 100, 95)
	require.NoError(t, err)

	// Insert claims
	claims := []string{"user_001", "user_002", "user_003", "user_004", "user_005"}
	for _, userID := range claims {
		_, err := testPool.Exec(context.Background(),
			"INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)",
			userID, "PROMO_SUPER")
		require.NoError(t, err)
	}

	resp, err := getJSON(formatURL("/api/coupons/PROMO_SUPER"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "PROMO_SUPER", result["name"])
	assert.Equal(t, float64(100), result["amount"])
	assert.Equal(t, float64(95), result["remaining_amount"])

	claimedBy, ok := result["claimed_by"].([]interface{})
	require.True(t, ok, "claimed_by should be an array")
	assert.Len(t, claimedBy, 5)
}

func TestGetCoupon_Integration_NoClaims(t *testing.T) {
	cleanupTables(t)

	// Create coupon with no claims
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"NEW_PROMO", 100, 100)
	require.NoError(t, err)

	resp, err := getJSON(formatURL("/api/coupons/NEW_PROMO"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "NEW_PROMO", result["name"])
	assert.Equal(t, float64(100), result["amount"])
	assert.Equal(t, float64(100), result["remaining_amount"])

	// claimed_by should be empty array, not null
	claimedBy, ok := result["claimed_by"].([]interface{})
	require.True(t, ok, "claimed_by should be an array (not null)")
	assert.Len(t, claimedBy, 0, "claimed_by should be empty array")
}

func TestGetCoupon_Integration_NotFound(t *testing.T) {
	cleanupTables(t)

	resp, err := getJSON(formatURL("/api/coupons/NONEXISTENT"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon not found", result["error"])
}

func TestGetCoupon_Integration_SnakeCaseJSON(t *testing.T) {
	cleanupTables(t)

	// Create coupon
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"SNAKE_CASE_TEST", 100, 90)
	require.NoError(t, err)

	resp, err := getJSON(formatURL("/api/coupons/SNAKE_CASE_TEST"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse raw JSON to verify field names
	respBody, _ := io.ReadAll(resp.Body)
	var rawJSON map[string]interface{}
	err = json.Unmarshal(respBody, &rawJSON)
	require.NoError(t, err)

	// Verify snake_case field names exist
	_, hasName := rawJSON["name"]
	_, hasAmount := rawJSON["amount"]
	_, hasRemainingAmount := rawJSON["remaining_amount"]
	_, hasClaimedBy := rawJSON["claimed_by"]

	assert.True(t, hasName, "Response should have 'name' field")
	assert.True(t, hasAmount, "Response should have 'amount' field")
	assert.True(t, hasRemainingAmount, "Response should have 'remaining_amount' field (snake_case)")
	assert.True(t, hasClaimedBy, "Response should have 'claimed_by' field (snake_case)")

	// Verify no camelCase fields
	_, hasRemainingAmountCamel := rawJSON["remainingAmount"]
	_, hasClaimedByCamel := rawJSON["claimedBy"]

	assert.False(t, hasRemainingAmountCamel, "Response should NOT have 'remainingAmount' field (camelCase)")
	assert.False(t, hasClaimedByCamel, "Response should NOT have 'claimedBy' field (camelCase)")
}

// POST /api/coupons/claim Integration Tests

func TestClaimCoupon_Integration_Success(t *testing.T) {
	cleanupTables(t)

	// Create coupon with stock
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"PROMO_CLAIM", 100, 5)
	require.NoError(t, err)

	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_001",
		"coupon_name": "PROMO_CLAIM",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK for successful claim")

	// Verify empty response body per AC
	respBody, _ := io.ReadAll(resp.Body)
	assert.Empty(t, respBody, "Response body should be empty on success")

	// Verify database state: claim record exists
	var claimCount int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		"user_001", "PROMO_CLAIM").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 1, claimCount, "Claim record should exist")

	// Verify database state: remaining_amount decremented
	var remainingAmount int
	err = testPool.QueryRow(context.Background(),
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"PROMO_CLAIM").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 4, remainingAmount, "remaining_amount should be decremented to 4")
}

func TestClaimCoupon_Integration_DuplicateClaim(t *testing.T) {
	cleanupTables(t)

	// Create coupon
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"PROMO_DUP", 100, 10)
	require.NoError(t, err)

	// First claim - should succeed
	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_001",
		"coupon_name": "PROMO_DUP",
	})
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Second claim - should fail with 409
	resp, err = postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_001",
		"coupon_name": "PROMO_DUP",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "Expected 409 Conflict for duplicate claim")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon already claimed by user", result["error"], "Exact error message required per AC2")

	// Verify remaining_amount only decremented once
	var remainingAmount int
	err = testPool.QueryRow(context.Background(),
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"PROMO_DUP").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 9, remainingAmount, "remaining_amount should only decrement once")
}

func TestClaimCoupon_Integration_OutOfStock(t *testing.T) {
	cleanupTables(t)

	// Create coupon with zero stock
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"PROMO_EMPTY", 100, 0)
	require.NoError(t, err)

	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_999",
		"coupon_name": "PROMO_EMPTY",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for out of stock")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon out of stock", result["error"], "Exact error message required per AC3")

	// Verify no claim was created
	var claimCount int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
		"user_999", "PROMO_EMPTY").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 0, claimCount, "No claim should be created for out of stock")
}

func TestClaimCoupon_Integration_CouponNotFound(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_001",
		"coupon_name": "NONEXISTENT",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected 404 Not Found for missing coupon")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon not found", result["error"], "Exact error message required per AC4")
}

func TestClaimCoupon_Integration_MissingUserID(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"coupon_name": "PROMO_SUPER",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for missing user_id")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: user_id is required", result["error"], "Exact error message required per AC5")
}

func TestClaimCoupon_Integration_MissingCouponName(t *testing.T) {
	cleanupTables(t)

	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id": "user_001",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for missing coupon_name")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: coupon_name is required", result["error"], "Exact error message required per AC6")
}

func TestClaimCoupon_Integration_AtomicTransaction(t *testing.T) {
	cleanupTables(t)

	// Create coupon with limited stock
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
		"PROMO_ATOMIC", 100, 3)
	require.NoError(t, err)

	// Claim 3 times with different users
	users := []string{"user_a", "user_b", "user_c"}
	for _, userID := range users {
		resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
			"user_id":     userID,
			"coupon_name": "PROMO_ATOMIC",
		})
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "User %s should claim successfully", userID)
	}

	// Fourth claim should fail - out of stock
	resp, err := postJSON(formatURL("/api/coupons/claim"), map[string]string{
		"user_id":     "user_d",
		"coupon_name": "PROMO_ATOMIC",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Fourth claim should fail - out of stock")

	// Verify final state
	var remainingAmount int
	err = testPool.QueryRow(context.Background(),
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		"PROMO_ATOMIC").Scan(&remainingAmount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingAmount, "remaining_amount should be 0 after 3 claims")

	var claimCount int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		"PROMO_ATOMIC").Scan(&claimCount)
	require.NoError(t, err)
	assert.Equal(t, 3, claimCount, "Exactly 3 claims should exist")
}
