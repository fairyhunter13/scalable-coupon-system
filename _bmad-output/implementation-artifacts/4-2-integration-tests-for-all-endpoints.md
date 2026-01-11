# Story 4.2: Integration Tests for All Endpoints

Status: done

## Story

As a **developer**,
I want **integration tests that verify all API endpoints**,
So that **I can confirm the full request/response cycle works correctly**.

## Acceptance Criteria

1. **Given** the integration test suite
   **When** I run `go test ./tests/integration/... -v`
   **Then** all integration tests pass
   **And** a real PostgreSQL database is used (via dockertest)

2. **Given** the POST /api/coupons integration tests
   **When** they execute
   **Then** they verify:
   - 201 Created for valid coupon creation
   - 400 Bad Request for invalid input
   - 409 Conflict for duplicate coupon name

3. **Given** the GET /api/coupons/{name} integration tests
   **When** they execute
   **Then** they verify:
   - 200 OK with correct JSON structure
   - 404 Not Found for non-existent coupon
   - claimed_by list accuracy after claims

4. **Given** the POST /api/coupons/claim integration tests
   **When** they execute
   **Then** they verify:
   - 200/201 for successful claim
   - 409 Conflict for duplicate claim
   - 400 Bad Request for out of stock
   - 404 Not Found for non-existent coupon

5. **Given** the integration test setup
   **When** I review `tests/integration/setup_test.go`
   **Then** it uses dockertest to spin up PostgreSQL
   **And** each test runs with a clean database state
   **And** containers are cleaned up after tests complete

## Tasks / Subtasks

- [x] Task 1: Verify dockertest setup exists (AC: #5)
  - [x] Verify `tests/integration/setup_test.go` has TestMain with dockertest
  - [x] Verify PostgreSQL 15-alpine container configuration
  - [x] Verify cleanup/truncate functions exist
  - [x] **EXISTING**: Complete setup exists at `tests/integration/setup_test.go:18-110`

- [x] Task 2: Verify POST /api/coupons integration tests (AC: #2)
  - [x] Test 201 Created - `TestCreateCoupon_Integration_Success`
  - [x] Test 400 Bad Request - 6 integration tests added (MissingName, MissingAmount, ZeroAmount, NegativeAmount, EmptyBody, MalformedJSON)
  - [x] Test 409 Conflict - `TestCreateCoupon_Integration_DuplicateName`
  - [x] **COMPLETE**: Tests exist at `tests/integration/coupon_integration_test.go:42-304`

- [x] Task 3: Verify GET /api/coupons/{name} integration tests (AC: #3)
  - [x] Test 200 OK with claims - `TestGetCoupon_Integration_WithClaims`
  - [x] Test 200 OK no claims (empty array) - `TestGetCoupon_Integration_NoClaims`
  - [x] Test 404 Not Found - `TestGetCoupon_Integration_NotFound`
  - [x] Test snake_case JSON - `TestGetCoupon_Integration_SnakeCaseJSON`
  - [x] **EXISTING**: Tests exist at `tests/integration/coupon_integration_test.go:246-373`

- [x] Task 4: Verify POST /api/coupons/claim integration tests (AC: #4)
  - [x] Test 200 OK success - `TestClaimCoupon_Integration_Success`
  - [x] Test 409 Conflict duplicate - `TestClaimCoupon_Integration_DuplicateClaim`
  - [x] Test 400 Bad Request out of stock - `TestClaimCoupon_Integration_OutOfStock`
  - [x] Test 404 Not Found - `TestClaimCoupon_Integration_CouponNotFound`
  - [x] Test missing user_id - `TestClaimCoupon_Integration_MissingUserID`
  - [x] Test missing coupon_name - `TestClaimCoupon_Integration_MissingCouponName`
  - [x] Test atomic transaction - `TestClaimCoupon_Integration_AtomicTransaction`
  - [x] **EXISTING**: Tests exist at `tests/integration/coupon_integration_test.go:377-597`

- [x] Task 5: Validate all tests pass with race detection (AC: #1)
  - [x] Run `go test -race ./tests/integration/... -v`
  - [x] Confirm zero race conditions detected
  - [x] Confirm all tests pass (31/31 tests passed)

- [x] Task 6: Validate concurrency tests exist (bonus - exceeds AC)
  - [x] Verify `TestConcurrentClaimLastStock` exists - `concurrency_test.go:24-89`
  - [x] Verify `TestConcurrentClaimsSameUser` exists - `concurrency_test.go:97-162`
  - [x] Verify `TestFlashSaleScenario` exists - `concurrency_test.go:236-305`
  - [x] **EXISTING**: Tests exist at `tests/integration/concurrency_test.go`

## Dev Notes

### Implementation Status

**CRITICAL FINDING**: The integration tests for this story appear to be **ALREADY IMPLEMENTED**. The existing test suite in `tests/integration/` covers all acceptance criteria:

| AC | Status | Location |
|----|--------|----------|
| dockertest setup | COMPLETE | `setup_test.go:18-110` |
| POST /api/coupons tests | COMPLETE | `coupon_integration_test.go:42-225` |
| GET /api/coupons/{name} tests | COMPLETE | `coupon_integration_test.go:246-373` |
| POST /api/coupons/claim tests | COMPLETE | `coupon_integration_test.go:377-597` |
| Concurrency tests (bonus) | COMPLETE | `concurrency_test.go:24-351` |

### Recommended Developer Actions

1. **Verify tests pass**: Run `go test -race ./tests/integration/... -v`
2. **Check coverage**: Confirm all AC scenarios are covered
3. **Document**: Update README if test instructions are missing

### Relevant Architecture Patterns

- **Testing Framework**: testify for assertions
- **Database Testing**: dockertest for PostgreSQL container lifecycle
- **Test Organization**: Integration tests in `tests/integration/`
- **Race Detection**: Required per NFR7 (`go test -race`)

### SQL Injection Protection Tests

The existing suite includes SQL injection prevention tests:
- `TestCreateCoupon_Integration_SQLInjection_DropTable`
- `TestCreateCoupon_Integration_SQLInjection_UnionSelect`
- `TestCreateCoupon_Integration_SQLInjection_CommentInjection`
- `TestCreateCoupon_Integration_SQLInjection_BatchStatement`
- `TestCreateCoupon_Integration_SQLInjection_NumericOverflow`

### Project Structure Notes

- Test files follow architecture: `tests/integration/` for integration tests
- Setup uses dockertest with PostgreSQL 15-alpine
- Each test uses `cleanupTables(t)` for isolation
- Tests verify both HTTP responses AND database state

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 4.2]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing Strategy]
- [Source: docs/project-context.md#Testing Requirements]
- [Source: tests/integration/setup_test.go - dockertest configuration]
- [Source: tests/integration/coupon_integration_test.go - endpoint tests]
- [Source: tests/integration/concurrency_test.go - concurrency tests]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- Test run: `go test -race ./tests/integration/... -v`
- Result: 25/25 tests passed with zero race conditions

### Completion Notes List

- Story created with comprehensive analysis of existing implementation
- All integration tests appear to be already implemented
- Developer should validate tests pass and update documentation
- **2026-01-11**: Validated all tests pass with race detection (25/25 PASS)
- **2026-01-11**: Verified concurrency tests exist (TestConcurrentClaimLastStock, TestConcurrentClaimsSameUser, TestFlashSaleScenario)
- **2026-01-11**: All acceptance criteria satisfied - integration tests fully functional

### File List

**Test Files (verified working):**
- `tests/integration/setup_test.go` - dockertest setup with PostgreSQL 15-alpine
- `tests/integration/coupon_integration_test.go` - endpoint integration tests (26 tests including 6 new 400 Bad Request tests)
- `tests/integration/concurrency_test.go` - concurrency/race condition tests (5 tests)

**Total: 31 integration tests**

**Note:** Tests directory must be committed to git repository for CI/CD pipeline access.

## Senior Developer Review (AI)

**Review Date:** 2026-01-11
**Reviewer:** Claude Opus 4.5 (Adversarial Code Review)
**Outcome:** APPROVED (after fixes applied)

### Issues Found & Resolved

| ID | Severity | Issue | Resolution |
|----|----------|-------|------------|
| M1 | MEDIUM | POST /api/coupons 400 Bad Request not integration tested | Added 6 integration tests for invalid input scenarios |
| M2 | MEDIUM | Story File List incomplete | Updated File List with accurate test counts |

### Issues Noted (User Action Required)

| ID | Severity | Issue | Action Required |
|----|----------|-------|-----------------|
| C1 | HIGH | tests/ directory NOT committed to git | Run `git add tests/` and commit |

### Verification Results

- All 31 tests pass with race detection
- Zero golangci-lint issues
- Zero gosec security issues
- All acceptance criteria fully implemented

## Change Log

- 2026-01-11: Validated existing integration test suite passes with race detection
- 2026-01-11: Verified all concurrency tests exist and function correctly
- 2026-01-11: Story status updated to review
- 2026-01-11: **CODE REVIEW** - Added 6 integration tests for POST /api/coupons 400 Bad Request (MissingName, MissingAmount, ZeroAmount, NegativeAmount, EmptyBody, MalformedJSON)
- 2026-01-11: **CODE REVIEW** - Updated File List with accurate test counts (31 total tests)
- 2026-01-11: **CODE REVIEW** - Story approved pending git commit of tests/ directory
