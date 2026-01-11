# Story 6.1: Scale Stress Tests

Status: done

## Story

As a **maintainer**,
I want **stress tests scaled to 100-500 concurrent claims (CI-only)**,
So that **system resilience is proven beyond the spec requirements of 50 concurrent**.

## Acceptance Criteria

1. **AC1:** Given the CI pipeline runs the scale stress test job, when 100 concurrent goroutines attempt to claim a coupon with stock=10, then exactly 10 claims succeed (200/201 responses), and exactly 90 claims fail (400 out of stock), and remaining_amount is exactly 0, and test completes without race conditions (`-race` flag)

2. **AC2:** Given the CI pipeline runs the scale stress test job, when 200 concurrent goroutines attempt to claim a coupon with stock=20, then exactly 20 claims succeed, and test completes within 60 seconds

3. **AC3:** Given the CI pipeline runs the scale stress test job, when 500 concurrent goroutines attempt to claim a coupon with stock=50, then exactly 50 claims succeed, and no database connection pool exhaustion occurs, and test is tagged with `//go:build ci` to prevent local execution

## Tasks / Subtasks

- [x] Task 1: Create CI-only build tags infrastructure (AC: #3)
  - [x] Create new test file `tests/stress/scale_test.go` with `//go:build ci` tag
  - [x] Verify build tags work correctly (test not included in `go test ./...` by default)
  - [x] Document build tag usage in test file header comments

- [x] Task 2: Implement 100-concurrent scale stress test (AC: #1)
  - [x] Create `TestScaleStress100` function
  - [x] Configure: 100 goroutines, stock=10, timeout=60s
  - [x] Use existing `testPool` and service patterns from `flash_sale_test.go`
  - [x] Assert exactly 10 successes, 90 no-stock failures
  - [x] Verify remaining_amount = 0, claim_count = 10
  - [x] Add detailed logging for CI debugging

- [x] Task 3: Implement 200-concurrent scale stress test (AC: #2)
  - [x] Create `TestScaleStress200` function
  - [x] Configure: 200 goroutines, stock=20, timeout=60s
  - [x] Assert exactly 20 successes, 180 no-stock failures
  - [x] Verify database state consistency

- [x] Task 4: Implement 500-concurrent scale stress test (AC: #3)
  - [x] Create `TestScaleStress500` function
  - [x] Configure: 500 goroutines, stock=50, timeout=120s
  - [x] Monitor for connection pool exhaustion
  - [x] Add connection pool metrics logging
  - [x] Assert exactly 50 successes, 450 no-stock failures

- [x] Task 5: Update CI workflow to run scale tests (AC: #1, #2, #3)
  - [x] Add new CI job or step for scale stress tests
  - [x] Use `-tags ci` flag to include CI-only tests
  - [x] Ensure PostgreSQL service is available
  - [x] Set appropriate timeout for 500-concurrent test

## Dev Notes

### Relevant Architecture Patterns and Constraints

**From Architecture Document:**
- Transaction pattern: `BEGIN → SELECT FOR UPDATE → CHECK → INSERT → UPDATE → COMMIT`
- Row locking prevents race conditions
- UNIQUE constraint on (user_id, coupon_name) prevents duplicates

**From Project Context:**
- Technology Stack: Fiber v2 + pgx v5 + PostgreSQL 15+
- Testing: testify for assertions, dockertest for container lifecycle
- Quality Gates: `go test -race ./...` required

### Source Tree Components to Touch

| File | Action | Purpose |
|------|--------|---------|
| `tests/stress/scale_test.go` | CREATE | New file for CI-only scale tests |
| `.github/workflows/ci.yml` | MODIFY | Add scale test job with `-tags ci` |
| `tests/stress/setup_test.go` | REVIEW | May need pool size adjustments for 500-concurrent |

### Testing Standards Summary

**CI-Only Test Pattern:**
```go
//go:build ci

package stress
```

**Test Pattern (from existing flash_sale_test.go):**
```go
func TestScaleStress100(t *testing.T) {
    cleanupTables(t)

    const (
        couponName         = "SCALE_100_TEST"
        availableStock     = 10
        concurrentRequests = 100
        timeout            = 60 * time.Second
    )

    // Use existing service setup pattern
    couponRepo := repository.NewCouponRepository(testPool)
    claimRepo := repository.NewClaimRepository(testPool)
    couponService := service.NewCouponService(testPool, couponRepo, claimRepo)

    // ... concurrent goroutine pattern from flash_sale_test.go
}
```

### Project Structure Notes

**Test File Location:**
- `tests/stress/scale_test.go` - New CI-only scale tests
- Follows existing structure: `tests/stress/*.go`

**Build Tag Behavior:**
- Without `-tags ci`: Tests in `scale_test.go` excluded
- With `-tags ci`: Full test suite including scale tests

### CI Workflow Modification

**Add to `.github/workflows/ci.yml`:**
```yaml
# New job or step for scale stress tests
- name: Run Scale Stress Tests (CI-only)
  run: |
    go test -race -v -tags ci ./tests/stress/... -run "TestScaleStress"
  timeout-minutes: 5
```

### Connection Pool Considerations

**From pgxpool defaults:**
- Default max connections: 4 * runtime.NumCPU()
- For 500 concurrent: may need pool tuning

**Monitoring pattern:**
```go
stats := testPool.Stat()
t.Logf("Pool stats - Total: %d, Idle: %d, In-Use: %d",
    stats.TotalConns(),
    stats.IdleConns(),
    stats.AcquiredConns())
```

### Previous Story Intelligence

**From Epic 4 (Flash Sale Test Implementation):**
- sync.WaitGroup pattern for goroutine coordination
- Buffered channel for collecting results
- Database verification after concurrent operations
- Execution time logging for performance tracking

**From Epic 5 Retrospective:**
- Go version locked at 1.25.5
- CI pipeline completes in ~1m27s (well under 10min target)
- 94.7% test coverage achieved

### Epic 6 Context

**This is the first story in Epic 6: Advanced Testing & Chaos Engineering**

Epic 6 builds on the solid foundation from Epics 1-5 to prove system resilience through chaos engineering tests that exceed spec requirements.

**Key Constraint:** All Epic 6 tests are CI-ONLY - not intended for local execution due to resource requirements and infrastructure dependencies.

**Remaining Epic 6 Stories:**
- 6.2: Database Resilience Testing
- 6.3: Input Boundary Testing
- 6.4: Transaction Edge Cases
- 6.5: Mixed Load & Chaos Testing

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 6: Advanced Testing & Chaos Engineering]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing Strategy]
- [Source: docs/project-context.md#Testing Requirements]
- [Source: tests/stress/flash_sale_test.go - Pattern reference]
- [Source: tests/stress/setup_test.go - Test infrastructure]
- [Source: _bmad-output/implementation-artifacts/epic-5-retro-2026-01-11.md - Previous context]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- All scale stress tests pass with `-race` flag (no race conditions detected)
- Fixed race condition in TestScaleStress500 by using atomic operations for pool metrics tracking

### Completion Notes List

- Created `tests/stress/scale_test.go` with `//go:build ci` tag for CI-only execution
- Implemented TestScaleStress100: 100 concurrent goroutines, stock=10, all assertions pass (~73ms)
- Implemented TestScaleStress200: 200 concurrent goroutines, stock=20, all assertions pass (~98ms)
- Implemented TestScaleStress500: 500 concurrent goroutines, stock=50, all assertions pass (~192ms)
- Added connection pool monitoring with atomic operations for thread safety
- Updated CI workflow with new "Run scale stress tests (CI-only)" step using `-tags ci -run "TestScaleStress"`
- Build tags verified: scale tests excluded from default `go test ./...`, included with `-tags ci`
- All tests complete within timeout limits (60s for 100/200, 120s for 500)

### File List

| File | Action | Description |
|------|--------|-------------|
| `tests/stress/scale_test.go` | CREATE | CI-only scale stress tests (100/200/500 concurrent) |
| `.github/workflows/ci.yml` | MODIFY | Added scale stress test step with `-tags ci` |

### Change Log

- 2026-01-11: Implemented Story 6.1 - Scale Stress Tests
  - Added CI-only scale stress tests for 100, 200, and 500 concurrent claims
  - Tests use `//go:build ci` tag to prevent local execution
  - CI workflow updated to run scale tests with appropriate timeout
  - All acceptance criteria satisfied

- 2026-01-11: Code Review Fixes (AI)
  - Removed premature chaos tests step from CI workflow (belongs to Stories 6.2/6.4/6.5)
  - Strengthened pool exhaustion handling in TestScaleStress500:
    - Added explicit assertion for no connection acquisition failures
    - Clarified that reaching max pool capacity != pool exhaustion
    - Changed WARNING to INFO for pool utilization logging

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5 (Adversarial Review)
**Date:** 2026-01-11
**Outcome:** APPROVED with fixes applied

### Review Summary

All acceptance criteria verified:
- AC1: ✅ 100 concurrent claims properly tested with race detection
- AC2: ✅ 200 concurrent claims complete well under 60s (~98ms)
- AC3: ✅ 500 concurrent claims with pool exhaustion assertion

### Issues Found & Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| MEDIUM | CI workflow included chaos tests step (belongs to other stories) | Removed from 6.1, to be added in appropriate story |
| MEDIUM | Pool exhaustion handling was warning-only, not assertion | Added explicit assertion for connection acquisition failures |
| LOW | Pool exhaustion terminology was unclear | Added comments clarifying max capacity vs exhaustion |

### Verification

- Build tags confirmed working (tests excluded without `-tags ci`)
- All 3 scale tests pass consistently with `-race` flag
- Linting passes with no issues
- Tests complete well under timeout limits

