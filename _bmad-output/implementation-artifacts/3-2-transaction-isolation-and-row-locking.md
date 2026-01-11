# Story 3.2: Transaction Isolation and Row Locking

Status: done

## Story

As an **API consumer**,
I want **claim operations to be atomic and isolated**,
So that **concurrent claims never result in overselling or data corruption**.

## Acceptance Criteria

### AC1: Correct Transaction Flow
**Given** the claim service implementation
**When** I review the transaction flow
**Then** it follows this exact sequence within a single transaction:
1. BEGIN transaction
2. SELECT ... FROM coupons WHERE name = $1 FOR UPDATE (locks the row)
3. Check remaining_amount > 0 (return error if not)
4. INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)
5. UPDATE coupons SET remaining_amount = remaining_amount - 1
6. COMMIT transaction

### AC2: Transaction Rollback on Failure
**Given** a claim operation fails at any step
**When** an error occurs
**Then** the entire transaction is rolled back
**And** no partial changes are persisted
**And** the error is logged with request context

### AC3: Unique Constraint Violation Handling
**Given** the claims table unique constraint on (user_id, coupon_name)
**When** a duplicate claim is attempted concurrently
**Then** the database constraint violation is caught
**And** the transaction is rolled back
**And** 409 Conflict is returned

### AC4: Race Condition Prevention for Last Stock
**Given** two concurrent claim requests for the last available coupon
**When** both requests attempt to claim simultaneously
**Then** exactly one succeeds with 200/201
**And** exactly one fails with 400 (out of stock)
**And** remaining_amount is exactly 0 (not negative)

### AC5: SELECT FOR UPDATE Serialization
**Given** the SELECT FOR UPDATE implementation
**When** multiple transactions attempt to lock the same coupon row
**Then** they are serialized (one waits for the other)
**And** no deadlocks occur under normal operation

## Tasks / Subtasks

- [x] Task 1: Create TxQuerier Interface (AC: #1, #2)
  - [x] Subtask 1.1: Add `TxQuerier` interface in `pkg/database/postgres.go`
  - [x] Subtask 1.2: Interface must be satisfied by both `*pgxpool.Pool` and `pgx.Tx`
  - [x] Subtask 1.3: Include methods: `Exec`, `QueryRow`, `Query`

- [x] Task 2: Add Transaction Repository Methods (AC: #1, #4, #5)
  - [x] Subtask 2.1: Add `GetCouponForUpdate(ctx, tx TxQuerier, name string)` to CouponRepository
  - [x] Subtask 2.2: SQL MUST be: `SELECT name, amount, remaining_amount, created_at FROM coupons WHERE name = $1 FOR UPDATE`
  - [x] Subtask 2.3: Add `DecrementStock(ctx, tx TxQuerier, name string)` to CouponRepository
  - [x] Subtask 2.4: SQL MUST be: `UPDATE coupons SET remaining_amount = remaining_amount - 1 WHERE name = $1`

- [x] Task 3: Add Claim Insert with Transaction Support (AC: #1, #3)
  - [x] Subtask 3.1: Modify ClaimRepository to accept `TxQuerier` for Insert method
  - [x] Subtask 3.2: Add `Insert(ctx, tx TxQuerier, userID, couponName string) error` method
  - [x] Subtask 3.3: Handle PostgreSQL error code `23505` -> return `ErrAlreadyClaimed`

- [x] Task 4: Add Domain Errors for Claim Operations (AC: #3)
  - [x] Subtask 4.1: Add `ErrAlreadyClaimed` to `internal/service/errors.go`
  - [x] Subtask 4.2: Add `ErrNoStock` to `internal/service/errors.go`

- [x] Task 5: Update CouponService for Transaction Management (AC: #1, #2, #3, #4, #5)
  - [x] Subtask 5.1: Add `TxBeginner` interface (pool abstraction) to CouponService struct
  - [x] Subtask 5.2: Update constructor: `NewCouponService(pool *pgxpool.Pool, couponRepo, claimRepo)`
  - [x] Subtask 5.3: Implement `ClaimCoupon(ctx, userID, couponName) error` using exact transaction pattern
  - [x] Subtask 5.4: Use `defer tx.Rollback(ctx)` pattern - safe no-op if committed
  - [x] Subtask 5.5: Verify error propagation: ErrCouponNotFound, ErrNoStock, ErrAlreadyClaimed

- [x] Task 6: Update Repository Interfaces in Service (AC: #1)
  - [x] Subtask 6.1: Extend `CouponRepositoryInterface` with `GetCouponForUpdate` and `DecrementStock`
  - [x] Subtask 6.2: Extend `ClaimRepositoryInterface` with `Insert` method

- [x] Task 7: Unit Tests for Transaction Behavior (AC: #1, #2, #3, #4, #5)
  - [x] Subtask 7.1: Test successful claim transaction flow
  - [x] Subtask 7.2: Test rollback on claim failure
  - [x] Subtask 7.3: Test unique constraint violation handling
  - [x] Subtask 7.4: Test stock check before claim
  - [x] Subtask 7.5: Run tests with `-race` flag

- [x] Task 8: Integration Tests for Concurrency (AC: #4, #5)
  - [x] Subtask 8.1: Test 2 concurrent claims for 1 remaining stock -> exactly 1 success
  - [x] Subtask 8.2: Verify remaining_amount = 0 after test
  - [x] Subtask 8.3: Verify exactly 1 claim record created
  - [x] Subtask 8.4: Use sync.WaitGroup for goroutine coordination

## Dev Notes

### CRITICAL: Story 3-1 is a Prerequisite

This story builds on Story 3-1 (Claim Coupon Endpoint with Atomic Transaction). Story 3-1 implements the basic claim endpoint and handler. This story focuses specifically on the **transaction isolation and row locking mechanism** that makes it concurrency-safe.

If Story 3-1 is NOT yet implemented, implement the transaction logic here but expect to wire up the handler from Story 3-1.

### CRITICAL: Transaction Pattern (MANDATORY)

The `ClaimCoupon` method MUST follow this exact implementation pattern:

```go
func (s *CouponService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) // Safe: no-op if committed

    // 1. Lock the coupon row (SELECT FOR UPDATE)
    coupon, err := s.couponRepo.GetCouponForUpdate(ctx, tx, couponName)
    if err != nil {
        if errors.Is(err, ErrCouponNotFound) {
            return ErrCouponNotFound
        }
        return fmt.Errorf("get coupon for update: %w", err)
    }

    // 2. Check stock AFTER locking
    if coupon.RemainingAmount <= 0 {
        return ErrNoStock
    }

    // 3. Insert claim (UNIQUE constraint catches duplicates)
    err = s.claimRepo.Insert(ctx, tx, userID, couponName)
    if err != nil {
        if errors.Is(err, ErrAlreadyClaimed) {
            return ErrAlreadyClaimed
        }
        return fmt.Errorf("insert claim: %w", err)
    }

    // 4. Decrement stock
    err = s.couponRepo.DecrementStock(ctx, tx, couponName)
    if err != nil {
        return fmt.Errorf("decrement stock: %w", err)
    }

    return tx.Commit(ctx)
}
```

### CRITICAL: SELECT FOR UPDATE SQL

The row lock query MUST be:
```sql
SELECT name, amount, remaining_amount, created_at
FROM coupons
WHERE name = $1
FOR UPDATE
```

This locks the row until the transaction completes, preventing concurrent modifications.

### CRITICAL: TxQuerier Interface

Create a common interface that both `*pgxpool.Pool` and `pgx.Tx` satisfy:

```go
// internal/repository/interfaces.go (NEW FILE)
package repository

import (
    "context"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

// TxQuerier is implemented by both pgxpool.Pool and pgx.Tx
// This allows repository methods to work with either.
type TxQuerier interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}
```

### CRITICAL: PostgreSQL Error Code Handling

Handle unique constraint violation:
```go
import "github.com/jackc/pgx/v5/pgconn"

var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return ErrAlreadyClaimed
}
```

### CRITICAL: Why SELECT FOR UPDATE Matters

Without `FOR UPDATE`:
1. Transaction A: SELECT remaining_amount (gets 1)
2. Transaction B: SELECT remaining_amount (gets 1)
3. Transaction A: UPDATE remaining_amount - 1 (now 0)
4. Transaction B: UPDATE remaining_amount - 1 (now -1!) **OVERSOLD!**

With `FOR UPDATE`:
1. Transaction A: SELECT ... FOR UPDATE (locks row, gets 1)
2. Transaction B: SELECT ... FOR UPDATE (BLOCKED, waits)
3. Transaction A: UPDATE, COMMIT (remaining = 0, releases lock)
4. Transaction B: (now unblocked) SELECT returns 0 -> ErrNoStock

### Project Structure Notes

**Files to CREATE:**
- `internal/repository/interfaces.go` - TxQuerier interface

**Files to MODIFY:**
- `internal/service/errors.go` - Add ErrAlreadyClaimed, ErrNoStock
- `internal/service/coupon_service.go` - Add pool field, ClaimCoupon method, update constructor
- `internal/repository/coupon_repository.go` - Add GetCouponForUpdate, DecrementStock methods
- `internal/repository/claim_repository.go` - Add Insert method with TxQuerier

**Files NOT to MODIFY:**
- Database schema (already has UNIQUE constraint and correct structure)
- `internal/handler/` - Handler logic is Story 3-1 scope
- docker-compose.yml, Dockerfile

### Updated CouponService Constructor

```go
type CouponService struct {
    pool       *pgxpool.Pool // For transaction management
    couponRepo CouponRepositoryInterface
    claimRepo  ClaimRepositoryInterface
}

func NewCouponService(pool *pgxpool.Pool, couponRepo CouponRepositoryInterface, claimRepo ClaimRepositoryInterface) *CouponService {
    return &CouponService{
        pool:       pool,
        couponRepo: couponRepo,
        claimRepo:  claimRepo,
    }
}
```

### Updated Repository Interfaces

```go
// In internal/service/coupon_service.go
type CouponRepositoryInterface interface {
    Insert(ctx context.Context, coupon *model.Coupon) error
    GetByName(ctx context.Context, name string) (*model.Coupon, error)
    GetCouponForUpdate(ctx context.Context, tx TxQuerier, name string) (*model.Coupon, error) // NEW
    DecrementStock(ctx context.Context, tx TxQuerier, name string) error // NEW
}

type ClaimRepositoryInterface interface {
    GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error)
    Insert(ctx context.Context, tx TxQuerier, userID, couponName string) error // NEW
}
```

**Note:** You'll need to import the TxQuerier interface or define it locally.

### Existing Code Patterns to Follow

From `internal/repository/coupon_repository.go`:
- Use `errors.As(err, &pgErr)` for PostgreSQL error handling
- Wrap errors with `fmt.Errorf("operation: %w", err)`
- Return `nil, nil` for "not found" cases (service handles as error)

From `internal/service/coupon_service.go`:
- Define repository interfaces in service package
- Return domain errors (ErrCouponNotFound, ErrNoStock, etc.)
- Defense-in-depth: check for nil even if caller validates

From `internal/service/errors.go`:
- Define domain errors as package-level var using `errors.New()`

### Testing Strategy

**Unit Tests (co-located):**
- Mock TxQuerier interface for repository tests
- Mock repository interfaces for service tests
- Test each error path: not found, no stock, already claimed
- Test successful transaction commit
- Test rollback behavior (mock tx that fails on commit)

**Integration Tests (tests/integration/):**
- Use dockertest for real PostgreSQL
- Test actual row locking behavior with concurrent goroutines
- Verify database state after concurrent operations

**Concurrency Test Example:**
```go
func TestConcurrentClaimLastStock(t *testing.T) {
    // Setup: Create coupon with remaining_amount = 1

    var wg sync.WaitGroup
    results := make(chan error, 2)

    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(userID string) {
            defer wg.Done()
            err := service.ClaimCoupon(ctx, userID, "TEST_COUPON")
            results <- err
        }(fmt.Sprintf("user_%d", i))
    }

    wg.Wait()
    close(results)

    // Verify: exactly 1 success, exactly 1 ErrNoStock
    var successes, noStocks int
    for err := range results {
        if err == nil {
            successes++
        } else if errors.Is(err, service.ErrNoStock) {
            noStocks++
        }
    }

    assert.Equal(t, 1, successes)
    assert.Equal(t, 1, noStocks)

    // Verify database state
    coupon, _ := repo.GetByName(ctx, "TEST_COUPON")
    assert.Equal(t, 0, coupon.RemainingAmount)
}
```

### Web Research Intelligence (pgx v5 - 2025)

**pgx v5 Transaction Best Practices:**
- Use `pool.Begin(ctx)` for standard transactions (Read Committed isolation)
- `defer tx.Rollback(ctx)` is idempotent - safe to call after commit
- For higher isolation, use `pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})`
- PostgreSQL error codes: `23505` (unique violation), `40001` (serialization failure), `40P01` (deadlock detected)

**SELECT FOR UPDATE Behavior:**
- Locks the specific row until transaction ends
- Other transactions attempting to SELECT FOR UPDATE the same row will BLOCK
- Read Committed + SELECT FOR UPDATE is the industry standard pattern for inventory systems
- Avoid `NOWAIT` option unless you want immediate failure on lock contention

**Connection Pool Sizing:**
- Default pgxpool max connections = 4
- For stress tests with 50 concurrent claims, ensure pool size >= 50
- Configure via `POOL_MAX_CONNS` environment variable

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Transaction Pattern] - Exact transaction flow
- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture] - SELECT FOR UPDATE specification
- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.2] - Acceptance criteria
- [Source: _bmad-output/implementation-artifacts/3-1-claim-coupon-endpoint-with-atomic-transaction.md] - Previous story context
- [Source: docs/project-context.md#Concurrency Pattern] - Mandatory transaction pattern
- [Source: internal/repository/coupon_repository.go] - Existing repository patterns
- [Source: internal/service/coupon_service.go] - Existing service patterns
- [Source: https://pkg.go.dev/github.com/jackc/pgx/v5] - pgx v5 documentation
- [Source: https://www.postgresql.org/docs/current/explicit-locking.html] - PostgreSQL row locking

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation found to be already complete from Story 3-1. Only Task 8 (Integration Tests for Concurrency) required new work.

### Completion Notes List

1. **Tasks 1-7: Already Implemented** - Most of this story's implementation was already completed as part of Story 3-1. Upon analysis:
   - TxQuerier interface exists in `pkg/database/postgres.go:16-20`
   - GetCouponForUpdate and DecrementStock implemented in `internal/repository/coupon_repository.go:80-109`
   - ClaimRepository.Insert with TxQuerier implemented in `internal/repository/claim_repository.go:72-84`
   - ErrAlreadyClaimed and ErrNoStock defined in `internal/service/errors.go:16-19`
   - CouponService.ClaimCoupon with full transaction pattern in `internal/service/coupon_service.go:107-144`
   - Unit tests comprehensive in `*_test.go` files

2. **Task 8: Implemented Concurrency Integration Tests** - Created `tests/integration/concurrency_test.go` with:
   - `TestConcurrentClaimLastStock`: Tests AC4 - 2 concurrent claims for 1 stock, exactly 1 succeeds
   - `TestConcurrentClaimsSameUser`: Tests AC3 - Unique constraint enforcement under concurrency
   - `TestSELECTFORUPDATESerialization`: Tests AC5 - Row locking serializes concurrent transactions
   - `TestFlashSaleScenario`: Realistic test with 20 concurrent requests for 5 stock
   - `TestTransactionRollbackOnFailure_OutOfStock`: Tests AC2 - Transaction rollback on failure

3. **All Tests Pass** - Full test suite with `-race` flag passes, including all new concurrency tests

### Change Log

- 2026-01-11: Implemented Task 8 - Added concurrency integration tests in `tests/integration/concurrency_test.go`
- 2026-01-11: Verified Tasks 1-7 already implemented from Story 3-1
- 2026-01-11: All acceptance criteria validated, story marked for review
- 2026-01-11: **Code Review Completed** - Fixed 4 MEDIUM issues, story marked done

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5
**Date:** 2026-01-11
**Outcome:** ✅ APPROVED

### Review Summary

All 5 Acceptance Criteria verified as fully implemented:
- AC1: Transaction flow matches exact specification
- AC2: Rollback on failure confirmed with tests
- AC3: Unique constraint handling (23505) working
- AC4: Race condition prevention tested with concurrent claims
- AC5: SELECT FOR UPDATE serialization verified

### Issues Found & Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| MEDIUM | Missing context timeout in concurrency tests | Added `context.WithTimeout(30s)` to all 5 tests |
| MEDIUM | Unused `validator` import and dead code | Removed unused import and `_ = validator.New()` |
| MEDIUM | Inconsistent test naming | Renamed `TestSELECTFORUPDATESerialization` → `TestSelectForUpdateSerialization` |
| MEDIUM | Variable shadowing (`errors` package) | Changed `var errors int` → `var errs int` |

### Verification

- All tests pass with `-race` flag
- `golangci-lint run ./...`: 0 issues
- `gosec ./...`: 0 issues
- Coverage: 80-96% across packages

### File List

**Files Created:**
- tests/integration/concurrency_test.go

**Files Already Existing (from Story 3-1):**
- pkg/database/postgres.go (contains TxQuerier interface)
- internal/service/errors.go (contains ErrAlreadyClaimed, ErrNoStock)
- internal/service/coupon_service.go (contains ClaimCoupon with transaction pattern)
- internal/repository/coupon_repository.go (contains GetCouponForUpdate, DecrementStock)
- internal/repository/claim_repository.go (contains Insert with TxQuerier)
- internal/service/coupon_service_test.go (contains unit tests)
- internal/repository/coupon_repository_test.go (contains unit tests)
- internal/repository/claim_repository_test.go (contains unit tests)
