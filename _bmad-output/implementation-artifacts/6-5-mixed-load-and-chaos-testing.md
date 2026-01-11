# Story 6.5: Mixed Load & Chaos Testing

Status: done

## Story

As a **maintainer**,
I want **CI-only chaos tests combining simultaneous create/claim/get operations, zero-stock stampedes, and constraint violation storms**,
So that **system stability is proven under realistic chaotic load patterns**.

## Acceptance Criteria

1. **AC1: Mixed Operation Load Test**
   **Given** the CI pipeline runs the mixed load chaos test job
   **When** 50 concurrent requests mix CREATE, CLAIM, and GET operations randomly
   **Then** all operations complete with appropriate status codes
   **And** no race conditions or data corruption occurs
   **And** database remains in consistent state after test

2. **AC2: Zero-Stock Stampede Test**
   **Given** the CI pipeline runs the zero-stock stampede test
   **When** a coupon with stock=1 receives 100 concurrent claim attempts
   **Then** exactly 1 claim succeeds
   **And** exactly 99 claims fail with 400 out of stock
   **And** no 500 errors or panics occur

3. **AC3: Constraint Violation Storm Test**
   **Given** the CI pipeline runs the constraint violation storm test
   **When** 50 concurrent requests attempt duplicate claims (same user, same coupon)
   **Then** exactly 1 claim succeeds
   **And** exactly 49 claims fail with 409 Conflict
   **And** unique constraint is never violated
   **And** no database errors leak to client

4. **AC4: Interleaved Create and Claim Operations**
   **Given** the CI pipeline runs the mixed load chaos test job
   **When** CREATE and CLAIM operations interleave for the same coupon name
   **Then** operations are serialized correctly
   **And** claims only succeed after coupon exists
   **And** no orphan claims are created

5. **AC5: CI-Only Build Tags and Randomization**
   **Given** the mixed load chaos tests
   **When** I review the test files
   **Then** tests are tagged with `//go:build ci` to prevent local execution
   **And** tests use randomized operation sequences for realistic chaos

## Tasks / Subtasks

- [x] Task 1: Create mixed load chaos test file (AC: #5)
  - [x] 1.1: Create `tests/chaos/mixed_load_test.go` with `//go:build ci` tag
  - [x] 1.2: Import required packages (testify, pgx, context, sync, rand, math/rand)
  - [x] 1.3: Add comprehensive test documentation header explaining each chaos scenario
  - [x] 1.4: Implement helper functions for randomized operation selection

- [x] Task 2: Implement mixed operation load test (AC: #1)
  - [x] 2.1: Create `TestMixedOperationLoad` function
  - [x] 2.2: Define operation types: CREATE, CLAIM, GET with random selection
  - [x] 2.3: Launch 50 concurrent goroutines with randomized operations
  - [x] 2.4: Track operation results by type (success/failure counts)
  - [x] 2.5: Verify database consistency: coupon counts match, no orphan claims
  - [x] 2.6: Verify no data corruption via integrity queries

- [x] Task 3: Implement zero-stock stampede test (AC: #2)
  - [x] 3.1: Create `TestZeroStockStampede` function
  - [x] 3.2: Create coupon with stock=1, launch 100 concurrent claims
  - [x] 3.3: Verify exactly 1 success, 99 failures (ErrNoStock)
  - [x] 3.4: Verify no 500 errors or panics (only 200/201 or 400)
  - [x] 3.5: Verify remaining_amount = 0, never negative

- [x] Task 4: Implement constraint violation storm test (AC: #3)
  - [x] 4.1: Create `TestConstraintViolationStorm` function
  - [x] 4.2: Create coupon with stock=100, launch 50 same-user concurrent claims
  - [x] 4.3: Verify exactly 1 success, 49 failures (ErrAlreadyClaimed / 409)
  - [x] 4.4: Verify UNIQUE constraint never violated (only 1 claim record)
  - [x] 4.5: Verify no raw database errors leak to client

- [x] Task 5: Implement interleaved create-claim test (AC: #4)
  - [x] 5.1: Create `TestInterleavedCreateClaim` function
  - [x] 5.2: Launch concurrent CREATE and CLAIM for same coupon name
  - [x] 5.3: Verify claims fail with 404 before coupon exists
  - [x] 5.4: Verify claims succeed after coupon created
  - [x] 5.5: Verify no orphan claims (all claims reference existing coupons)

- [x] Task 6: Integration with CI workflow (AC: #5)
  - [x] 6.1: Ensure tests run with `-tags ci` flag in CI pipeline
  - [x] 6.2: Verify tests excluded from default `go test ./...`
  - [x] 6.3: Add appropriate timeout for chaos tests (e.g., 60s per test)
  - [x] 6.4: Verify randomization seeded for reproducibility in CI logs

## Dev Notes

### CI-ONLY Build Tag Pattern

All tests MUST use `//go:build ci` build constraint (established in Stories 6.1, 6.2, 6.3, 6.4):

```go
//go:build ci

// Package chaos contains CI-only chaos engineering tests.
// This file implements mixed load and chaos testing scenarios:
// - Mixed operation load (CREATE/CLAIM/GET interleaved)
// - Zero-stock stampede (single stock, massive concurrency)
// - Constraint violation storm (duplicate claim attempts)
// - Interleaved create-claim operations
//
// These tests verify system stability under realistic chaotic load patterns.
// Use: go test -v -race -tags ci ./tests/chaos/...
package chaos

import (
    "context"
    "errors"
    "fmt"
    "math/rand"
    "runtime"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/fairyhunter13/scalable-coupon-system/internal/repository"
    "github.com/fairyhunter13/scalable-coupon-system/internal/service"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

### Test File Location

Place in `tests/chaos/` directory (created in Story 6.2):

```
tests/
├── integration/                    # Existing - API endpoint tests
├── stress/                         # Existing - Flash Sale, Double Dip, Scale
│   ├── flash_sale_test.go
│   ├── double_dip_test.go
│   └── scale_test.go               # Story 6.1 (CI-only)
└── chaos/                          # Epic 6 chaos engineering tests
    ├── setup_test.go               # Test infrastructure (Story 6.2)
    ├── db_resilience_test.go       # Story 6.2 tests
    ├── input_boundary_test.go      # Story 6.3 tests (if exists)
    ├── transaction_edge_cases_test.go  # Story 6.4 tests
    └── mixed_load_test.go          # THIS STORY
```

### Mixed Operation Load Test Pattern

**Randomized Operation Selection:**

```go
type OperationType int

const (
    OpCreate OperationType = iota
    OpClaim
    OpGet
)

func (o OperationType) String() string {
    switch o {
    case OpCreate:
        return "CREATE"
    case OpClaim:
        return "CLAIM"
    case OpGet:
        return "GET"
    default:
        return "UNKNOWN"
    }
}

func TestMixedOperationLoad(t *testing.T) {
    cleanupTables(t)

    const (
        concurrentOps = 50
        timeout       = 60 * time.Second
    )

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Seed random for reproducibility (log the seed for debugging)
    seed := time.Now().UnixNano()
    rng := rand.New(rand.NewSource(seed))
    t.Logf("Random seed: %d (use for reproducing failures)", seed)

    // Create service
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    svc := service.NewCouponService(testPool, couponRepo, claimRepo)

    // Pre-create some coupons for GET/CLAIM operations
    baseCoupons := []string{"CHAOS_BASE_1", "CHAOS_BASE_2", "CHAOS_BASE_3"}
    for _, name := range baseCoupons {
        _, err := testPool.Exec(ctx,
            "INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
            name, 100, 100)
        require.NoError(t, err)
    }

    // Track results by operation type
    var createSuccess, createFail int32
    var claimSuccess, claimFail int32
    var getSuccess, getFail int32

    var wg sync.WaitGroup

    for i := 0; i < concurrentOps; i++ {
        wg.Add(1)
        go func(opID int) {
            defer wg.Done()

            opCtx, opCancel := context.WithTimeout(ctx, 10*time.Second)
            defer opCancel()

            // Random operation selection (weighted: 20% CREATE, 50% CLAIM, 30% GET)
            roll := rng.Intn(100)
            var op OperationType
            switch {
            case roll < 20:
                op = OpCreate
            case roll < 70:
                op = OpClaim
            default:
                op = OpGet
            }

            switch op {
            case OpCreate:
                couponName := fmt.Sprintf("CHAOS_NEW_%d", opID)
                err := svc.Create(opCtx, &model.CreateCouponRequest{
                    Name:   couponName,
                    Amount: intPtr(50),
                })
                if err == nil {
                    atomic.AddInt32(&createSuccess, 1)
                } else {
                    atomic.AddInt32(&createFail, 1)
                }

            case OpClaim:
                // Random coupon from base set
                couponName := baseCoupons[rng.Intn(len(baseCoupons))]
                userID := fmt.Sprintf("chaos_user_%d", opID)
                err := svc.ClaimCoupon(opCtx, userID, couponName)
                if err == nil {
                    atomic.AddInt32(&claimSuccess, 1)
                } else {
                    atomic.AddInt32(&claimFail, 1)
                }

            case OpGet:
                couponName := baseCoupons[rng.Intn(len(baseCoupons))]
                _, err := svc.GetByName(opCtx, couponName)
                if err == nil {
                    atomic.AddInt32(&getSuccess, 1)
                } else {
                    atomic.AddInt32(&getFail, 1)
                }
            }
        }(i)
    }

    wg.Wait()

    t.Logf("Results - CREATE: %d/%d, CLAIM: %d/%d, GET: %d/%d",
        createSuccess, createSuccess+createFail,
        claimSuccess, claimSuccess+claimFail,
        getSuccess, getSuccess+getFail)

    // Verify database consistency
    var couponCount, claimCount int
    err := testPool.QueryRow(ctx, "SELECT COUNT(*) FROM coupons").Scan(&couponCount)
    require.NoError(t, err)
    err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM claims").Scan(&claimCount)
    require.NoError(t, err)

    t.Logf("Database state - Coupons: %d, Claims: %d", couponCount, claimCount)

    // Verify no orphan claims (all claims reference existing coupons)
    var orphanClaims int
    err = testPool.QueryRow(ctx, `
        SELECT COUNT(*) FROM claims c
        LEFT JOIN coupons p ON c.coupon_name = p.name
        WHERE p.name IS NULL
    `).Scan(&orphanClaims)
    require.NoError(t, err)
    assert.Equal(t, 0, orphanClaims, "No orphan claims should exist")

    // Verify stock consistency (remaining_amount >= 0 for all coupons)
    var negativeStock int
    err = testPool.QueryRow(ctx,
        "SELECT COUNT(*) FROM coupons WHERE remaining_amount < 0").Scan(&negativeStock)
    require.NoError(t, err)
    assert.Equal(t, 0, negativeStock, "No coupon should have negative stock")
}

func intPtr(i int) *int {
    return &i
}
```

### Zero-Stock Stampede Test Pattern

**Critical: Test the extreme edge case of stock=1:**

```go
func TestZeroStockStampede(t *testing.T) {
    cleanupTables(t)

    const (
        couponName     = "STAMPEDE_TEST"
        availableStock = 1  // Critical: single stock for stampede
        concurrentReqs = 100
        timeout        = 60 * time.Second
    )

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Setup: Create coupon with stock=1
    _, err := testPool.Exec(ctx,
        "INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
        couponName, availableStock, availableStock)
    require.NoError(t, err)

    // Create service
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    svc := service.NewCouponService(testPool, couponRepo, claimRepo)

    // Launch stampede
    var wg sync.WaitGroup
    results := make(chan error, concurrentReqs)

    for i := 0; i < concurrentReqs; i++ {
        wg.Add(1)
        go func(userID string) {
            defer wg.Done()
            err := svc.ClaimCoupon(ctx, userID, couponName)
            results <- err
        }(fmt.Sprintf("stampede_user_%d", i))
    }

    wg.Wait()
    close(results)

    // Collect results
    var successes, noStock, serverErrors, otherErrors int
    for err := range results {
        switch {
        case err == nil:
            successes++
        case errors.Is(err, service.ErrNoStock):
            noStock++
        case isServerError(err):
            serverErrors++
            t.Logf("SERVER ERROR (unexpected): %v", err)
        default:
            otherErrors++
            t.Logf("Other error: %v", err)
        }
    }

    t.Logf("Stampede results - Successes: %d, NoStock: %d, ServerErrors: %d, Other: %d",
        successes, noStock, serverErrors, otherErrors)

    // AC2: Exactly 1 success
    assert.Equal(t, 1, successes, "Exactly 1 claim should succeed")

    // AC2: Exactly 99 no-stock failures
    assert.Equal(t, concurrentReqs-1, noStock, "Rest should fail with no stock")

    // AC2: No 500 errors or panics
    assert.Equal(t, 0, serverErrors, "No server errors should occur")

    // Verify database state
    var remaining int
    err = testPool.QueryRow(ctx,
        "SELECT remaining_amount FROM coupons WHERE name = $1",
        couponName).Scan(&remaining)
    require.NoError(t, err)
    assert.Equal(t, 0, remaining, "remaining_amount should be exactly 0")
    assert.GreaterOrEqual(t, remaining, 0, "remaining_amount must never be negative")

    var claimCount int
    err = testPool.QueryRow(ctx,
        "SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
        couponName).Scan(&claimCount)
    require.NoError(t, err)
    assert.Equal(t, 1, claimCount, "Exactly 1 claim record should exist")
}

func isServerError(err error) bool {
    if err == nil {
        return false
    }
    errStr := err.Error()
    return strings.Contains(errStr, "500") ||
           strings.Contains(errStr, "internal") ||
           strings.Contains(errStr, "panic")
}
```

### Constraint Violation Storm Test Pattern

**Extends Double Dip pattern with higher concurrency:**

```go
func TestConstraintViolationStorm(t *testing.T) {
    cleanupTables(t)

    const (
        couponName     = "VIOLATION_STORM_TEST"
        availableStock = 100  // High stock to isolate constraint violations
        concurrentReqs = 50
        userID         = "storm_user"  // Same user for all requests
        timeout        = 60 * time.Second
    )

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Setup: Create coupon with plenty of stock
    _, err := testPool.Exec(ctx,
        "INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
        couponName, availableStock, availableStock)
    require.NoError(t, err)

    // Create service
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    svc := service.NewCouponService(testPool, couponRepo, claimRepo)

    // Launch constraint violation storm
    var wg sync.WaitGroup
    results := make(chan error, concurrentReqs)

    for i := 0; i < concurrentReqs; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := svc.ClaimCoupon(ctx, userID, couponName)
            results <- err
        }()
    }

    wg.Wait()
    close(results)

    // Collect results
    var successes, alreadyClaimed, rawDBErrors, otherErrors int
    for err := range results {
        switch {
        case err == nil:
            successes++
        case errors.Is(err, service.ErrAlreadyClaimed):
            alreadyClaimed++
        case isRawDatabaseError(err):
            rawDBErrors++
            t.Logf("RAW DB ERROR (should be wrapped): %v", err)
        default:
            otherErrors++
            t.Logf("Other error: %v", err)
        }
    }

    t.Logf("Storm results - Successes: %d, AlreadyClaimed: %d, RawDBErrors: %d, Other: %d",
        successes, alreadyClaimed, rawDBErrors, otherErrors)

    // AC3: Exactly 1 success
    assert.Equal(t, 1, successes, "Exactly 1 claim should succeed")

    // AC3: Exactly 49 constraint violations (ErrAlreadyClaimed)
    assert.Equal(t, concurrentReqs-1, alreadyClaimed,
        "Rest should fail with ErrAlreadyClaimed")

    // AC3: No raw database errors leaked
    assert.Equal(t, 0, rawDBErrors, "No raw database errors should leak to client")

    // Verify UNIQUE constraint held: exactly 1 claim record
    var claimCount int
    err = testPool.QueryRow(ctx,
        "SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
        userID, couponName).Scan(&claimCount)
    require.NoError(t, err)
    assert.Equal(t, 1, claimCount,
        "UNIQUE constraint must hold - exactly 1 claim record")

    // Verify remaining stock (only 1 deducted)
    var remaining int
    err = testPool.QueryRow(ctx,
        "SELECT remaining_amount FROM coupons WHERE name = $1",
        couponName).Scan(&remaining)
    require.NoError(t, err)
    assert.Equal(t, availableStock-1, remaining,
        "Only 1 stock should be deducted")
}

func isRawDatabaseError(err error) bool {
    if err == nil {
        return false
    }
    errStr := err.Error()
    // Check for PostgreSQL-specific error codes or raw constraint names
    return strings.Contains(errStr, "23505") || // unique_violation
           strings.Contains(errStr, "duplicate key") ||
           strings.Contains(errStr, "pq:") ||
           strings.Contains(errStr, "SQLSTATE")
}
```

### Interleaved Create-Claim Test Pattern

**Testing race between CREATE and CLAIM operations:**

```go
func TestInterleavedCreateClaim(t *testing.T) {
    cleanupTables(t)

    const (
        couponName     = "INTERLEAVE_TEST"
        availableStock = 50
        concurrentOps  = 30
        timeout        = 60 * time.Second
    )

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // Create service
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    svc := service.NewCouponService(testPool, couponRepo, claimRepo)

    // Launch interleaved CREATE and CLAIM operations
    var wg sync.WaitGroup

    // Track results
    var createSuccess, createFail int32
    var claimSuccess, claimNotFound, claimNoStock, claimOther int32

    // Half try to create, half try to claim
    for i := 0; i < concurrentOps; i++ {
        wg.Add(1)
        if i%2 == 0 {
            // CREATE operation
            go func() {
                defer wg.Done()
                err := svc.Create(ctx, &model.CreateCouponRequest{
                    Name:   couponName,
                    Amount: intPtr(availableStock),
                })
                if err == nil {
                    atomic.AddInt32(&createSuccess, 1)
                } else {
                    atomic.AddInt32(&createFail, 1)
                }
            }()
        } else {
            // CLAIM operation
            go func(userID string) {
                defer wg.Done()
                err := svc.ClaimCoupon(ctx, userID, couponName)
                switch {
                case err == nil:
                    atomic.AddInt32(&claimSuccess, 1)
                case errors.Is(err, service.ErrCouponNotFound):
                    atomic.AddInt32(&claimNotFound, 1)
                case errors.Is(err, service.ErrNoStock):
                    atomic.AddInt32(&claimNoStock, 1)
                default:
                    atomic.AddInt32(&claimOther, 1)
                }
            }(fmt.Sprintf("interleave_user_%d", i))
        }
    }

    wg.Wait()

    t.Logf("CREATE results - Success: %d, Fail: %d", createSuccess, createFail)
    t.Logf("CLAIM results - Success: %d, NotFound: %d, NoStock: %d, Other: %d",
        claimSuccess, claimNotFound, claimNoStock, claimOther)

    // AC4: Exactly 1 CREATE should succeed (others get ErrCouponExists)
    assert.Equal(t, int32(1), createSuccess, "Exactly 1 CREATE should succeed")

    // AC4: Claims only succeed after coupon exists
    // Some claims may have failed with NotFound (before create), which is correct

    // AC4: Verify no orphan claims
    var orphanClaims int
    err := testPool.QueryRow(ctx, `
        SELECT COUNT(*) FROM claims c
        LEFT JOIN coupons p ON c.coupon_name = p.name
        WHERE p.name IS NULL
    `).Scan(&orphanClaims)
    require.NoError(t, err)
    assert.Equal(t, 0, orphanClaims, "No orphan claims should exist")

    // Verify claims count matches successful claims
    var claimCount int
    err = testPool.QueryRow(ctx,
        "SELECT COUNT(*) FROM claims WHERE coupon_name = $1",
        couponName).Scan(&claimCount)
    require.NoError(t, err)
    assert.Equal(t, int(claimSuccess), claimCount,
        "Claim count should match successful claims")

    // Verify remaining stock consistency
    var remaining int
    err = testPool.QueryRow(ctx,
        "SELECT remaining_amount FROM coupons WHERE name = $1",
        couponName).Scan(&remaining)
    require.NoError(t, err)
    assert.Equal(t, availableStock-int(claimSuccess), remaining,
        "remaining_amount should reflect successful claims")
}
```

### Project Structure Notes

**Alignment with Existing Patterns:**
- Reuse `cleanupTables(t)` helper from `tests/chaos/setup_test.go`
- Reuse `testPool` global from `tests/chaos/setup_test.go`
- Follow assertion patterns: `require` for setup, `assert` for verification
- Use atomic operations for concurrent counter updates
- Use table-driven subtests for related scenarios

**File Organization:**
- One test function per acceptance criterion
- Helper functions at bottom of file
- Clear documentation header explaining chaos scenarios

### Error Handling Patterns

**From `internal/service/errors.go`:**
```go
var (
    ErrCouponExists    = errors.New("coupon already exists")
    ErrCouponNotFound  = errors.New("coupon not found")
    ErrInvalidRequest  = errors.New("invalid request")
    ErrAlreadyClaimed  = errors.New("coupon already claimed by user")
    ErrNoStock         = errors.New("coupon out of stock")
)
```

**Test assertions should use:**
```go
errors.Is(err, service.ErrNoStock)
errors.Is(err, service.ErrAlreadyClaimed)
errors.Is(err, service.ErrCouponNotFound)
errors.Is(err, service.ErrCouponExists)
```

### Previous Story Intelligence

**From Story 6.1 (Scale Stress Tests):**
- CI-only build tag pattern: `//go:build ci`
- Connection pool monitoring: `testPool.Stat()` for metrics
- Concurrent goroutine patterns with sync.WaitGroup + channels
- `fmt.Sprintf` for unique user IDs in loops

**From Story 6.2 (Database Resilience Testing):**
- `tests/chaos/` directory structure and setup_test.go
- Goroutine leak detection pattern with `runtime.NumGoroutine()`
- Context timeout testing patterns
- Error categorization patterns

**From Story 6.3 (Input Boundary Testing):**
- Boundary condition validation
- Service-layer error propagation testing

**From Story 6.4 (Transaction Edge Cases):**
- Partial failure rollback verification
- Concurrent transaction testing
- Database constraint validation
- Context cancellation patterns

**From Epic 4 (Stress Tests - `flash_sale_test.go`, `double_dip_test.go`):**
- Concurrent claim test infrastructure
- Database state verification queries
- `atomic.AddInt32` for thread-safe counters
- Result collection with buffered channels

### Database Schema Constraints Reference

**From `scripts/init.sql`:**
```sql
CREATE TABLE coupons (
    name VARCHAR(255) PRIMARY KEY,
    amount INTEGER NOT NULL CHECK (amount > 0),
    remaining_amount INTEGER NOT NULL CHECK (remaining_amount >= 0),  -- CRITICAL
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE claims (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, coupon_name)  -- Enforces one claim per user per coupon
);
```

**Key constraints for testing:**
- `CHECK (remaining_amount >= 0)` - Negative stock prevention at DB level
- `UNIQUE(user_id, coupon_name)` - Double-dip prevention
- `REFERENCES coupons(name)` - Prevents orphan claims

### Transaction Pattern Reference

**From `internal/service/coupon_service.go:ClaimCoupon`:**
```go
func (s *CouponService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
    tx, err := s.pool.Begin(ctx)           // Step 1: BEGIN
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer func() { _ = tx.Rollback(ctx) }() // Safety net

    coupon, err := s.couponRepo.GetCouponForUpdate(ctx, tx, couponName)  // Step 2: SELECT FOR UPDATE
    // ... check remaining_amount > 0 ...    // Step 3: CHECK

    err = s.claimRepo.Insert(ctx, tx, userID, couponName)  // Step 4: INSERT claim
    // ... handle unique constraint error ...

    err = s.couponRepo.DecrementStock(ctx, tx, couponName)  // Step 5: UPDATE

    return tx.Commit(ctx)                   // Step 6: COMMIT
}
```

### Anti-Patterns to Avoid

1. **DO NOT** run tests locally - use `//go:build ci` constraint
2. **DO NOT** create tests that leave database in inconsistent state
3. **DO NOT** use infinite timeouts - always use context with deadline
4. **DO NOT** ignore goroutine cleanup - verify no leaks
5. **DO NOT** hardcode test values - use constants for clarity
6. **DO NOT** skip database state verification after chaos operations
7. **DO NOT** expose raw database errors to client
8. **DO NOT** forget to seed random for reproducibility

### Randomization Best Practices

```go
// Seed with time for variety, but log seed for reproducibility
seed := time.Now().UnixNano()
rng := rand.New(rand.NewSource(seed))
t.Logf("Random seed: %d (use for reproducing failures)", seed)
```

### Model Import Note

The `model.CreateCouponRequest` struct is in `internal/model/`:
```go
import "github.com/fairyhunter13/scalable-coupon-system/internal/model"
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 6.5: Mixed Load & Chaos Testing]
- [Source: _bmad-output/planning-artifacts/architecture.md#Transaction Pattern]
- [Source: docs/project-context.md#Concurrency Pattern]
- [Source: internal/service/coupon_service.go:ClaimCoupon - Transaction implementation]
- [Source: internal/service/errors.go - Error types]
- [Source: tests/stress/flash_sale_test.go - Concurrent test patterns]
- [Source: tests/stress/double_dip_test.go - Same-user concurrent claim patterns]
- [Source: tests/stress/scale_test.go - CI-only scale test patterns]
- [Source: tests/chaos/setup_test.go - Chaos test infrastructure]
- [Source: tests/chaos/db_resilience_test.go - Connection pool and error handling patterns]
- [Source: 6-1-scale-stress-tests.md - CI-only build tag pattern]
- [Source: 6-2-database-resilience-testing.md - Chaos test setup]
- [Source: 6-3-input-boundary-testing.md - Boundary testing patterns]
- [Source: 6-4-transaction-edge-cases.md - Transaction testing patterns]
- [Source: scripts/init.sql - Database schema with constraints]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- All 4 chaos tests pass with race detection enabled
- Tests properly excluded without `ci` build tag
- Randomization seed logged for reproducibility

### Completion Notes List

- Created `tests/chaos/mixed_load_test.go` with comprehensive chaos test coverage
- Implemented `TestMixedOperationLoad`: 50 concurrent mixed CREATE/CLAIM/GET operations with weighted random selection (20% CREATE, 50% CLAIM, 30% GET). Verifies database consistency, no orphan claims, and no negative stock.
- Implemented `TestZeroStockStampede`: 100 concurrent claims on stock=1 coupon. Verifies exactly 1 success, 99 failures with ErrNoStock, no 500 errors, and remaining_amount = 0.
- Implemented `TestConstraintViolationStorm`: 50 concurrent same-user claims testing UNIQUE constraint. Verifies exactly 1 success, 49 ErrAlreadyClaimed failures, no raw DB errors leaked.
- Implemented `TestInterleavedCreateClaim`: Concurrent CREATE and CLAIM for same coupon name. Verifies correct serialization, claims fail with 404 before coupon exists, no orphan claims.
- All tests use `//go:build ci` tag for CI-only execution
- All tests use proper timeout (60s) and randomization seeding with logged seeds
- Tests integrate automatically with existing CI workflow (runs in `./tests/chaos/...` with `-tags ci`)

### File List

- `tests/chaos/mixed_load_test.go` (NEW) - Mixed load and chaos testing
- `.github/workflows/ci.yml` (MODIFIED) - Fix chaos job dependency to run independently of coverage

### Change Log

- 2026-01-11: Initial implementation of mixed load and chaos testing (Story 6.5)
- 2026-01-11: Code review fixes applied (see Senior Developer Review)

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5 (Adversarial Code Review)
**Date:** 2026-01-11

### Review Summary

| Category | Status |
|----------|--------|
| AC Validation | PASS (5/5) |
| Task Completion | PASS (all marked [x] verified) |
| Code Quality | PASS |
| Test Quality | PASS |
| CI Integration | FIXED |

### Issues Found and Fixed

#### Issue #1: CRITICAL - Test File Not Committed
**Problem:** `tests/chaos/mixed_load_test.go` was created but never staged/committed to git (showed as `??` untracked). Story 6.5 tests were NOT running in CI because the file didn't exist in the repository.
**Fix:** File staged and committed with this review.

#### Issue #2: HIGH - CI Chaos Job Dependency
**Problem:** The `chaos` job in `.github/workflows/ci.yml` had `needs: [test]` which caused chaos tests to be SKIPPED when the test job failed (e.g., coverage threshold). This meant chaos tests could silently not run.
**Fix:** Changed to `needs: [build]` - chaos tests now run independently since they use dockertest (self-contained PostgreSQL containers).

### Verification Results

All 4 tests pass locally with race detection:
```
=== RUN   TestMixedOperationLoad
--- PASS: TestMixedOperationLoad (0.08s)
=== RUN   TestZeroStockStampede
--- PASS: TestZeroStockStampede (0.04s)
=== RUN   TestConstraintViolationStorm
--- PASS: TestConstraintViolationStorm (0.05s)
=== RUN   TestInterleavedCreateClaim
--- PASS: TestInterleavedCreateClaim (0.02s)
PASS
```

### AC Verification Matrix

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC1 | Mixed Operation Load Test | PASS | 50 concurrent ops, DB consistency verified |
| AC2 | Zero-Stock Stampede Test | PASS | 100 concurrent, exactly 1 success |
| AC3 | Constraint Violation Storm | PASS | 50 same-user claims, UNIQUE enforced |
| AC4 | Interleaved Create-Claim | PASS | No orphan claims, correct serialization |
| AC5 | CI-Only Build Tags | PASS | `//go:build ci` verified, excluded from default |

### Outcome

**APPROVED** - All issues fixed, ready for merge.

