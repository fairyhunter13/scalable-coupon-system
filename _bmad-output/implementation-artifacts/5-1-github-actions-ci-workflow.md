# Story 5.1: GitHub Actions CI Workflow

Status: done

## Story

As a **maintainer**,
I want **a GitHub Actions workflow that runs on every push and PR**,
So that **code quality is automatically verified before merging**.

## Acceptance Criteria

1. **Given** a push to any branch
   **When** GitHub Actions triggers
   **Then** the CI workflow starts automatically
   **And** the workflow runs all defined jobs

2. **Given** a pull request is opened or updated
   **When** GitHub Actions triggers
   **Then** the CI workflow runs on the PR
   **And** status checks are reported on the PR

3. **Given** the CI workflow file `.github/workflows/ci.yml`
   **When** I review its structure
   **Then** it defines these jobs:
   - `build`: Builds the Docker image
   - `test`: Runs unit, integration, and stress tests
   - `lint`: Runs linting and static analysis
   - `security`: Runs security scanning

4. **Given** the `test` job
   **When** it executes
   **Then** it spins up PostgreSQL service container
   **And** it waits for PostgreSQL to be healthy
   **And** it runs `go test -race -coverprofile=coverage.out ./...`
   **And** it uploads coverage report as artifact

5. **Given** the `test` job includes stress tests
   **When** stress tests execute in CI
   **Then** Flash Sale test passes (50 concurrent -> 5 claims)
   **And** Double Dip test passes (10 same-user -> 1 claim)
   **And** tests complete within CI timeout limits

6. **Given** the workflow configuration
   **When** I review the PostgreSQL service
   **Then** it uses the same version as docker-compose (PostgreSQL 15+)
   **And** health checks ensure DB is ready before tests run

7. **Given** the need to monitor CI/CD results
   **When** verifying workflow execution
   **Then** developers MUST use `gh` CLI commands:
   - `gh run list` - list recent workflow runs
   - `gh run watch` - watch running workflow in real-time
   - `gh run view <run-id> --log` - view workflow logs
   - `gh run view --log-failed` - view failed job logs
   - `gh run rerun --failed` - re-run failed jobs
   **And** `gh pr checks` to verify PR status checks

## Tasks / Subtasks

- [x] Task 1: Create `.github/workflows/ci.yml` base structure (AC: #1, #2)
  - [x] Create `.github/workflows/` directory
  - [x] Define workflow name and trigger events (push, pull_request)
  - [x] Configure branches filter (main and feature branches)
  - [x] Set workflow-level environment variables

- [x] Task 2: Implement `build` job (AC: #3)
  - [x] Set up Go environment with actions/setup-go
  - [x] Cache Go modules for faster builds
  - [x] Run `go build ./cmd/api` to verify compilation
  - [x] Optionally build Docker image (validate Dockerfile)

- [x] Task 3: Implement `test` job with PostgreSQL service (AC: #4, #5, #6)
  - [x] Configure PostgreSQL 15 service container
  - [x] Set PostgreSQL health check options
  - [x] Configure environment variables for test database connection
  - [x] Run unit tests: `go test -v -race ./internal/...`
  - [x] Run integration tests: `go test -v -race ./tests/integration/...`
  - [x] Run stress tests: `go test -v -race -count=1 ./tests/stress/...`
  - [x] Generate coverage report: `go test -race -coverprofile=coverage.out ./...`
  - [x] Upload coverage report as artifact

- [x] Task 4: Implement `lint` job (AC: #3)
  - [x] Set up Go environment
  - [x] Install and run golangci-lint
  - [x] Run `go vet ./...`
  - [x] Fail job on any lint errors

- [x] Task 5: Implement `security` job (AC: #3)
  - [x] Set up Go environment
  - [x] Install and run gosec
  - [x] Install and run govulncheck
  - [x] Fail job on high/critical findings

- [x] Task 6: Verify workflow execution (AC: #7)
  - [x] Push changes to trigger workflow
  - [x] Use `gh run watch` to monitor execution
  - [x] Verify all jobs pass
  - [x] Document any issues encountered

## Dev Notes

### Critical Implementation Details

**MANDATORY - Follow these exact specifications from Architecture document:**

1. **PostgreSQL Version**: Must be 15+ (same as docker-compose.yml which uses `postgres:15-alpine`)
2. **Go Version**: Must be 1.21+ (go.mod shows 1.24.0)
3. **Test Commands**: Must use `-race` flag as per project requirements
4. **Coverage Target**: >= 80% (from NFR11)

### GitHub Actions Service Container Configuration

The PostgreSQL service container MUST match the docker-compose.yml configuration:

```yaml
services:
  postgres:
    image: postgres:15
    env:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: coupon_db
    ports:
      - 5432:5432
    options: >-
      --health-cmd pg_isready
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
```

### Environment Variables for Tests

Tests require these environment variables (from docker-compose.yml):

```yaml
DB_HOST: localhost  # Not 'postgres' - service runs on localhost in CI
DB_PORT: 5432
DB_USER: postgres
DB_PASSWORD: postgres
DB_NAME: coupon_db
```

### Workflow Structure Reference

Based on Architecture and PRD requirements:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go build ./cmd/api

  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        # ... config
    steps:
      # ... test steps

  lint:
    runs-on: ubuntu-latest
    steps:
      # ... lint steps

  security:
    runs-on: ubuntu-latest
    steps:
      # ... security scan steps
```

### Test Execution Order

Tests should run in this order to ensure proper coverage:
1. Unit tests (`./internal/...`) - fastest, no external dependencies
2. Integration tests (`./tests/integration/...`) - requires PostgreSQL
3. Stress tests (`./tests/stress/...`) - requires PostgreSQL, intensive

### Stress Test Considerations for CI

From existing stress tests (tests/stress/):
- **Flash Sale**: 50 concurrent goroutines, 5 stock -> exactly 5 succeed
- **Double Dip**: 10 concurrent same-user requests -> exactly 1 succeeds
- These tests use dockertest for PostgreSQL container management
- In CI, tests will use the service container instead

### golangci-lint Configuration

Story 5.2 will create `.golangci.yml`, but the lint job should still work with defaults:

```yaml
- name: golangci-lint
  uses: golangci/golangci-lint-action@v4
  with:
    version: latest
```

### Security Scanning Tools

From Architecture requirements:
- **gosec**: Security scanner for Go code - `gosec ./...`
- **govulncheck**: Vulnerability checker - `govulncheck ./...`

Both tools should be installed via `go install`:
```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

### Existing Makefile Targets

The Makefile already has these relevant targets:
- `make test` - runs tests with coverage
- `make lint` - runs golangci-lint
- `make vet` - runs go vet
- `make all` - runs fmt, lint, vet, test

Consider using Makefile targets in CI for consistency.

### Coverage Artifact Upload

Coverage report should be uploaded as an artifact for later analysis:

```yaml
- name: Upload coverage
  uses: actions/upload-artifact@v4
  with:
    name: coverage-report
    path: coverage/coverage.out
```

### Job Dependencies

Jobs can run in parallel where possible:
- `build` - independent, fast
- `lint` - independent, fast
- `security` - independent, moderate
- `test` - independent but slowest (requires PostgreSQL service)

### Project Structure Notes

- CI workflow: `.github/workflows/ci.yml` (new file)
- No existing GitHub workflows (verified via glob)
- Makefile has relevant targets that can be reused

### Previous Story Intelligence

**From Epic 4 (Testing):**
- Unit tests in `internal/*_test.go`
- Integration tests in `tests/integration/`
- Stress tests in `tests/stress/`
- All tests use testify for assertions
- Integration/stress tests use dockertest for PostgreSQL container

**From Epics 1-3:**
- PostgreSQL 15 (from docker-compose.yml)
- Go 1.24.0 (from go.mod)
- Fiber v2.52.10 web framework
- pgx v5.8.0 database driver

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-5.1-GitHub-Actions-CI-Workflow]
- [Source: _bmad-output/planning-artifacts/architecture.md#CI/CD-Pipeline]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing-Strategy]
- [Source: docs/project-context.md#Quality-Gates]
- [Source: docs/project-context.md#CI/CD-Monitoring]
- [Source: docker-compose.yml - PostgreSQL configuration]
- [Source: go.mod - Go version and dependencies]
- [Source: Makefile - existing build/test targets]

### Anti-Patterns to AVOID

1. **DO NOT** use a different PostgreSQL version than docker-compose.yml (must be 15+)
2. **DO NOT** skip the `-race` flag on tests - it's mandatory per project requirements
3. **DO NOT** use DB_HOST=postgres in CI - use localhost (service container runs on localhost)
4. **DO NOT** skip stress tests - they're critical for verifying concurrency correctness
5. **DO NOT** hardcode secrets - use GitHub secrets for any sensitive values
6. **DO NOT** skip coverage reporting - it's required for quality gate in Story 5.4

### gh CLI Verification Pattern

After implementing and pushing the workflow:

```bash
# Push changes
git add .github/workflows/ci.yml
git commit -m "feat: Add GitHub Actions CI workflow"
git push origin main

# Watch workflow execution
gh run watch

# If issues, check failed logs
gh run view --log-failed

# Verify all checks pass
gh pr checks  # (if on a PR)
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- GitHub Actions Run #20897286714: All 4 jobs passed successfully

### Completion Notes List

- Created `.github/workflows/ci.yml` with comprehensive CI pipeline
- Build job: Go 1.24 setup with caching, application build, Docker image verification
- Test job: PostgreSQL 15 service container, unit/integration/stress tests with `-race` flag, coverage report artifact upload
- Lint job: golangci-lint action with go vet validation
- Security job: gosec and govulncheck security scanning
- Workflow verified via `gh run watch` - all jobs passed:
  - Build: 1m7s ✅
  - Lint: 59s ✅
  - Security: 1m15s ✅
  - Test: 1m31s ✅ (including stress tests)

### File List

- `.github/workflows/ci.yml` (new)

### Change Log

- 2026-01-11: Created GitHub Actions CI workflow with build, test, lint, and security jobs. All jobs verified passing in GitHub Actions run #20897286714.
- 2026-01-11: [Code Review] Fixed 4 MEDIUM and 3 LOW issues:
  - M1: Changed PostgreSQL image from `postgres:15` to `postgres:15-alpine` to match docker-compose.yml
  - M4: Added coverage threshold validation (fails if < 80%)
  - L1: Added DB_SSL_MODE=disable to all test environment variables
  - L2: Pinned golangci-lint to v1.62.2 for build reproducibility
  - Note: Security enhancements (SARIF output, CodeQL upload) were already present in working copy

## Senior Developer Review (AI)

**Review Date:** 2026-01-11
**Reviewer:** Claude Opus 4.5 (Code Review Agent)
**Outcome:** APPROVED (after fixes applied)

### Issues Found and Resolved

| ID | Severity | Issue | Resolution |
|----|----------|-------|------------|
| M1 | MEDIUM | PostgreSQL image mismatch (postgres:15 vs postgres:15-alpine) | Fixed: Changed to postgres:15-alpine |
| M2 | MEDIUM | Uncommitted security enhancements | Kept: SARIF/CodeQL changes are valid improvements |
| M3 | MEDIUM | Makefile changes not in File List | N/A: Belongs to Story 5-2 |
| M4 | MEDIUM | No coverage threshold validation | Fixed: Added 80% threshold check |
| L1 | LOW | DB_SSL_MODE not configured | Fixed: Added DB_SSL_MODE=disable |
| L2 | LOW | golangci-lint version not pinned | Fixed: Pinned to v1.62.2 |
| L3 | LOW | Tests run twice | Deferred: Acceptable trade-off for clarity |

### Acceptance Criteria Validation

All 7 Acceptance Criteria verified as PASS:
- AC #1-#5: Workflow structure, triggers, jobs verified
- AC #6: PostgreSQL version now exactly matches docker-compose.yml (15-alpine)
- AC #7: gh CLI documentation complete

### Code Quality Notes

- Security job now includes SARIF output and GitHub Security integration
- Coverage threshold enforcement added per NFR11 (≥80%)
- All environment variables explicitly set for CI reproducibility
