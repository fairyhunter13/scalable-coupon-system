//go:build ci

// Package chaos contains CI-only chaos engineering tests for input boundary validation.
// These tests verify the system's behavior under extreme input scenarios including
// large payloads, special characters, SQL injection attempts, and malformed requests.
//
// IMPORTANT: These tests are tagged with "ci" build constraint and should
// only run in CI environments where infrastructure is controlled.
// Use: go test -v -race -tags ci ./tests/chaos/...
package chaos

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/handler"
	"github.com/fairyhunter13/scalable-coupon-system/internal/repository"
	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// Test data generators

// generateLongString creates a string of the specified length filled with 'a'.
func generateLongString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// ptrInt is a helper to create a pointer to an int.
func ptrInt(i int) *int {
	return &i
}

// SQL injection payloads to test parameterized query protection.
var sqlInjectionPayloads = []string{
	"'; DROP TABLE coupons;--",
	"' OR '1'='1",
	"' UNION SELECT * FROM information_schema.tables--",
	"coupon_name/**/OR/**/1=1",
	"1; SELECT * FROM coupons WHERE 1=1--",
	"'; DELETE FROM claims;--",
	"' OR 1=1--",
	"1' OR '1' = '1",
	"admin'--",
	"' OR 'x'='x",
}

// Special character payloads to test character handling.
var specialCharPayloads = []struct {
	name    string
	payload string
}{
	{"null_byte", "coupon\x00name"},
	{"newline", "coupon\nname"},
	{"tab", "coupon\tname"},
	{"carriage_return", "coupon\rname"},
	{"single_quote", "coupon'name"},
	{"double_quote", "coupon\"name"},
	{"backslash", "coupon\\name"},
	{"emoji", "emoji🎉coupon"},
	{"chinese", "中文优惠券"},
	{"arabic", "كوبون"},
	{"mixed_unicode", "coupon_日本語_emoji_🎯"},
	{"control_chars", "coupon\x01\x02\x03name"},
	{"semicolon", "coupon;name"},
	{"pipe", "coupon|name"},
	{"ampersand", "coupon&name"},
	{"less_than", "coupon<name"},
	{"greater_than", "coupon>name"},
	{"percent", "coupon%name"},
}

// setupTestApp creates a Fiber app with routes configured for testing.
func setupTestApp(t *testing.T) *fiber.App {
	t.Helper()

	couponRepo := repository.NewCouponRepository(testPool)
	claimRepo := repository.NewClaimRepository(testPool)
	couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

	v := validator.New()
	couponHandler := handler.NewCouponHandler(couponService, v)
	claimHandler := handler.NewClaimHandler(couponService, v)

	app := fiber.New(fiber.Config{
		BodyLimit:             4 * 1024 * 1024, // 4MB body limit for testing
		ReadBufferSize:        16 * 1024,       // 16KB read buffer for long URLs
		DisableStartupMessage: true,
	})

	app.Post("/api/coupons", couponHandler.CreateCoupon)
	app.Get("/api/coupons/:name", couponHandler.GetCoupon)
	app.Post("/api/coupons/claim", claimHandler.ClaimCoupon)

	return app
}

// ============================================================================
// Task 2: Coupon Name Length Boundary Tests (AC: #1)
// ============================================================================

func TestCreateCoupon_LongNameBoundary(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	testCases := []struct {
		name           string
		couponNameLen  int
		expectedStatus int
		expectDBError  bool
		description    string
	}{
		{
			name:           "255_chars_at_db_limit",
			couponNameLen:  255,
			expectedStatus: fiber.StatusCreated,
			expectDBError:  false,
			description:    "Exactly at VARCHAR(255) limit - should succeed",
		},
		{
			name:           "256_chars_exceeds_db_limit",
			couponNameLen:  256,
			expectedStatus: fiber.StatusInternalServerError, // DB constraint violation
			expectDBError:  true,
			description:    "1 char over VARCHAR(255) limit - DB should reject",
		},
		{
			name:           "1000_chars_far_exceeds_limit",
			couponNameLen:  1000,
			expectedStatus: fiber.StatusInternalServerError, // DB constraint violation
			expectDBError:  true,
			description:    "1000+ chars per AC#1 - DB should reject",
		},
		{
			name:           "10000_chars_extreme",
			couponNameLen:  10000,
			expectedStatus: fiber.StatusInternalServerError, // DB constraint violation
			expectDBError:  true,
			description:    "Extreme length - DB should reject",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanupTables(t)
			couponName := generateLongString(tc.couponNameLen)

			body, _ := json.Marshal(map[string]interface{}{
				"name":   couponName,
				"amount": 100,
			})

			req := httptest.NewRequest("POST", "/api/coupons", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode,
				"Expected status %d for %s, got %d",
				tc.expectedStatus, tc.description, resp.StatusCode)

			// Verify no database entries for rejected names
			if tc.expectDBError {
				// The name shouldn't exist in database
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				var count int
				err := testPool.QueryRow(ctx,
					"SELECT COUNT(*) FROM coupons WHERE name = $1", couponName).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count, "No coupon should exist for rejected name")
			}
		})
	}
}

func TestGetCoupon_LongNameBoundary(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	testCases := []struct {
		name           string
		couponNameLen  int
		expectedStatus int
	}{
		{"1000_chars", 1000, fiber.StatusNotFound}, // Valid request, just not found
		{"5000_chars", 5000, fiber.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			couponName := generateLongString(tc.couponNameLen)

			// URL-encode the name to create valid HTTP request
			encodedName := url.PathEscape(couponName)
			req := httptest.NewRequest("GET", "/api/coupons/"+encodedName, nil)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode,
				"Long name GET should return %d", tc.expectedStatus)
		})
	}
}

func TestClaimCoupon_LongNameBoundary(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	testCases := []struct {
		name          string
		couponNameLen int
		userIDLen     int
	}{
		{"long_coupon_name", 1000, 10},
		{"long_user_id", 10, 1000},
		{"both_long", 1000, 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"coupon_name": generateLongString(tc.couponNameLen),
				"user_id":     generateLongString(tc.userIDLen),
			})

			req := httptest.NewRequest("POST", "/api/coupons/claim", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 404 (not found) since coupon doesn't exist
			// The important thing is no panic or crash
			assert.True(t,
				resp.StatusCode == fiber.StatusNotFound ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"Should handle long names gracefully")
		})
	}
}

// ============================================================================
// Task 3: SQL Injection Prevention Tests (AC: #2)
// ============================================================================

func TestCreateCoupon_SQLInjection(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	for _, payload := range sqlInjectionPayloads {
		t.Run(payload, func(t *testing.T) {
			cleanupTables(t)

			body, _ := json.Marshal(map[string]interface{}{
				"name":   payload,
				"amount": 100,
			})

			req := httptest.NewRequest("POST", "/api/coupons", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should either succeed (safely stored) or fail validation
			// The key is no SQL injection should occur
			assert.True(t,
				resp.StatusCode == fiber.StatusCreated ||
					resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"SQL injection payload should be handled safely, got status %d", resp.StatusCode)

			// Verify tables still exist (injection didn't drop them)
			verifyTablesExist(t)
		})
	}
}

func TestGetCoupon_SQLInjection(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	// First create a valid coupon
	createValidCoupon(t, app, "valid_coupon", 100)

	for _, payload := range sqlInjectionPayloads {
		t.Run(payload, func(t *testing.T) {
			// URL-encode the payload to create valid HTTP request
			encodedPayload := url.PathEscape(payload)
			req := httptest.NewRequest("GET", "/api/coupons/"+encodedPayload, nil)

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 404 (not found) - injection should not bypass security
			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"SQL injection in GET should return 404")

			// Verify tables still exist
			verifyTablesExist(t)
		})
	}
}

func TestClaimCoupon_SQLInjection(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	// Create a valid coupon first
	createValidCoupon(t, app, "valid_coupon", 100)

	testCases := []struct {
		name       string
		couponName string
		userID     string
	}{
		{"injection_in_coupon_name", sqlInjectionPayloads[0], "user1"},
		{"injection_in_user_id", "valid_coupon", sqlInjectionPayloads[0]},
		{"injection_in_both", sqlInjectionPayloads[1], sqlInjectionPayloads[2]},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"coupon_name": tc.couponName,
				"user_id":     tc.userID,
			})

			req := httptest.NewRequest("POST", "/api/coupons/claim", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 404 (not found) or 200 (if actually claiming valid coupon with injection user)
			// The key is no SQL injection occurs
			assert.True(t,
				resp.StatusCode == fiber.StatusNotFound ||
					resp.StatusCode == fiber.StatusOK ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"SQL injection should be handled safely")

			// Verify tables still exist
			verifyTablesExist(t)
		})
	}
}

// ============================================================================
// Task 4: Special Character Handling Tests (AC: #3)
// ============================================================================

func TestCreateCoupon_SpecialCharacters(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	for _, tc := range specialCharPayloads {
		t.Run(tc.name, func(t *testing.T) {
			cleanupTables(t)

			body, _ := json.Marshal(map[string]interface{}{
				"name":   tc.payload,
				"amount": 100,
			})

			req := httptest.NewRequest("POST", "/api/coupons", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Either accept safely or reject clearly - no crashes
			assert.True(t,
				resp.StatusCode == fiber.StatusCreated ||
					resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"Special chars should be handled safely, got %d for %s",
				resp.StatusCode, tc.name)

			// If created, verify we can retrieve it
			if resp.StatusCode == fiber.StatusCreated {
				// URL-encode the payload for GET request
				encodedPayload := url.PathEscape(tc.payload)
				getReq := httptest.NewRequest("GET", "/api/coupons/"+encodedPayload, nil)
				getResp, err := app.Test(getReq, -1)
				require.NoError(t, err)
				defer getResp.Body.Close()

				// Should be able to retrieve or get 404 (URL decoding differences)
				assert.True(t,
					getResp.StatusCode == fiber.StatusOK ||
						getResp.StatusCode == fiber.StatusNotFound,
					"Should handle special char retrieval")
			}
		})
	}
}

func TestClaimCoupon_SpecialCharacters(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	for _, tc := range specialCharPayloads {
		t.Run(tc.name+"_in_user_id", func(t *testing.T) {
			cleanupTables(t)

			// First create a valid coupon
			createValidCoupon(t, app, "test_coupon", 100)

			body, _ := json.Marshal(map[string]interface{}{
				"coupon_name": "test_coupon",
				"user_id":     tc.payload,
			})

			req := httptest.NewRequest("POST", "/api/coupons/claim", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Either succeed or fail gracefully - no crashes
			assert.True(t,
				resp.StatusCode == fiber.StatusOK ||
					resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"Special chars in user_id should be handled safely")
		})
	}
}

// ============================================================================
// Task 5: Amount Field Boundary Tests (AC: #4)
// ============================================================================

func TestCreateCoupon_AmountBoundary(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	testCases := []struct {
		name           string
		amount         interface{} // Use interface{} to test different types
		expectedStatus int
		description    string
	}{
		{"amount_zero", 0, fiber.StatusBadRequest, "Zero should be rejected (gte=1)"},
		{"amount_negative", -1, fiber.StatusBadRequest, "Negative should be rejected"},
		{"amount_negative_large", -100, fiber.StatusBadRequest, "Large negative should be rejected"},
		{"amount_one", 1, fiber.StatusCreated, "Minimum valid (1) should succeed"},
		{"amount_positive", 100, fiber.StatusCreated, "Normal positive should succeed"},
		{"amount_max_int32", math.MaxInt32, fiber.StatusCreated, "MaxInt32 should succeed"},
		{"amount_float", 1.5, fiber.StatusBadRequest, "Float should be rejected or truncated"},
		{"amount_string", "100", fiber.StatusBadRequest, "String type should be rejected"},
		{"amount_null", nil, fiber.StatusBadRequest, "Null should be rejected (required)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanupTables(t)

			payload := map[string]interface{}{
				"name": "test_coupon_" + tc.name,
			}

			// Only add amount if not nil (to test missing field)
			if tc.amount != nil {
				payload["amount"] = tc.amount
			}

			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/coupons", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// For float, Fiber might truncate or reject - both are acceptable
			if tc.name == "amount_float" {
				assert.True(t,
					resp.StatusCode == fiber.StatusCreated ||
						resp.StatusCode == fiber.StatusBadRequest,
					"Float handling should be consistent")
			} else {
				assert.Equal(t, tc.expectedStatus, resp.StatusCode,
					"Expected status %d for %s, got %d",
					tc.expectedStatus, tc.description, resp.StatusCode)
			}
		})
	}
}

func TestCreateCoupon_AmountOverflow(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	// Test MaxInt64 + 1 via raw JSON (overflow)
	overflowPayloads := []struct {
		name    string
		rawJSON string
	}{
		{
			"max_int64_overflow",
			`{"name": "overflow_test", "amount": 9223372036854775808}`, // MaxInt64 + 1
		},
		{
			"extremely_large",
			`{"name": "overflow_test2", "amount": 99999999999999999999999999999}`,
		},
	}

	for _, tc := range overflowPayloads {
		t.Run(tc.name, func(t *testing.T) {
			cleanupTables(t)

			req := httptest.NewRequest("POST", "/api/coupons", strings.NewReader(tc.rawJSON))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should reject with 400 (JSON parsing error or validation error)
			assert.True(t,
				resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"Overflow should be rejected, got %d", resp.StatusCode)
		})
	}
}

// ============================================================================
// Task 6: Malformed JSON and Request Size Tests (AC: #5)
// ============================================================================

func TestCreateCoupon_MalformedJSON(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	malformedPayloads := []struct {
		name    string
		body    string
		isValid bool
	}{
		{"completely_invalid", `{invalid}`, false},
		{"truncated_json", `{"name": "test"`, false},
		{"missing_closing_brace", `{"name": "test", "amount": 100`, false},
		{"extra_comma", `{"name": "test", "amount": 100,}`, false},
		{"single_quotes", `{'name': 'test', 'amount': 100}`, false},
		{"unquoted_keys", `{name: "test", amount: 100}`, false},
		{"trailing_data", `{"name": "test", "amount": 100}garbage`, false},
		{"empty_body", ``, false},
		{"just_brackets", `{}`, false}, // Valid JSON but missing required fields
		{"null_json", `null`, false},
		{"array_instead_of_object", `[1, 2, 3]`, false},
		{"number_instead_of_object", `42`, false},
		{"string_instead_of_object", `"hello"`, false},
	}

	for _, tc := range malformedPayloads {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/coupons", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// All malformed JSON should return 400
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode,
				"Malformed JSON should return 400, got %d for %s", resp.StatusCode, tc.name)
		})
	}
}

func TestCreateCoupon_WrongContentType(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	contentTypes := []struct {
		name        string
		contentType string
		body        string
	}{
		{"form_urlencoded", "application/x-www-form-urlencoded", "name=test&amount=100"},
		{"multipart_form", "multipart/form-data", "name=test&amount=100"},
		{"text_plain", "text/plain", `{"name": "test", "amount": 100}`},
		{"text_html", "text/html", `{"name": "test", "amount": 100}`},
		{"no_content_type", "", `{"name": "test", "amount": 100}`},
	}

	for _, tc := range contentTypes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/coupons", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Wrong content type should return 400 or succeed if Fiber parses it
			// The key is no crashes
			assert.True(t,
				resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusCreated,
				"Wrong content type should be handled gracefully")
		})
	}
}

func TestCreateCoupon_LargePayload(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	payloadSizes := []struct {
		name          string
		sizeKB        int
		expectedLimit bool // true if we expect it to be rejected
	}{
		{"100KB", 100, false},
		{"500KB", 500, false},
		{"5MB", 5 * 1024, true}, // Should exceed 4MB limit
	}

	for _, tc := range payloadSizes {
		t.Run(tc.name, func(t *testing.T) {
			cleanupTables(t)

			// Create a large JSON payload
			var largeData strings.Builder
			largeData.WriteString(`{"name": "test_coupon_large", "amount": 100, "extra": "`)

			targetSize := tc.sizeKB * 1024

			// Fill with data
			for largeData.Len() < targetSize {
				largeData.WriteString("A")
			}
			largeData.WriteString(`"}`)

			req := httptest.NewRequest("POST", "/api/coupons", strings.NewReader(largeData.String()))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)

			if tc.expectedLimit {
				// For oversized payloads, either an error is returned or a 413/400 status
				if err != nil {
					// This is expected - body size exceeds limit
					assert.Contains(t, err.Error(), "body size exceeds",
						"Expected body size limit error")
				} else {
					defer resp.Body.Close()
					assert.True(t,
						resp.StatusCode == fiber.StatusRequestEntityTooLarge ||
							resp.StatusCode == fiber.StatusBadRequest,
						"Large payload should be rejected, got %d", resp.StatusCode)
				}
			} else {
				require.NoError(t, err)
				defer resp.Body.Close()
				// Should process normally - will likely fail on create since extra field exists
				// but that's fine, the key is no crash or resource exhaustion
				assert.True(t,
					resp.StatusCode == fiber.StatusCreated ||
						resp.StatusCode == fiber.StatusBadRequest ||
						resp.StatusCode == fiber.StatusConflict ||
						resp.StatusCode == fiber.StatusInternalServerError,
					"Normal payload should be processed, got %d", resp.StatusCode)
			}
		})
	}
}

func TestCreateCoupon_DeeplyNestedJSON(t *testing.T) {
	cleanupTables(t)
	app := setupTestApp(t)

	testCases := []struct {
		name  string
		depth int
	}{
		{"depth_10", 10},
		{"depth_50", 50},
		{"depth_100", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build deeply nested JSON
			var nested strings.Builder
			for i := 0; i < tc.depth; i++ {
				nested.WriteString(`{"nested":`)
			}
			nested.WriteString(`{"name": "test", "amount": 100}`)
			for i := 0; i < tc.depth; i++ {
				nested.WriteString(`}`)
			}

			req := httptest.NewRequest("POST", "/api/coupons", strings.NewReader(nested.String()))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should handle gracefully - either reject or fail validation
			assert.True(t,
				resp.StatusCode == fiber.StatusBadRequest ||
					resp.StatusCode == fiber.StatusInternalServerError,
				"Deeply nested JSON should be handled gracefully, got %d", resp.StatusCode)
		})
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// verifyTablesExist checks that the coupons and claims tables still exist.
func verifyTablesExist(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check coupons table
	var couponsExists bool
	err := testPool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'coupons'
		)
	`).Scan(&couponsExists)
	require.NoError(t, err)
	assert.True(t, couponsExists, "coupons table should still exist")

	// Check claims table
	var claimsExists bool
	err = testPool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'claims'
		)
	`).Scan(&claimsExists)
	require.NoError(t, err)
	assert.True(t, claimsExists, "claims table should still exist")
}

// createValidCoupon creates a valid coupon for testing.
func createValidCoupon(t *testing.T, app *fiber.App, name string, amount int) {
	t.Helper()

	body, _ := json.Marshal(map[string]interface{}{
		"name":   name,
		"amount": amount,
	})

	req := httptest.NewRequest("POST", "/api/coupons", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read body to ensure it's fully consumed
	_, _ = io.ReadAll(resp.Body)

	require.Equal(t, fiber.StatusCreated, resp.StatusCode,
		"Failed to create test coupon %s", name)
}
