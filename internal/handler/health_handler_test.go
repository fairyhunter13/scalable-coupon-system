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

func TestHealthHandler_Check_ContextCanceled(t *testing.T) {
	// Test that context cancellation is properly handled
	// We simulate a canceled context by having the mock return context.Canceled
	app := fiber.New()

	// Mock pool that returns context.Canceled error (simulates canceled context)
	pool := &mockPool{
		pingErr: context.Canceled,
	}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	// Should return 503 unhealthy when ping fails due to context cancellation
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"unhealthy"`)
}

func TestHealthHandler_Check_DeadlineExceeded(t *testing.T) {
	// Test that context deadline exceeded is properly handled
	app := fiber.New()

	// Mock pool that returns context.DeadlineExceeded error
	pool := &mockPool{
		pingErr: context.DeadlineExceeded,
	}
	handler := NewHealthHandler(pool)
	app.Get("/health", handler.Check)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()

	// Should return 503 unhealthy when ping times out
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"unhealthy"`)
}
