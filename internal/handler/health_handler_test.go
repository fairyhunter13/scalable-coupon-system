package handler

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPool implements a minimal interface for testing health checks
type mockPool struct {
	pingErr error
}

func (m *mockPool) Ping(ctx context.Context) error {
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
