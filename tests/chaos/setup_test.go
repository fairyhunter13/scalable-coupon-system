//go:build ci

// Package chaos contains CI-only chaos engineering tests for database resilience.
// These tests verify the system's behavior under database stress conditions
// including connection pool exhaustion, query timeouts, and connection drops.
//
// IMPORTANT: These tests are tagged with "ci" build constraint and should
// only run in CI environments where infrastructure is controlled.
// Use: go test -v -race -tags ci ./tests/chaos/...
package chaos

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var (
	// testPool is the standard connection pool for test operations
	testPool *pgxpool.Pool
	// databaseURL stores the connection string for creating custom pools
	databaseURL string
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=testpass",
			"POSTGRES_USER=testuser",
			"POSTGRES_DB=testdb",
			"listen_addresses='*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseURL = fmt.Sprintf("postgres://testuser:testpass@%s/testdb?sslmode=disable", hostAndPort)

	log.Println("Connecting to database on url:", databaseURL)

	// Tell docker to kill the container after 120 seconds (safety limit)
	_ = resource.Expire(120)

	// Retry connection
	pool.MaxWait = 120 * time.Second
	if err = pool.Retry(func() error {
		var err error
		testPool, err = pgxpool.New(context.Background(), databaseURL)
		if err != nil {
			return err
		}
		return testPool.Ping(context.Background())
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Run migrations
	if err := runMigrations(testPool); err != nil {
		log.Fatalf("Could not run migrations: %s", err)
	}

	code := m.Run()

	// Cleanup
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func runMigrations(pool *pgxpool.Pool) error {
	schema := `
		CREATE TABLE IF NOT EXISTS coupons (
			name VARCHAR(255) PRIMARY KEY,
			amount INTEGER NOT NULL CHECK (amount > 0),
			remaining_amount INTEGER NOT NULL CHECK (remaining_amount >= 0),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS claims (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(user_id, coupon_name)
		);

		CREATE INDEX IF NOT EXISTS idx_claims_coupon_name ON claims(coupon_name);
	`
	_, err := pool.Exec(context.Background(), schema)
	return err
}

func cleanupTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE claims, coupons CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup tables: %v", err)
	}
}

// createPoolWithConfig creates a new pgxpool with custom configuration.
// This allows tests to create pools with specific settings like low max_conns
// for exhaustion testing.
func createPoolWithConfig(ctx context.Context, maxConns int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = maxConns
	config.MinConns = 1
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	return pgxpool.NewWithConfig(ctx, config)
}
