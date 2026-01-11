# Story 4.4: Double Dip Stress Test

Status: done

## Story

As a **developer**,
I want **a stress test simulating duplicate claim attempts**,
So that **I can prove the unique constraint prevents double claims**.

## Acceptance Criteria

1. **Given** a coupon "DOUBLE_TEST" with amount=100
   **And** a single user "user_greedy"
   **When** 10 concurrent goroutines attempt to claim for "user_greedy" simultaneously
   **Then** exactly 1 claim succeeds (200/201 response)
   **And** exactly 9 claims fail (409 Conflict)
   **And** remaining_amount is exactly 99
   **And** claimed_by contains exactly ["user_greedy"]

2. **Given** the double dip stress test
   **When** I run `go test ./tests/stress/... -run TestDoubleDip -v`
   **Then** the test passes consistently (100% of runs)
   **And** the unique constraint violation is properly handled

3. **Given** the stress test
   **When** I run it 10 times consecutively
   **Then** it passes all 10 runs without flakiness
   **And** exactly 1 success is recorded each time

4. **Given** the double dip scenario
   **When** I verify the database state
   **Then** only one claim record exists for (user_greedy, DOUBLE_TEST)
   **And** no duplicate records were ever inserted (even temporarily)

## Tasks / Subtasks

- [x] Task 1: Create tests/stress directory and setup if not exists (AC: #2)
  - [x] Check if `tests/stress/` directory exists (may already exist from 4-3)
  - [x] If not exists, create `tests/stress/setup_test.go` with TestMain using dockertest
  - [x] Replicate PostgreSQL 15-alpine setup from integration tests
  - [x] Add schema migrations matching `scripts/init.sql`
  - [x] Add `cleanupTables()` helper function

- [x] Task 2: Implement Double Dip stress test (AC: #1, #2)
  - [x] Create `tests/stress/double_dip_test.go`
  - [x] Create coupon "DOUBLE_TEST" with amount=100, remaining_amount=100
  - [x] Launch 10 concurrent goroutines using sync.WaitGroup
  - [x] **ALL goroutines claim with the SAME user_id "user_greedy"**
  - [x] Collect all errors/successes in buffered channel
  - [x] Count successes (nil error) and failures (service.ErrAlreadyClaimed)

- [x] Task 3: Add result verification (AC: #1, #4)
  - [x] Assert exactly 1 success
  - [x] Assert exactly 9 ErrAlreadyClaimed failures
  - [x] Assert 0 other errors (no ErrNoStock expected)
  - [x] Query database to verify remaining_amount = 99
  - [x] Query database to verify exactly 1 claim exists
  - [x] Verify claim record has user_id = "user_greedy"

- [x] Task 4: Add reliability checks (AC: #3)
  - [x] Set context timeout to 30 seconds
  - [x] Log execution time for monitoring
  - [x] Add test documentation explaining the double-dip prevention mechanism

- [x] Task 5: Verify race detection and flakiness (AC: #3)
  - [x] Run `go test -race ./tests/stress/... -run TestDoubleDip -v`
  - [x] Confirm zero race conditions detected
  - [x] Run test 10 times: `for i in {1..10}; do go test ./tests/stress/... -run TestDoubleDip -v; done`
  - [x] Confirm 100% pass rate

## Dev Notes

### Critical Implementation Details

**MANDATORY - From Architecture & Project Context:**

The Double Dip stress test validates:
- **NFR2**: System handles 10 concurrent same-user requests with exactly 1 success
- **Database UNIQUE constraint** on `(user_id, coupon_name)` enforces this
- **service.ErrAlreadyClaimed** is returned when unique constraint is violated

### Key Difference from Flash Sale Test (4-3)

| Aspect | Flash Sale (4-3) | Double Dip (4-4) |
|--------|------------------|------------------|
| Concurrent requests | 50 | 10 |
| User IDs | 50 different users | 1 SAME user |
| Expected success | 5 (limited by stock) | 1 (limited by unique constraint) |
| Expected failure | ErrNoStock | ErrAlreadyClaimed |
| Stock after test | 0 | 99 |

### Existing Pattern Reference

The integration test `TestConcurrentClaimsSameUser` in `tests/integration/concurrency_test.go:97-162` already tests this scenario at a smaller scale:
- Uses 10 concurrent requests with same user
- Expects 1 success, 9 ErrAlreadyClaimed
- This stress test REPLICATES the exact same pattern

**Key Pattern from Existing Test (lines 115-128):**
```go
// Execute: 10 concurrent claims by the SAME user
var wg sync.WaitGroup
results := make(chan error, 10)

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := couponService.ClaimCoupon(ctx, "same_user", "SAME_USER_TEST")
        results <- err
    }()
}

wg.Wait()
close(results)
```

### Service Layer Error Types

From `internal/service/errors.go`:
```go
var (
    ErrAlreadyClaimed = errors.New("coupon already claimed by user")
    ErrNoStock = errors.New("coupon out of stock")
)
```

**CRITICAL:** Use `errors.Is(err, service.ErrAlreadyClaimed)` to check for duplicate claim errors.

### Database Constraint Mechanics

The unique constraint prevents double-dipping at the database level:
```sql
CREATE TABLE claims (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, coupon_name)  -- Prevents same user from claiming twice
);
```

When a duplicate INSERT is attempted:
1. PostgreSQL returns a unique constraint violation error
2. pgx translates this to a specific error code
3. The service layer catches this and returns `ErrAlreadyClaimed`
4. The transaction is rolled back (no stock decrement occurs)

### Why 10 Concurrent Requests?

Per FR24 in the epics file:
> "FR24: System includes Double Dip stress test (10 concurrent same-user requests -> exactly 1 claim)"

The spec explicitly requires 10 concurrent requests.

### Reusing tests/stress/ Setup

If `tests/stress/setup_test.go` was created in story 4-3, **reuse it**. Do not duplicate the TestMain function.

Check before creating:
```bash
ls -la tests/stress/
```

If exists, only create `double_dip_test.go`.

### Expected Test Output

```
=== RUN   TestDoubleDip
    double_dip_test.go:XX: Starting double dip stress test: 10 concurrent same-user requests
    double_dip_test.go:XX: Results - Successes: 1, AlreadyClaimed: 9, Other: 0
    double_dip_test.go:XX: Database verification - remaining_amount: 99, claim_count: 1
--- PASS: TestDoubleDip (X.XXs)
PASS
```

### Library/Framework Requirements

| Library | Version | Purpose |
|---------|---------|---------|
| testify | latest | Assertions (assert, require) |
| dockertest/v3 | latest | PostgreSQL container lifecycle |
| pgx/v5 | v5.x | Database operations |
| sync | stdlib | WaitGroup for goroutine coordination |

### Project Structure Notes

**Files to Create/Modify:**
```
tests/
├── integration/                 # Existing - DO NOT MODIFY
│   ├── setup_test.go
│   ├── coupon_integration_test.go
│   └── concurrency_test.go
└── stress/                      # From story 4-3 OR create new
    ├── setup_test.go            # Reuse from 4-3 if exists
    ├── flash_sale_test.go       # From story 4-3
    └── double_dip_test.go       # NEW - Create this file
```

### Previous Story Intelligence

**From Story 4-3 (Flash Sale Stress Test):**
- tests/stress/ directory and setup may already exist
- Dockertest setup uses PostgreSQL 15-alpine
- Container auto-removes after 120 seconds
- Schema migrations are applied inline in TestMain
- `cleanupTables(t)` uses `TRUNCATE TABLE claims, coupons CASCADE`

**From tests/integration/concurrency_test.go (lines 97-162):**
- `TestConcurrentClaimsSameUser` provides EXACT working pattern
- Error checking: `errors.Is(err, service.ErrAlreadyClaimed)`
- Verification pattern: count successes vs alreadyClaimed vs otherErrors

### NFR Compliance

This story addresses:
- **NFR2**: System handles 10 concurrent same-user requests with exactly 1 success
- **NFR5**: No goroutine leaks or resource exhaustion under stress test load
- **NFR6**: Stress tests pass 100% of runs (no flaky tests)
- **NFR7**: Race detector reports zero data races

### Anti-Patterns to AVOID

1. **DO NOT** check for ErrNoStock - the coupon has 100 stock, failures are due to ErrAlreadyClaimed
2. **DO NOT** use different user IDs - ALL 10 requests must use the SAME user ID
3. **DO NOT** use mock database - stress tests require real PostgreSQL
4. **DO NOT** run tests without `-race` flag during verification
5. **DO NOT** use `time.Sleep` for synchronization - use sync.WaitGroup
6. **DO NOT** forget to close the results channel after WaitGroup.Wait()
7. **DO NOT** duplicate TestMain if it already exists from story 4-3

### Test Implementation Template

```go
func TestDoubleDip(t *testing.T) {
    cleanupTables(t)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Setup: Create coupon with plenty of stock (100)
    _, err := testPool.Exec(ctx,
        "INSERT INTO coupons (name, amount, remaining_amount) VALUES ($1, $2, $3)",
        "DOUBLE_TEST", 100, 100)
    require.NoError(t, err)

    // Setup service
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

    // Execute: 10 concurrent claims by SAME user
    concurrentRequests := 10
    var wg sync.WaitGroup
    results := make(chan error, concurrentRequests)

    t.Logf("Starting double dip stress test: %d concurrent same-user requests", concurrentRequests)

    for i := 0; i < concurrentRequests; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := couponService.ClaimCoupon(ctx, "user_greedy", "DOUBLE_TEST")
            results <- err
        }()
    }

    wg.Wait()
    close(results)

    // Verify: exactly 1 success, 9 ErrAlreadyClaimed
    var successes, alreadyClaimed, otherErrors int
    for err := range results {
        if err == nil {
            successes++
        } else if errors.Is(err, service.ErrAlreadyClaimed) {
            alreadyClaimed++
        } else {
            otherErrors++
            t.Logf("Unexpected error: %v", err)
        }
    }

    t.Logf("Results - Successes: %d, AlreadyClaimed: %d, Other: %d", successes, alreadyClaimed, otherErrors)

    assert.Equal(t, 1, successes, "Exactly one claim should succeed")
    assert.Equal(t, 9, alreadyClaimed, "Nine claims should fail with ErrAlreadyClaimed")
    assert.Equal(t, 0, otherErrors, "No other errors should occur")

    // Verify database state: remaining_amount = 99 (only 1 successful claim)
    var remainingAmount int
    err = testPool.QueryRow(ctx,
        "SELECT remaining_amount FROM coupons WHERE name = $1",
        "DOUBLE_TEST").Scan(&remainingAmount)
    require.NoError(t, err)
    assert.Equal(t, 99, remainingAmount, "remaining_amount should be 99")

    // Verify: exactly 1 claim record for user_greedy
    var claimCount int
    err = testPool.QueryRow(ctx,
        "SELECT COUNT(*) FROM claims WHERE user_id = $1 AND coupon_name = $2",
        "user_greedy", "DOUBLE_TEST").Scan(&claimCount)
    require.NoError(t, err)
    assert.Equal(t, 1, claimCount, "Exactly 1 claim record should exist for user_greedy")

    t.Logf("Database verification - remaining_amount: %d, claim_count: %d", remainingAmount, claimCount)
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.4-Double-Dip-Stress-Test]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing-Strategy]
- [Source: docs/project-context.md#Testing-Requirements]
- [Source: tests/integration/concurrency_test.go:97-162 - TestConcurrentClaimsSameUser pattern]
- [Source: tests/integration/setup_test.go - dockertest configuration]
- [Source: internal/service/errors.go - ErrAlreadyClaimed definition]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- Race detection test: Zero race conditions detected
- 10 consecutive runs: 100% pass rate (10/10)
- All runs consistently show: 1 success, 9 ErrAlreadyClaimed, 0 other errors

### Completion Notes List

- **Task 1**: Reused existing `tests/stress/` directory and `setup_test.go` from story 4-3. Directory already contained TestMain with dockertest, PostgreSQL 15-alpine, migrations, and `cleanupTables()` helper.

- **Task 2**: Created `tests/stress/double_dip_test.go` implementing the double dip stress test. Uses 10 concurrent goroutines with sync.WaitGroup, all claiming with the SAME user_id "user_greedy" to test the UNIQUE(user_id, coupon_name) constraint.

- **Task 3**: Implemented comprehensive result verification:
  - Asserts exactly 1 success
  - Asserts exactly 9 ErrAlreadyClaimed failures
  - Asserts 0 other errors
  - Verifies remaining_amount = 99
  - Verifies exactly 1 claim record exists
  - Verifies claim record belongs to "user_greedy"

- **Task 4**: Added reliability checks:
  - 30-second context timeout
  - Execution time logging
  - Comprehensive test documentation in code comments explaining NFR2 compliance

- **Task 5**: Verified race detection and flakiness:
  - `go test -race ./tests/stress/... -run TestDoubleDip -v` - PASS, zero race conditions
  - 10 consecutive test runs - 100% pass rate
  - Consistent results across all runs

### Change Log

- 2026-01-11: Implemented double dip stress test (story 4-4)
- 2026-01-11: Code review fixes applied (7 issues resolved)

### File List

- tests/stress/double_dip_test.go (NEW, then MODIFIED during review)

## Senior Developer Review (AI)

### Review Date
2026-01-11

### Review Outcome
**APPROVED** - All issues fixed automatically

### Issues Found & Resolved

**MEDIUM (4 issues - all fixed):**
1. **M1**: Missing test for context cancellation handling → Added `TestDoubleDip_ContextCancellation`
2. **M2**: No performance upper-bound assertion → Added 5s threshold check
3. **M3**: Hardcoded magic numbers without explanation → Added design note documenting why stock=100
4. **M4**: No documentation for why ErrNoStock shouldn't occur → Documented in function comment

**LOW (3 issues - all fixed):**
1. **L1**: Missing package-level documentation → Added package doc comment
2. **L2**: Inconsistent AC reference style → Updated to match story format (AC #1, #2, etc.)
3. **L3**: Missing claimed_by list verification → Added service layer GetByName verification

### Verification Results
- All tests pass with `-race` flag
- golangci-lint: 0 issues
- 3 consecutive test runs: 100% pass rate
- New context cancellation test validates graceful shutdown

### Files Modified During Review
- `tests/stress/double_dip_test.go` - Added context cancellation test, performance assertion, documentation improvements, claimed_by verification
