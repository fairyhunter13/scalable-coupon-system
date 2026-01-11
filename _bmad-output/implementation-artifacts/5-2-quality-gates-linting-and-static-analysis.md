# Story 5.2: Quality Gates - Linting and Static Analysis

Status: done

## Story

As a **maintainer**,
I want **automated linting and static analysis**,
So that **code quality standards are enforced consistently**.

## Acceptance Criteria

1. **Given** the `lint` job in CI **When** it executes **Then** it runs golangci-lint with project configuration **And** it runs go vet on all packages **And** any errors cause the job to fail

2. **Given** the `.golangci.yml` configuration file **When** I review its settings **Then** it enables recommended linters: errcheck, gosimple, govet, ineffassign, staticcheck, unused

3. **Given** the codebase **When** I run `golangci-lint run ./...` locally **Then** zero errors are reported **And** output matches CI results

4. **Given** the codebase **When** I run `go vet ./...` locally **Then** zero issues are reported

5. **Given** a PR with linting errors **When** the CI workflow runs **Then** the `lint` job fails **And** the PR cannot be merged until fixed **And** error details are visible in the workflow logs

6. **Given** the Makefile **When** I review its targets **Then** it includes: `make lint`, `make vet`, `make check`

## Tasks / Subtasks

- [x] Task 1: Create `.golangci.yml` configuration (AC: #2)
  - [x] Subtask 1.1: Configure enabled linters (errcheck, gosimple, govet, ineffassign, staticcheck, unused)
  - [x] Subtask 1.2: Add additional recommended linters (gofmt, goimports, misspell, gocyclo)
  - [x] Subtask 1.3: Configure timeout and output settings
  - [x] Subtask 1.4: Add exclusions for test files and generated code where appropriate

- [x] Task 2: Update Makefile with check target (AC: #6)
  - [x] Subtask 2.1: Verify existing `lint` and `vet` targets work correctly
  - [x] Subtask 2.2: Add `make check` target that runs lint + vet + security checks

- [x] Task 3: Fix any existing linting issues (AC: #3, #4)
  - [x] Subtask 3.1: Run golangci-lint and identify all issues
  - [x] Subtask 3.2: Fix each linting error in the codebase
  - [x] Subtask 3.3: Verify zero errors after fixes

- [x] Task 4: Create GitHub Actions lint job (AC: #1, #5)
  - [x] Subtask 4.1: Create `.github/workflows/ci.yml` with lint job (or update if exists)
  - [x] Subtask 4.2: Configure golangci-lint-action with proper version
  - [x] Subtask 4.3: Add go vet step to workflow
  - [x] Subtask 4.4: Ensure job fails on any lint errors

- [x] Task 5: Verify CI integration (AC: #5)
  - [x] Subtask 5.1: Push changes and trigger CI workflow
  - [x] Subtask 5.2: Use `gh run watch` to monitor workflow execution
  - [x] Subtask 5.3: Verify lint job passes with current code
  - [x] Subtask 5.4: Test failure scenario by introducing temporary lint error

## Dev Notes

### Architecture Compliance

This story implements FR34 (Pipeline runs linting and static analysis) and NFR12 (Zero errors from golangci-lint) and NFR13 (Zero issues from go vet static analysis).

**Key Architecture Requirements:**
- golangci-lint is the mandated linter per Architecture doc
- Must integrate with GitHub Actions CI pipeline
- Quality gates must be enforced before merging

### Technical Requirements

**golangci-lint Configuration Must Include:**

```yaml
# .golangci.yml structure reference
linters:
  enable:
    - errcheck      # Unchecked errors
    - gosimple      # Simplifications
    - govet         # Suspicious constructs (required by NFR13)
    - ineffassign   # Ineffectual assignments
    - staticcheck   # Static analysis
    - unused        # Unused code
    - gofmt         # Code formatting
    - goimports     # Import organization
    - misspell      # Spelling errors
    - gocyclo       # Cyclomatic complexity
```

**GitHub Actions Integration:**

The lint job uses `golangci/golangci-lint-action@v7` with pinned version for reproducibility:

```yaml
- name: Run golangci-lint
  uses: golangci/golangci-lint-action@v7
  with:
    version: v2.5.0
    args: --timeout=5m
```

### Library/Framework Requirements

| Tool | Version | Purpose |
|------|---------|---------|
| golangci-lint | v2.5.0 (pinned) | Meta-linter for Go |
| go vet | Go 1.21+ built-in | Static analysis |
| golangci-lint-action | v7 | GitHub Actions integration |

**Installation (local development):**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### File Structure Requirements

Files to create/modify:
```
/
├── .golangci.yml                    # NEW - Linter configuration
├── .github/
│   └── workflows/
│       └── ci.yml                   # NEW/UPDATE - CI pipeline with lint job
└── Makefile                         # UPDATE - Add check target
```

### Testing Requirements

**Local Verification:**
```bash
# Verify linting passes
make lint            # Should complete with no errors
go vet ./...         # Should complete with no issues
make check           # Should run all quality checks

# Verify golangci-lint configuration is valid
golangci-lint config path  # Should show .golangci.yml path
golangci-lint linters      # Should show enabled linters
```

**CI Verification:**
```bash
# After pushing changes
gh run watch                    # Watch workflow execution
gh run view --log-failed       # Check failures if any
gh pr checks                   # Verify PR status checks
```

### Project Structure Notes

- Configuration file `.golangci.yml` goes in project root
- CI workflow goes in `.github/workflows/ci.yml`
- Existing Makefile already has `lint` and `vet` targets - add `check` target

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Development Tools] - golangci-lint specified
- [Source: _bmad-output/planning-artifacts/architecture.md#Quality Gates] - NFR12, NFR13 requirements
- [Source: _bmad-output/planning-artifacts/epics.md#Story 5.2] - Full acceptance criteria
- [Source: docs/project-context.md#Quality Gates] - Mandatory quality checks

### Previous Story Intelligence

**Story 5-1 (GitHub Actions CI Workflow):**
- Story 5-1 is still in backlog - this story may need to create the initial `.github/workflows/ci.yml` if not created by 5-1 first
- The CI workflow structure should follow this pattern:
  - `build` job: Builds Docker image
  - `test` job: Runs unit, integration, stress tests
  - `lint` job: Runs linting and static analysis (THIS STORY)
  - `security` job: Runs security scanning (Story 5-3)
- Jobs can run in parallel where possible (lint + security)

**Existing Makefile targets:**
- `make lint` - Already exists, runs golangci-lint
- `make vet` - Already exists, runs go vet
- `make test` - Already exists, runs tests with coverage
- Need to add: `make check` for combined quality checks

### Git Intelligence Summary

Recent commits show Epic 4 completion with comprehensive testing infrastructure:
- `5f355c2` - Story 5.5 added for README status badges
- `c87a256` - Epic 4 retrospective completed
- `3efc17b` - Epics 1-4 implementation complete

**Relevant patterns from previous work:**
- Tests use dockertest for PostgreSQL container lifecycle
- Coverage reports generated to `coverage/` directory
- Race detection enabled in test commands

### Latest Tech Information

**golangci-lint v2.5.0 (2026 stable):**
- Uses Go 1.21+ as minimum version
- `golangci-lint-action@v7` for GitHub Actions (improved caching)
- Default timeout increased to 5m for large projects
- v2 config format: gosimple merged into staticcheck

**GitHub Actions Best Practices (2025):**
- Use composite actions for reusable steps
- Cache Go modules and golangci-lint cache separately
- Set `GOLANGCI_LINT_SKIP_LINTERS=true` for faster initial runs during testing

### Project Context Reference

**Critical Rules from project-context.md:**
- All quality gates MUST pass: `golangci-lint run ./...`, `go vet ./...`
- Use `gh` CLI for CI/CD monitoring (NOT just GitHub web UI)
- Go 1.21+ required (check go.mod for exact version)
- Fiber v2 framework (fasthttp-based, not net/http)

### Dependency Note

This story depends on:
- Story 5-1 (GitHub Actions CI Workflow) for the base CI pipeline structure
- If 5-1 is not implemented first, this story should create the initial `.github/workflows/ci.yml`

This story is a prerequisite for:
- Story 5-3 (Security Scanning) - will add security job to CI
- Story 5-4 (CI Pipeline Integration) - will verify all jobs work together

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- Initially created .golangci.yml with v1 format, encountered version compatibility issues
- Updated to v2 format for golangci-lint 2.5.0 compatibility
- Fixed config syntax issues: output.formats, linters.default, formatters section
- gosimple linter merged into staticcheck in v2
- Fixed misspelling "cancelled" -> "canceled" in stress test comments
- Fixed gofmt spacing issue in mock struct methods
- [Code Review] Re-enabled test file linting with targeted exclusions for errcheck, bodyclose, nilerr

### Completion Notes List

- Created `.golangci.yml` with v2 format configuration for golangci-lint 2.5.0
- Enabled core linters: errcheck, govet, ineffassign, staticcheck, unused
- Enabled additional linters: misspell, gocyclo, bodyclose, nilerr, unconvert
- Enabled formatters: gofmt, goimports
- Disabled ST1000 (package comments) as not required for this project
- Disabled fieldalignment (optimization hints are noise)
- Updated GitHub Actions lint job to use golangci-lint-action@v7 with pinned v2.5.0
- Fixed spelling error in tests/stress/double_dip_test.go
- Makefile already contains `lint`, `vet`, and `check` targets
- Local verification: `golangci-lint run ./...` returns 0 issues
- Local verification: `go vet ./...` returns 0 issues
- All unit tests pass with race detection

### Code Review Fixes Applied

- **MED-1 Fixed**: Enabled test file linting with targeted exclusions (errcheck, bodyclose, nilerr)
- **MED-2 Fixed**: Added documentation that gosimple is included in staticcheck (v2 format)
- **MED-3 Fixed**: Added missing `defer resp.Body.Close()` to 9 handler test functions
- **MED-5 Fixed**: Pinned golangci-lint version to v2.5.0 (was `latest`)
- **AC#2 Note**: gosimple linter is enabled via staticcheck in golangci-lint v2

### File List

- .golangci.yml (NEW)
- .github/workflows/ci.yml (MODIFIED - updated golangci-lint-action version)
- tests/stress/double_dip_test.go (MODIFIED - fixed misspelling)
- internal/repository/claim_repository_test.go (MODIFIED - fixed gofmt spacing)
- internal/handler/claim_handler_test.go (MODIFIED - added defer resp.Body.Close())

### Change Log

- 2026-01-11: Implemented Story 5.2 - Quality Gates with golangci-lint v2 configuration, zero linting errors achieved
- 2026-01-11: Code Review completed - Fixed 5 issues (1 HIGH, 4 MEDIUM), pinned CI versions, improved test file handling

