package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/scalable-coupon-system/internal/service"
)

// mockClaimService is a mock implementation of ClaimServiceInterface.
type mockClaimService struct {
	claimCouponFn func(ctx context.Context, userID, couponName string) error
}

func (m *mockClaimService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
	if m.claimCouponFn != nil {
		return m.claimCouponFn(ctx, userID, couponName)
	}
	return nil
}

func setupClaimTestApp(mockSvc *mockClaimService) *fiber.App {
	app := fiber.New()
	validate := validator.New()
	h := NewClaimHandler(mockSvc, validate)
	app.Post("/api/coupons/claim", h.ClaimCoupon)
	return app
}

func TestClaimCoupon_Success(t *testing.T) {
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			return nil
		},
	}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected 200 OK")

	// Verify empty body
	respBody, _ := io.ReadAll(resp.Body)
	assert.Empty(t, respBody, "Response body should be empty on success")
}

func TestClaimCoupon_DuplicateClaim(t *testing.T) {
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			return service.ErrAlreadyClaimed
		},
	}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusConflict, resp.StatusCode, "Expected 409 Conflict")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon already claimed by user", result["error"], "Exact error message required")
}

func TestClaimCoupon_OutOfStock(t *testing.T) {
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			return service.ErrNoStock
		},
	}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_999", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon out of stock", result["error"], "Exact error message required")
}

func TestClaimCoupon_CouponNotFound(t *testing.T) {
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			return service.ErrCouponNotFound
		},
	}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_001", "coupon_name": "NONEXISTENT"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode, "Expected 404 Not Found")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "coupon not found", result["error"], "Exact error message required")
}

func TestClaimCoupon_MissingUserID(t *testing.T) {
	mockSvc := &mockClaimService{}
	app := setupClaimTestApp(mockSvc)

	body := `{"coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: user_id is required", result["error"], "Exact error message required")
}

func TestClaimCoupon_MissingCouponName(t *testing.T) {
	mockSvc := &mockClaimService{}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_001"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request: coupon_name is required", result["error"], "Exact error message required")
}

func TestClaimCoupon_MalformedJSON(t *testing.T) {
	mockSvc := &mockClaimService{}
	app := setupClaimTestApp(mockSvc)

	body := `{not valid json}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", result["error"], "Exact error message required")
}

func TestClaimCoupon_InternalServerError(t *testing.T) {
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			return errors.New("database connection failed")
		},
	}
	app := setupClaimTestApp(mockSvc)

	body := `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", result["error"], "Exact error message required")
}

func TestClaimCoupon_EmptyBody(t *testing.T) {
	mockSvc := &mockClaimService{}
	app := setupClaimTestApp(mockSvc)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	// Either user_id or coupon_name will be reported first
	assert.Contains(t, result["error"], "invalid request:", "Error should start with 'invalid request:'")
}

func TestClaimCoupon_RequestFieldsSnakeCase(t *testing.T) {
	var capturedUserID, capturedCouponName string
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			capturedUserID = userID
			capturedCouponName = couponName
			return nil
		},
	}
	app := setupClaimTestApp(mockSvc)

	// Use snake_case field names as per AC
	body := `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "user_001", capturedUserID, "user_id should be captured correctly")
	assert.Equal(t, "PROMO_SUPER", capturedCouponName, "coupon_name should be captured correctly")
}

// Edge case tests for claim validation
func TestClaimCoupon_UnicodeUserID(t *testing.T) {
	var capturedUserID string
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			capturedUserID = userID
			return nil
		},
	}
	app := setupClaimTestApp(mockSvc)

	// Test with unicode user ID
	body := `{"user_id": "用户_001_🎉", "coupon_name": "PROMO_SUPER"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "用户_001_🎉", capturedUserID, "Unicode user_id should be preserved")
}

func TestClaimCoupon_SpecialCharactersInCouponName(t *testing.T) {
	var capturedCouponName string
	mockSvc := &mockClaimService{
		claimCouponFn: func(ctx context.Context, userID, couponName string) error {
			capturedCouponName = couponName
			return nil
		},
	}
	app := setupClaimTestApp(mockSvc)

	// Test with special characters that could be problematic
	body := `{"user_id": "user_001", "coupon_name": "PROMO-100%_OFF!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/coupons/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "PROMO-100%_OFF!", capturedCouponName, "Special characters should be preserved")
}
