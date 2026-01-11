# Story 6.6: Restructure CI Pipeline with Staged Quality Gates

Status: done

## Story

As a **maintainer**,
I want **the CI pipeline restructured so Stage 2 tests only run after Stage 1 (unit tests with 80% coverage, lint, security) passes**,
So that **expensive database tests don't waste resources when basic quality checks fail, and I get faster feedback on code quality issues**.

## Acceptance Criteria

1. **AC1: Stage 1 Parallel Execution**
   **Given** a push to any branch or a pull request
   **When** the CI workflow triggers
   **Then** three Stage 1 jobs start in parallel:
   - `unit-tests`: Runs `go test -race -coverprofile=coverage.out ./internal/...` with 80% coverage enforcement
   - `lint`: Runs `golangci-lint run ./...` and `go vet ./...`
   - `security`: Runs `gosec ./...` and `govulncheck ./...`
   **And** all three jobs run concurrently (not sequentially)

2. **AC2: Coverage Threshold Enforcement (80%)**
   **Given** the `unit-tests` job completes with coverage >= 80%
   **When** the coverage is evaluated
   **Then** the job passes successfully
   **And** the coverage percentage is displayed in the job output

   **Given** the `unit-tests` job completes with coverage < 80%
   **When** the coverage is evaluated
   **Then** the job **fails hard** (exit code non-zero)
   **And** an error message clearly states: "Coverage XX.X% is below required threshold of 80%"
   **And** Stage 2 jobs are blocked from running

3. **AC3: Stage 2 Dependent Execution (includes Stories 6.1-6.5 tests)**
   **Given** ALL Stage 1 jobs pass
   **When** Stage 1 completes successfully
   **Then** Stage 2 jobs begin execution in parallel:
   - `integration-tests`: API endpoint tests with concurrency tests
   - `stress-tests`: Flash Sale, Double Dip, Scale tests (Story 6.1)
   - `chaos-tests`: DB Resilience (6.2), Input Boundary (6.3), Transaction Edge Cases (6.4), Mixed Load (6.5)

   **Given** ANY Stage 1 job fails
   **When** that job reports failure
   **Then** Stage 2 jobs are **never started**
   **And** they are marked as "skipped" in the workflow

4. **AC4: Stage 2 Job Dependencies**
   **Given** the CI workflow file `.github/workflows/ci.yml`
   **When** I review Stage 2 job definitions
   **Then** all Stage 2 jobs have:
   ```yaml
   needs: [unit-tests, lint, security]
   ```

5. **AC5: Test Coverage Verification**
   **Given** the Stage 2 jobs execute
   **When** I verify the test execution
   **Then** these specific test files are run:
   - `tests/stress/scale_test.go` (Story 6.1)
   - `tests/chaos/db_resilience_test.go` (Story 6.2)
   - `tests/chaos/input_boundary_test.go` (Story 6.3)
   - `tests/chaos/transaction_edge_cases_test.go` (Story 6.4)
   - `tests/chaos/mixed_load_test.go` (Story 6.5)
   **And** all existing integration and stress tests continue to run

6. **AC6: Pipeline Visualization Structure**
   **Given** the complete pipeline execution
   **When** I view the GitHub Actions UI
   **Then** the staged structure is visually apparent:
   ```
   STAGE 1 (parallel): unit-tests | lint | security
                                 |
                         ALL MUST PASS
                                 |
   STAGE 2 (parallel): integration-tests | stress-tests | chaos-tests
   ```

7. **AC7: README Documentation**
   **Given** the README.md file
   **When** I read the "CI/CD Pipeline" section
   **Then** it documents the staged structure and 80% coverage requirement
   **And** it explains why Stage 2 depends on Stage 1

## Tasks / Subtasks

- [x] Task 1: Analyze current CI structure (AC: #1, #3)
  - [x] 1.1: Review current `.github/workflows/ci.yml` job structure
  - [x] 1.2: Identify which jobs are Stage 1 (fast, no DB) vs Stage 2 (slow, DB-dependent)
  - [x] 1.3: Document current job dependencies and timing

- [x] Task 2: Restructure Stage 1 jobs (AC: #1, #2)
  - [x] 2.1: Create/modify `unit-tests` job - runs `go test -race ./internal/...` only
  - [x] 2.2: Ensure `unit-tests` generates coverage profile and checks 80% threshold
  - [x] 2.3: Verify `lint` job runs golangci-lint and go vet in parallel with unit-tests
  - [x] 2.4: Verify `security` job runs gosec and govulncheck in parallel with unit-tests
  - [x] 2.5: Remove any DB service dependencies from Stage 1 jobs

- [x] Task 3: Create Stage 2 jobs with dependencies (AC: #3, #4, #5)
  - [x] 3.1: Create/modify `integration-tests` job with `needs: [unit-tests, lint, security]`
  - [x] 3.2: Create/modify `stress-tests` job with `needs: [unit-tests, lint, security]`
  - [x] 3.3: Modify `chaos-tests` job with `needs: [unit-tests, lint, security]`
  - [x] 3.4: Ensure all Stage 2 jobs have PostgreSQL service attached
  - [x] 3.5: Verify `-tags ci` flag used for CI-only tests (scale, chaos)

- [x] Task 4: Remove redundant coverage step from Stage 2 (AC: #2)
  - [x] 4.1: Remove coverage generation from Stage 2 `test` job (if exists)
  - [x] 4.2: Keep only the unit-tests job responsible for coverage reporting
  - [x] 4.3: Ensure coverage artifact upload happens in Stage 1

- [x] Task 5: Verify existing tests still run (AC: #5)
  - [x] 5.1: Confirm integration tests include: `./tests/integration/...`
  - [x] 5.2: Confirm stress tests include: `./tests/stress/...` (Flash Sale, Double Dip, Scale)
  - [x] 5.3: Confirm chaos tests include: `./tests/chaos/...` (all 6.2-6.5 tests)
  - [x] 5.4: Run local verification of all test paths

- [x] Task 6: Update README documentation (AC: #7)
  - [x] 6.1: Add "CI/CD Pipeline Structure" section to README.md
  - [x] 6.2: Document Stage 1 jobs and their purpose
  - [x] 6.3: Document Stage 2 jobs and their purpose
  - [x] 6.4: Explain 80% coverage requirement and why Stage 2 depends on Stage 1
  - [x] 6.5: Include pipeline visualization diagram

- [x] Task 7: Verify CI pipeline execution (AC: #6)
  - [x] 7.1: Push changes to trigger CI workflow
  - [x] 7.2: Use `gh run watch` to monitor execution
  - [x] 7.3: Verify Stage 1 jobs run in parallel
  - [x] 7.4: Verify Stage 2 jobs only start after all Stage 1 jobs pass
  - [x] 7.5: Test failure scenario: verify Stage 2 skipped when Stage 1 fails

## Dev Notes

### Current CI Structure Analysis

**Current `.github/workflows/ci.yml` structure:**

```yaml
jobs:
  build:
    name: Build
    # No dependencies - runs immediately

  test:
    name: Test
    services:
      postgres: ...  # DB attached
    steps:
      - Run unit tests
      - Run integration tests
      - Run stress tests
      - Run scale stress tests (CI-only)
      - Generate coverage report
      - Check coverage threshold
      - Upload coverage report

  lint:
    name: Lint
    # No dependencies - runs immediately

  security:
    name: Security
    # No dependencies - runs immediately

  chaos:
    name: Chaos Tests
    needs: [build]  # Only depends on build
    # Uses dockertest - self-contained
```

**Problem:** The current `test` job combines unit tests, integration tests, stress tests, and coverage in one monolithic job. This means:
1. All tests require PostgreSQL service even for unit tests
2. Expensive integration/stress tests run even if lint/security fails
3. No clear separation between fast checks and slow checks

### Target CI Structure

**Stage 1 (Parallel - No DB Required):**
```yaml
jobs:
  unit-tests:
    name: Unit Tests
    # No postgres service needed
    steps:
      - Run unit tests: go test -race -coverprofile=coverage.out ./internal/...
      - Check coverage >= 80%
      - Upload coverage artifact

  lint:
    name: Lint
    steps:
      - golangci-lint run ./...
      - go vet ./...

  security:
    name: Security
    steps:
      - gosec ./...
      - govulncheck ./...
```

**Stage 2 (Parallel - DB Required - Depends on Stage 1):**
```yaml
  integration-tests:
    name: Integration Tests
    needs: [unit-tests, lint, security]  # CRITICAL
    services:
      postgres: ...
    steps:
      - go test -v -race ./tests/integration/...

  stress-tests:
    name: Stress Tests
    needs: [unit-tests, lint, security]  # CRITICAL
    services:
      postgres: ...
    steps:
      - go test -v -race -count=1 ./tests/stress/...
      - go test -v -race -tags ci -count=1 ./tests/stress/... -run "TestScaleStress"

  chaos-tests:
    name: Chaos Tests
    needs: [unit-tests, lint, security]  # CHANGED from [build]
    steps:
      - go test -v -race -tags ci -count=1 ./tests/chaos/...
```

### Critical Implementation Details

**1. Unit Tests Coverage (Stage 1):**
```yaml
unit-tests:
  name: Unit Tests & Coverage
  runs-on: ubuntu-latest
  steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ env.GO_VERSION }}
        cache: true

    - name: Run unit tests with coverage
      run: |
        go test -race -coverprofile=coverage.out -coverpkg=./internal/...,./pkg/... ./internal/...

    - name: Check coverage threshold
      run: |
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')
        echo "Total coverage: ${COVERAGE}%"
        THRESHOLD=80
        if [ "$(echo "$COVERAGE < $THRESHOLD" | bc -l)" -eq 1 ]; then
          echo "::error::Coverage ${COVERAGE}% is below required threshold of ${THRESHOLD}%"
          exit 1
        fi
        echo "::notice::Coverage ${COVERAGE}% meets ${THRESHOLD}% threshold"

    - name: Upload coverage report
      uses: actions/upload-artifact@v4
      with:
        name: coverage-report
        path: coverage.out
```

**2. Stage 2 Job Dependencies:**
```yaml
integration-tests:
  name: Integration Tests
  needs: [unit-tests, lint, security]
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:15-alpine
      # ... config ...
```

**3. Build Job Remains Independent:**
The `build` job should remain independent (no dependencies) since it just compiles the code and is useful for quick validation.

### Migration Strategy

1. **Split the current `test` job:**
   - Extract unit test + coverage to new `unit-tests` job
   - Keep integration tests in separate `integration-tests` job
   - Keep stress tests in separate `stress-tests` job

2. **Update dependencies:**
   - `integration-tests`: `needs: [unit-tests, lint, security]`
   - `stress-tests`: `needs: [unit-tests, lint, security]`
   - `chaos-tests`: `needs: [unit-tests, lint, security]` (was: `needs: [build]`)

3. **Remove postgres from Stage 1:**
   - `unit-tests` job: NO postgres service
   - `lint` job: NO postgres service (already)
   - `security` job: NO postgres service (already)

### Expected Pipeline Flow

```
┌─────────────────── PUSH/PR TRIGGER ───────────────────┐
│                                                        │
▼                                                        │
┌──────────────────────────────────────────────────────┐ │
│              STAGE 1 (Parallel, ~2-3 min)            │ │
│                                                      │ │
│  ┌──────────────┐  ┌──────────┐  ┌───────────────┐  │ │
│  │ unit-tests   │  │  lint    │  │   security    │  │ │
│  │ (coverage)   │  │ golangci │  │ gosec+govuln  │  │ │
│  │   ~90s       │  │  ~60s    │  │    ~45s       │  │ │
│  └──────┬───────┘  └────┬─────┘  └───────┬───────┘  │ │
│         │               │                │          │ │
└─────────┼───────────────┼────────────────┼──────────┘ │
          │               │                │            │
          └───────────────┼────────────────┘            │
                          │                             │
                    ALL MUST PASS                       │
                          │                             │
                          ▼                             │
┌──────────────────────────────────────────────────────┐ │
│              STAGE 2 (Parallel, ~5-8 min)            │ │
│                                                      │ │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │ │
│  │ integration  │  │   stress     │  │   chaos    │ │ │
│  │   tests      │  │   tests      │  │   tests    │ │ │
│  │   ~3 min     │  │ Flash/Double │  │ 6.2-6.5    │ │ │
│  │              │  │ Scale (6.1)  │  │ DB + Edge  │ │ │
│  └──────────────┘  └──────────────┘  └────────────┘ │ │
│                                                      │ │
└──────────────────────────────────────────────────────┘ │
                                                        │
└────────────────────────────────────────────────────────┘
```

### Project Structure Notes

**CI Workflow Location:**
```
.github/
└── workflows/
    └── ci.yml  # Main CI pipeline - THIS FILE IS MODIFIED
```

**Test Directories:**
```
tests/
├── integration/            # Stage 2: integration-tests job
│   ├── setup_test.go
│   ├── coupon_integration_test.go
│   └── concurrency_test.go
├── stress/                 # Stage 2: stress-tests job
│   ├── flash_sale_test.go  # Core stress test
│   ├── double_dip_test.go  # Core stress test
│   └── scale_test.go       # CI-only (Story 6.1)
└── chaos/                  # Stage 2: chaos-tests job
    ├── setup_test.go       # Story 6.2 infrastructure
    ├── db_resilience_test.go    # Story 6.2
    ├── input_boundary_test.go   # Story 6.3
    ├── transaction_edge_cases_test.go  # Story 6.4
    └── mixed_load_test.go       # Story 6.5
```

**Unit Tests Location (Stage 1):**
```
internal/
├── handler/
│   ├── claim_handler_test.go
│   └── coupon_handler_test.go
├── repository/
│   ├── claim_repository_test.go
│   └── coupon_repository_test.go
└── service/
    └── coupon_service_test.go
```

### Previous Story Intelligence

**From Story 6.5 (Mixed Load & Chaos Testing):**
- CI workflow already has separate `chaos` job
- Chaos job uses dockertest (self-contained PostgreSQL)
- Current dependency is `needs: [build]` - needs to change to `needs: [unit-tests, lint, security]`
- All chaos tests tagged with `//go:build ci`

**From Epic 5 (CI/CD Pipeline):**
- Coverage threshold already implemented (80%)
- golangci-lint and gosec already configured
- govulncheck already running
- All quality gates functional

**From Recent Commits:**
- `30ccb1a`: Added chaos tests job to CI
- `9fd5653`: Added scale stress tests for CI (Story 6.1)
- Coverage currently at ~94.7% (well above 80% threshold)

### Git Intelligence

**Recent CI-related commits:**
```
30ccb1a fix(ci): Add chaos tests job and complete Story 6.3 input boundary testing
9fd5653 feat: Add scale stress tests for CI (Story 6.1)
034b09f fix: Update Dockerfile Go version and adjust coverage threshold
```

**Current coverage:** 86.1% (from CI run 20898570747)

### Anti-Patterns to Avoid

1. **DO NOT** keep postgres service on Stage 1 jobs - it slows them down unnecessarily
2. **DO NOT** run integration tests before unit tests pass - wastes CI resources
3. **DO NOT** remove the build job - it provides quick compilation feedback
4. **DO NOT** combine multiple test types in one job - defeats the purpose of staged gates
5. **DO NOT** forget to update chaos-tests dependency from `[build]` to `[unit-tests, lint, security]`
6. **DO NOT** remove the `-race` flag from any test command
7. **DO NOT** change the 80% coverage threshold (NFR11 requirement)

### Library/Framework Requirements

No new dependencies - this story only restructures existing CI workflow.

**Tools used in CI:**
- Go 1.25.5
- golangci-lint v2.5.0
- gosec (latest)
- govulncheck (latest)
- PostgreSQL 15-alpine (for Stage 2)

### Testing Strategy

**Local Verification:**
```bash
# Verify unit tests can run without DB
go test -race ./internal/...

# Verify coverage generation
go test -race -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out | grep total

# Verify integration tests (requires DB)
docker-compose up -d postgres
go test -v -race ./tests/integration/...

# Verify stress tests
go test -v -race -count=1 ./tests/stress/...

# Verify chaos tests (CI-only)
go test -v -race -tags ci ./tests/chaos/...
```

**CI Verification:**
```bash
# After pushing changes
gh run watch

# Check if Stage 1 jobs run in parallel
gh run view <run-id> --log

# Verify Stage 2 jobs waited for Stage 1
gh pr checks
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 6.6: Restructure CI Pipeline with Staged Quality Gates]
- [Source: docs/project-context.md#CI/CD Monitoring (MANDATORY)]
- [Source: docs/project-context.md#Quality Gates (MANDATORY)]
- [Source: .github/workflows/ci.yml - Current CI workflow structure]
- [Source: _bmad-output/implementation-artifacts/6-5-mixed-load-and-chaos-testing.md - Chaos job dependency analysis]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing Strategy]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- CI run 20898570747: All 7 jobs passed successfully
- Coverage: 86.1% (meets 80% threshold per NFR11)

### Completion Notes List

- Restructured CI pipeline into 2-stage architecture
- Stage 1 (parallel, no DB): unit-tests, lint, security - completes in ~37-54s
- Stage 2 (parallel, needs Stage 1): integration-tests, stress-tests, chaos-tests - completes in ~53-62s
- Split monolithic `test` job into separate unit-tests (Stage 1) and integration/stress tests (Stage 2)
- Updated chaos-tests dependency from `[build]` to `[unit-tests, lint, security]`
- Added CI/CD Pipeline section to README with visualization diagram
- 80% coverage enforcement in Stage 1 blocks Stage 2 if coverage drops
- All Stories 6.1-6.5 tests verified running in CI

### File List

- .github/workflows/ci.yml (modified - restructured pipeline)
- README.md (modified - added CI/CD Pipeline section)
- _bmad-output/implementation-artifacts/sprint-status.yaml (modified - status updates)
- _bmad-output/implementation-artifacts/6-6-restructure-ci-pipeline-with-staged-quality-gates.md (modified - task checkmarks, status)

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5 (Adversarial Code Review)
**Date:** 2026-01-11
**Outcome:** APPROVED with fixes applied

### Issues Found and Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| HIGH | Dev Notes listed non-existent `claim_integration_test.go` | Fixed: Updated to actual file `concurrency_test.go` |
| MEDIUM | CI YAML comment incorrectly grouped `build` with Stage 1 gates | Fixed: Clarified `build` is independent, added proper Stage 1 section |
| LOW | Coverage stated as 94.7% but CI shows 86.1% | Fixed: Updated to 86.1% with CI run reference |

### Verification Summary

- All 7 Acceptance Criteria verified as IMPLEMENTED
- All tasks marked [x] confirmed complete
- CI run 20898570747: All 7 jobs passed (Build, Unit Tests, Lint, Security, Integration, Stress, Chaos)
- Coverage: 86.1% (exceeds 80% threshold per NFR11)
- Stage 1 jobs run in parallel (~37-54s)
- Stage 2 jobs correctly depend on Stage 1 via `needs: [unit-tests, lint, security]`

## Change Log

- 2026-01-11: Code review completed - fixed documentation issues (Story 6.6)
- 2026-01-11: Restructured CI pipeline with staged quality gates (Story 6.6)
