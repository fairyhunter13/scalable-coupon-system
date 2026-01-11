package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPool_ContextCancellation(t *testing.T) {
	// Test that NewPool respects context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pool, err := NewPool(ctx, "postgres://invalid:invalid@localhost:9999/invalid", 3)
	assert.Nil(t, pool)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNewPool_InvalidDSN(t *testing.T) {
	// Test that NewPool fails gracefully with invalid DSN
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a short retry count for faster test
	pool, err := NewPool(ctx, "postgres://invalid:invalid@localhost:9999/invalid", 1)
	assert.Nil(t, pool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect after")
}

func TestNewPool_ZeroRetries(t *testing.T) {
	// Test edge case: zero retries should still attempt once
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, "postgres://invalid:invalid@localhost:9999/invalid", 0)
	assert.Nil(t, pool)
	assert.Error(t, err)
}

func TestNewPool_ValidConnection(t *testing.T) {
	// Skip if no PostgreSQL available (integration test)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires a running PostgreSQL instance
	// It will be tested via docker-compose in manual verification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsn := "postgres://postgres:postgres@localhost:5432/coupon_db?sslmode=disable"
	pool, err := NewPool(ctx, dsn, 5)

	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	require.NotNil(t, pool)
	defer pool.Close()

	// Verify pool is functional
	err = pool.Ping(ctx)
	assert.NoError(t, err)
}
