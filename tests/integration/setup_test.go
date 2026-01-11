//go:build integration

// Package integration contains integration tests that run against the real docker-compose infrastructure.
// These tests verify the system's HTTP API behavior end-to-end.
//
// Usage:
//   docker-compose up -d                                     # Start services
//   go test -v -race -tags integration ./tests/integration/... # Run tests
//   docker-compose down                                       # Cleanup
//
// Environment Variables:
//   TEST_SERVER_URL  - API server URL (default: http://localhost:3000)
//   TEST_DB_URL      - Database URL (default: postgres://postgres:postgres@localhost:5432/coupon_db?sslmode=disable)
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testPool   *pgxpool.Pool
	testServer string // The base URL for the test server (e.g., "http://localhost:3000")
	httpClient *http.Client
)

func TestMain(m *testing.M) {
	// Get server URL from environment or use default (docker-compose API)
	testServer = os.Getenv("TEST_SERVER_URL")
	if testServer == "" {
		testServer = "http://localhost:3000"
	}

	// Get database URL from environment or use default (docker-compose PostgreSQL)
	databaseURL := os.Getenv("TEST_DB_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/coupon_db?sslmode=disable"
	}

	log.Printf("Integration test configuration:")
	log.Printf("  Server URL: %s", testServer)
	log.Printf("  Database URL: %s", databaseURL)

	// Connect to the database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	testPool, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Verify database connection
	if err := testPool.Ping(ctx); err != nil {
		log.Fatalf("Could not ping database: %s", err)
	}
	log.Println("Database connection established")

	// Verify server is running by hitting the health endpoint
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Wait for server to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := httpClient.Get(testServer + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Println("Server is ready")
				break
			}
		}
		if i == maxRetries-1 {
			log.Fatalf("Server not responding at %s after %d retries. Ensure docker-compose is running.", testServer, maxRetries)
		}
		log.Printf("Waiting for server... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(1 * time.Second)
	}

	code := m.Run()

	// Cleanup
	testPool.Close()

	os.Exit(code)
}

func cleanupTables(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE claims, coupons CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup tables: %v", err)
	}
}

// Helper function to make POST requests with JSON body
func postJSON(url string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return httpClient.Do(req)
}

// Helper function to make GET requests
func getJSON(url string) (*http.Response, error) {
	return httpClient.Get(url)
}

// Helper function to read response body as JSON
func readJSONResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// formatURL creates a full URL from the test server base and a path
func formatURL(path string) string {
	return fmt.Sprintf("%s%s", testServer, path)
}

// createTestCoupon creates a coupon directly in the database for testing
func createTestCoupon(t *testing.T, name string, amount int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx,
		"INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $2)",
		name, amount)
	if err != nil {
		t.Fatalf("Failed to create test coupon: %v", err)
	}
}

// getCouponFromDB retrieves coupon data directly from the database
func getCouponFromDB(t *testing.T, name string) (remainingAmount int, claimCount int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := testPool.QueryRow(ctx,
		"SELECT remaining_amount FROM coupons WHERE name = $1",
		name).Scan(&remainingAmount)
	if err != nil {
		t.Fatalf("Failed to get coupon remaining_amount: %v", err)
	}

	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
		name).Scan(&claimCount)
	if err != nil {
		t.Fatalf("Failed to get claim count: %v", err)
	}

	return remainingAmount, claimCount
}
