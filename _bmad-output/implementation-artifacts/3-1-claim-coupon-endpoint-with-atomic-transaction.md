# Story 3.1: Claim Coupon Endpoint with Atomic Transaction

Status: done

## Story

As an **API consumer**,
I want **to claim a coupon for a user atomically**,
So that **claims are guaranteed correct even under high concurrency**.

## Acceptance Criteria

### AC1: Successful Claim
**Given** a coupon "PROMO_SUPER" exists with remaining_amount=5
**And** user "user_001" has not claimed this coupon
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 200 OK (or 201 Created)
**And** a claim record is inserted with user_id="user_001" and coupon_name="PROMO_SUPER"
**And** the coupon's remaining_amount is decremented to 4

### AC2: Duplicate Claim Prevention
**Given** user "user_001" has already claimed coupon "PROMO_SUPER"
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 409 Conflict
**And** the response body contains `{"error": "coupon already claimed by user"}`
**And** no database changes occur

### AC3: Out of Stock Prevention
**Given** a coupon "PROMO_SUPER" exists with remaining_amount=0
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_999", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "coupon out of stock"}`
**And** no database changes occur

### AC4: Coupon Not Found
**Given** no coupon named "NONEXISTENT" exists
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "NONEXISTENT"}`
**Then** I receive 404 Not Found
**And** the response body contains `{"error": "coupon not found"}`

### AC5: Missing user_id Validation
**Given** a request with missing `user_id` field
**When** I send POST to `/api/coupons/claim`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: user_id is required"}`

### AC6: Missing coupon_name Validation
**Given** a request with missing `coupon_name` field
**When** I send POST to `/api/coupons/claim`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: coupon_name is required"}`

## Tasks / Subtasks

- [x] Task 1: Add Domain Error for Claim Operations (AC: #2, #3)
  - [x] Subtask 1.1: Add `ErrAlreadyClaimed` to `internal/service/errors.go`
  - [x] Subtask 1.2: Add `ErrNoStock` to `internal/service/errors.go`

- [x] Task 2: Add ClaimCouponRequest Model (AC: #5, #6)
  - [x] Subtask 2.1: Create `ClaimCouponRequest` struct in `internal/model/coupon.go`
  - [x] Subtask 2.2: Add validation tags: `user_id validate:"required"`, `coupon_name validate:"required"`

- [x] Task 3: Extend Repository Interfaces for Transaction Support (AC: #1, #2, #3)
  - [x] Subtask 3.1: Add `TxQuerier` interface that both `pgxpool.Pool` and `pgx.Tx` satisfy
  - [x] Subtask 3.2: Add `GetCouponForUpdate(ctx, tx, name)` method to CouponRepository
  - [x] Subtask 3.3: Add `DecrementStock(ctx, tx, name)` method to CouponRepository
  - [x] Subtask 3.4: Add `Insert(ctx, tx, userID, couponName)` method to ClaimRepository

- [x] Task 4: Implement ClaimCoupon Service Method (AC: #1, #2, #3, #4)
  - [x] Subtask 4.1: Add `*pgxpool.Pool` field to `CouponService` (for transactions)
  - [x] Subtask 4.2: Implement `ClaimCoupon(ctx, userID, couponName) error` with transaction
  - [x] Subtask 4.3: Transaction flow: BEGIN -> SELECT FOR UPDATE -> Check stock -> INSERT claim -> UPDATE stock -> COMMIT
  - [x] Subtask 4.4: Handle unique constraint violation (23505) -> ErrAlreadyClaimed
  - [x] Subtask 4.5: Handle remaining_amount <= 0 -> ErrNoStock
  - [x] Subtask 4.6: Handle coupon not found -> ErrCouponNotFound

- [x] Task 5: Implement ClaimCoupon Handler (AC: #1-#6)
  - [x] Subtask 5.1: Create `claim_handler.go` in `internal/handler/`
  - [x] Subtask 5.2: Parse and validate `ClaimCouponRequest`
  - [x] Subtask 5.3: Map service errors to HTTP responses:
    - `ErrCouponNotFound` -> 404
    - `ErrAlreadyClaimed` -> 409
    - `ErrNoStock` -> 400
    - Other errors -> 500
  - [x] Subtask 5.4: Return 200 OK on success (empty body)

- [x] Task 6: Register Claim Route (AC: #1)
  - [x] Subtask 6.1: Add `POST /api/coupons/claim` route in `cmd/api/main.go`
  - [x] Subtask 6.2: Wire claim handler with service and validator

- [x] Task 7: Unit Tests for Claim Service (AC: #1-#6)
  - [x] Subtask 7.1: Test successful claim decrements stock
  - [x] Subtask 7.2: Test duplicate claim returns ErrAlreadyClaimed
  - [x] Subtask 7.3: Test zero stock returns ErrNoStock
  - [x] Subtask 7.4: Test missing coupon returns ErrCouponNotFound
  - [x] Subtask 7.5: Test transaction rollback on failure

- [x] Task 8: Unit Tests for Claim Handler (AC: #1-#6)
  - [x] Subtask 8.1: Test 200 OK on successful claim
  - [x] Subtask 8.2: Test 409 Conflict for duplicate claim
  - [x] Subtask 8.3: Test 400 Bad Request for out of stock
  - [x] Subtask 8.4: Test 404 Not Found for missing coupon
  - [x] Subtask 8.5: Test 400 Bad Request for missing user_id
  - [x] Subtask 8.6: Test 400 Bad Request for missing coupon_name

## Dev Notes

### CRITICAL: Transaction Pattern (from Architecture)

The claim operation MUST use this exact pattern within a single transaction:

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

    // 2. Check stock
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

### CRITICAL: Claim Insert SQL

```sql
INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)
```

The UNIQUE constraint on `(user_id, coupon_name)` will raise error code `23505` on duplicate.

### CRITICAL: Stock Decrement SQL

```sql
UPDATE coupons SET remaining_amount = remaining_amount - 1 WHERE name = $1
```

### CRITICAL: Error Code Handling

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return ErrAlreadyClaimed
}
```

### CRITICAL: HTTP Response Codes

| Scenario | Status Code | Response Body |
|----------|-------------|---------------|
| Successful claim | 200 OK | (empty body) |
| Duplicate claim | 409 Conflict | `{"error": "coupon already claimed by user"}` |
| Out of stock | 400 Bad Request | `{"error": "coupon out of stock"}` |
| Coupon not found | 404 Not Found | `{"error": "coupon not found"}` |
| Missing user_id | 400 Bad Request | `{"error": "invalid request: user_id is required"}` |
| Missing coupon_name | 400 Bad Request | `{"error": "invalid request: coupon_name is required"}` |
| Internal error | 500 Internal Server Error | `{"error": "internal server error"}` |

### CRITICAL: JSON Field Names (snake_case)

Request body MUST use snake_case:
```json
{
  "user_id": "user_001",
  "coupon_name": "PROMO_SUPER"
}
```

### CRITICAL: Existing Code Patterns to Follow

From `internal/handler/coupon_handler.go`:
- Use `formatValidationError()` for validation error messages
- Use `fiber.Map{"error": "..."}` for error responses
- Log errors with zerolog before returning 500

From `internal/repository/coupon_repository.go`:
- Use `errors.As(err, &pgErr)` for PostgreSQL error handling
- Wrap errors with `fmt.Errorf("operation: %w", err)`

From `internal/service/coupon_service.go`:
- Define interfaces for repositories
- Return domain errors (not HTTP status codes)

### Project Structure Notes

**Files to CREATE:**
- `internal/handler/claim_handler.go` - HTTP handler for POST /api/coupons/claim
- `internal/handler/claim_handler_test.go` - Unit tests for claim handler

**Files to MODIFY:**
- `internal/service/errors.go` - Add ErrAlreadyClaimed, ErrNoStock
- `internal/model/coupon.go` - Add ClaimCouponRequest struct
- `internal/service/coupon_service.go` - Add ClaimCoupon method, pool field
- `internal/service/coupon_service_test.go` - Add claim tests
- `internal/repository/coupon_repository.go` - Add GetCouponForUpdate, DecrementStock
- `internal/repository/coupon_repository_test.go` - Add repository tests
- `internal/repository/claim_repository.go` - Add Insert method with tx support
- `internal/repository/claim_repository_test.go` - Add repository tests
- `internal/handler/handler.go` - Register claim route

**Files NOT to MODIFY:**
- Database schema (already has UNIQUE constraint)
- docker-compose.yml
- Dockerfile

### CRITICAL: TxQuerier Interface

Create a common interface that both `*pgxpool.Pool` and `pgx.Tx` satisfy:

```go
// TxQuerier is implemented by both pgxpool.Pool and pgx.Tx
type TxQuerier interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}
```

Repository methods that need transaction support should accept `TxQuerier` instead of `*pgxpool.Pool`.

### CRITICAL: Service Constructor Update

The `CouponService` needs access to the pool for transaction management:

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

### Previous Epic Learnings

From Epic 2 (Stories 2.1-2.3):
1. **Error messages must be EXACT** - Use the precise wording from acceptance criteria
2. **JSON uses snake_case** - All field names lowercase with underscores
3. **Empty arrays, not null** - `claimed_by` returns `[]` not `null`
4. **Validation with go-playground/validator** - Use struct tags for validation
5. **Defense-in-depth** - Check for nil even if handler validates
6. **Return empty body for success** - 200/201 with `c.Send(nil)` or `c.Status(...).Send(nil)`

### Testing Requirements

**Unit Tests (co-located):**
- Mock repository interfaces for handler tests
- Test all error paths and success paths
- Use testify assertions

**Integration Tests (tests/integration/):**
- Use dockertest for real PostgreSQL
- Verify actual database state changes
- Test transaction isolation

### Web Research Intelligence (pgx v5 + Fiber v2 - 2025)

**pgx v5 Transaction Best Practices:**
- Use `pool.Begin(ctx)` for standard transactions
- `defer tx.Rollback(ctx)` is safe - no-op if already committed
- PostgreSQL error codes: `23505` (unique violation), `40001` (serialization failure)
- `SELECT FOR UPDATE` locks rows until COMMIT/ROLLBACK

**Fiber v2 Compatibility:**
- Fiber v2.52.x is production-stable (do NOT upgrade to v3 yet)
- Use `c.Context()` to get the request context for database operations
- Use `fiber.Map{}` for JSON responses

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Transaction Pattern] - Exact transaction flow
- [Source: _bmad-output/planning-artifacts/architecture.md#Error Handling Flow] - Error propagation
- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.1] - Acceptance criteria
- [Source: docs/project-context.md#Concurrency Pattern] - SELECT FOR UPDATE pattern
- [Source: docs/project-context.md#API Response Patterns] - HTTP status codes
- [Source: internal/handler/coupon_handler.go] - Existing handler patterns
- [Source: internal/repository/coupon_repository.go] - PostgreSQL error handling
- [Source: https://pkg.go.dev/github.com/jackc/pgx/v5] - pgx v5 documentation
- [Source: https://gofiber.io/] - Fiber framework documentation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation proceeded without issues

### Completion Notes List

- Implemented POST /api/coupons/claim endpoint with atomic transaction support
- Added TxQuerier interface to pkg/database/postgres.go to break import cycle
- Service uses SELECT FOR UPDATE pattern to lock coupon row during claim
- Handles duplicate claim (23505 unique violation) -> 409 Conflict
- Handles out of stock (remaining_amount <= 0) -> 400 Bad Request
- Handles coupon not found -> 404 Not Found
- All 15 service unit tests pass including 5 new ClaimCoupon tests
- All 27 handler unit tests pass including 10 new claim handler tests
- All integration tests pass with updated service constructor
- golangci-lint passes on main source code (0 issues)

### File List

**Files Created:**
- internal/handler/claim_handler.go
- internal/handler/claim_handler_test.go

**Files Modified:**
- internal/service/errors.go - Added ErrAlreadyClaimed, ErrNoStock
- internal/model/coupon.go - Added ClaimCouponRequest struct
- internal/service/coupon_service.go - Added ClaimCoupon method, TxBeginner interface, pool field
- internal/service/coupon_service_test.go - Added ClaimCoupon tests, updated mock repos
- internal/repository/coupon_repository.go - Added GetCouponForUpdate, DecrementStock methods
- internal/repository/claim_repository.go - Added Insert method with tx support
- pkg/database/postgres.go - Added TxQuerier interface
- cmd/api/main.go - Registered claim route, updated service constructor
- tests/integration/coupon_integration_test.go - Updated service constructor, added claim route

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5 (claude-opus-4-5-20251101)
**Review Date:** 2026-01-11
**Outcome:** APPROVED (after fixes applied)

### Issues Found and Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| HIGH | Missing repository unit tests for GetCouponForUpdate, DecrementStock, ClaimRepository.Insert | Added 12 new tests to coupon_repository_test.go and claim_repository_test.go |
| HIGH | Missing claim endpoint integration tests | Added 8 new integration tests covering all ACs |
| MEDIUM | Lint errors (errcheck) in integration tests | Fixed all errcheck violations |
| LOW | Missing request_id in claim handler logs | Added request_id, method, path fields to logging |

### Test Coverage After Review

- **Repository tests:** 25 tests (8 new for transaction methods)
- **Handler tests:** 27 tests (unchanged)
- **Service tests:** 15 tests (unchanged)
- **Integration tests:** 23 tests (8 new for claim endpoint)
- **All tests pass with race detection**
- **golangci-lint: 0 issues**

### Files Modified During Review

- internal/repository/coupon_repository_test.go - Added GetCouponForUpdate and DecrementStock tests
- internal/repository/claim_repository_test.go - Added Insert tests with unique violation handling
- tests/integration/coupon_integration_test.go - Added 8 claim integration tests, fixed errcheck
- tests/integration/setup_test.go - Fixed errcheck violation
- internal/handler/claim_handler.go - Added request_id, method, path to logging

## Change Log

| Date | Change |
|------|--------|
| 2026-01-11 | Implemented claim coupon endpoint with atomic transaction support (Story 3.1) |
| 2026-01-11 | Code review: Added missing repository and integration tests, fixed lint errors, enhanced logging |
