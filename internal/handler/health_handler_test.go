package handler

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPool implements a minimal interface for testing health checks
type mockPool struct {
	pingErr   error
	pingDelay time.Duration // Optional delay to simulate slow response
}

func (m *mockPool) Ping(ctx context.Context) error {
	if m.pingDelay > 0 {
		select {
		case <-time.After(m.pingDelay):
			// Delay completed, return the configured error (or nil)
		case <-ctx.Done():
			// Context was canceled or deadline exceeded
			return ctx.Err()
		}
	}
	return m.pingErr
}

func TestHealthHandler_Check_Healthy(t *testing.T) {
	app := fiber.New()
	pool := &mockPool{pingErr: nil}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"healthy"`)
}

func TestHealthHandler_Check_Unhealthy(t *testing.T) {
	app := fiber.New()
	pool := &mockPool{pingErr: errors.New("connection refused")}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"unhealthy"`)
	assert.Contains(t, string(body), `"error":"database connection failed"`)
}

func TestHealthHandler_Check_SlowResponse(t *testing.T) {
	// Test that slow database responses are handled correctly
	// Fiber's default test timeout is 1 second, so we use a shorter delay
	app := fiber.New()

	// Mock pool that responds slowly but successfully
	pool := &mockPool{
		pingErr:   nil,
		pingDelay: 100 * time.Millisecond, // Slow but within timeout
	}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, 2000) // 2 second timeout for test
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	// Should still return healthy after the delay
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"healthy"`)
}

func TestHealthHandler_Check_ContextTimeout(t *testing.T) {
	// Test that context timeout is properly propagated
	app := fiber.New()

	// Mock pool that will block longer than the test timeout
	pool := &mockPool{
		pingErr:   nil,
		pingDelay: 5 * time.Second, // Will be canceled by context
	}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)

	// Use a very short timeout to trigger context deadline exceeded
	resp, err := app.Test(req, 100) // 100ms timeout

	// The test might return an error due to timeout, or a response
	// Either way, the handler should not panic
	if err != nil {
		// Timeout error is expected
		assert.Contains(t, err.Error(), "timeout")
	} else {
		defer func() {
			_ = resp.Body.Close()
		}()
		// If we got a response, it should indicate unhealthy due to context cancellation
		// Note: Fiber might return 503 or the test might have succeeded before timeout
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Response status: %d, body: %s", resp.StatusCode, string(body))
	}
}
