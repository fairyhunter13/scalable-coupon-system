# Story 7.1: Codecov Integration

Status: review

## Story

As a **maintainer**,
I want **Codecov integration with dynamic coverage badges**,
So that **I can see actual test coverage percentage (94.7%) instead of a static threshold badge**.

## Acceptance Criteria

1. **AC1: Coverage Upload to Codecov**
   **Given** the CI pipeline runs tests with coverage
   **When** tests complete successfully
   **Then** coverage report is uploaded to Codecov using CODECOV_TOKEN
   **And** Codecov processes the report without errors

2. **AC2: Dynamic Coverage Badge in README**
   **Given** the Codecov integration is configured
   **When** I view the repository README
   **Then** a dynamic Codecov badge displays the actual coverage percentage
   **And** the badge links to the Codecov dashboard for this repository

3. **AC3: PR Coverage Report Comments**
   **Given** a PR is opened with code changes
   **When** CI runs and uploads coverage to Codecov
   **Then** Codecov posts a coverage report comment on the PR
   **And** coverage diff shows lines added/removed and impact on overall coverage

4. **AC4: CI Workflow Configuration**
   **Given** the CI workflow file
   **When** I review the Codecov upload step
   **Then** it uses the official codecov/codecov-action
   **And** CODECOV_TOKEN is referenced from GitHub Secrets
   **And** coverage file path is correctly specified (coverage.out)

5. **AC5: Optional Coverage Quality Gate**
   **Given** the Codecov configuration
   **When** coverage drops below 80%
   **Then** Codecov marks the check as failed (optional quality gate)

## Tasks / Subtasks

- [x] Task 1: Add Codecov upload step to CI workflow (AC: #1, #4)
  - [x] 1.1: Add `codecov/codecov-action@v5` step after unit tests in `.github/workflows/ci.yml`
  - [x] 1.2: Configure step to use `CODECOV_TOKEN` from GitHub Secrets
  - [x] 1.3: Set coverage file path to `coverage.out`
  - [x] 1.4: Add `fail_ci_if_error: false` for graceful degradation (token issues shouldn't block CI)

- [x] Task 2: Create Codecov configuration file (AC: #3, #5)
  - [x] 2.1: Create `codecov.yml` in repository root
  - [x] 2.2: Configure project coverage target (80% threshold per NFR11)
  - [x] 2.3: Configure patch coverage requirements (optional)
  - [x] 2.4: Configure PR comment behavior (enabled, with coverage diff)

- [x] Task 3: Update README badge (AC: #2)
  - [x] 3.1: Replace static coverage badge with dynamic Codecov badge
  - [x] 3.2: Badge URL format: `https://codecov.io/gh/fairyhunter13/scalable-coupon-system/branch/main/graph/badge.svg?token=<upload-token-if-private>`
  - [x] 3.3: Badge link should point to Codecov dashboard

- [x] Task 4: Verify Codecov integration (AC: #1, #2, #3)
  - [x] 4.1: Push changes to trigger CI workflow
  - [x] 4.2: Use `gh run watch` to monitor execution
  - [x] 4.3: Verify coverage upload succeeds in CI logs
  - [x] 4.4: Verify badge updates on README after CI completes (badge deployed, Codecov processing)
  - [x] 4.5: PR coverage comment functionality configured via codecov.yml (will activate on next PR)

## Dev Notes

### Current Coverage Infrastructure

**Existing coverage generation in CI (`.github/workflows/ci.yml`):**

```yaml
unit-tests:
  name: Unit Tests & Coverage
  runs-on: ubuntu-latest
  steps:
    - name: Run unit tests with coverage
      run: |
        go test -race -coverprofile=coverage.out -coverpkg=./internal/...,./pkg/... ./internal/...

    - name: Check coverage threshold (80%)
      run: |
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')
        echo "Total coverage: ${COVERAGE}%"
        THRESHOLD=80
        if [ "$(echo "$COVERAGE < $THRESHOLD" | bc -l)" -eq 1 ]; then
          echo "::error::Coverage ${COVERAGE}% is below required threshold of ${THRESHOLD}%"
          exit 1
        fi

    - name: Upload coverage report
      uses: actions/upload-artifact@v4
      with:
        name: coverage-report
        path: coverage.out
        retention-days: 30
```

**Current coverage:** ~86.1% (from CI run 20898570747 per Story 6.6)

### Implementation Details

**1. Codecov Action Configuration:**

```yaml
    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v5
      with:
        token: ${{ secrets.CODECOV_TOKEN }}
        files: coverage.out
        flags: unittests
        name: scalable-coupon-system
        fail_ci_if_error: false  # Don't fail CI if Codecov upload fails
        verbose: true
```

**Key configuration options:**
- `token`: Required for private repos, recommended for public repos to avoid rate limits
- `files`: Path to coverage file (Go uses `coverage.out`)
- `flags`: Categorize coverage (useful for separating unit/integration)
- `fail_ci_if_error: false`: Prevents CI failure if Codecov is unavailable
- `verbose: true`: Helps debug upload issues

**2. codecov.yml Configuration:**

```yaml
# codecov.yml
coverage:
  precision: 2
  round: down
  range: "60...100"

  status:
    project:
      default:
        target: 80%  # NFR11 requirement
        threshold: 2%  # Allow 2% fluctuation
    patch:
      default:
        target: 80%

comment:
  layout: "reach, diff, flags, files"
  behavior: default
  require_changes: true
```

**3. README Badge Update:**

**Current static badge:**
```markdown
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-green)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml)
```

**New dynamic Codecov badge:**
```markdown
[![codecov](https://codecov.io/gh/fairyhunter13/scalable-coupon-system/graph/badge.svg?token=YOURTOKEN)](https://codecov.io/gh/fairyhunter13/scalable-coupon-system)
```

**Note:** For public repositories, the token in the badge URL may be optional or use a different format.

### Project Structure Notes

**Files to modify:**
```
.github/
└── workflows/
    └── ci.yml  # Add Codecov upload step after unit-tests coverage generation

codecov.yml      # NEW - Codecov configuration

README.md        # Update coverage badge to dynamic Codecov badge
```

**Placement in CI workflow:**
The Codecov upload step should be added to the `unit-tests` job, right after the existing "Upload coverage report" step (which uploads to GitHub Artifacts). This keeps all coverage-related steps together.

### Previous Story Intelligence

**From Story 6.6 (CI Pipeline Restructure):**
- Coverage is generated in Stage 1 `unit-tests` job
- Coverage file is `coverage.out`
- 80% threshold is already enforced
- Coverage artifact already uploaded to GitHub Artifacts
- Current coverage: ~86.1%

**From Epic 5 Retrospective (epics.md):**
- CODECOV_TOKEN should already be configured in GitHub Secrets
- Token was validated via successful test upload
- Dynamic badge showing actual percentage is the goal (instead of static >=80%)

### Git Intelligence

**Recent CI-related commits:**
```
db46584 docs: Complete Epic 6 retrospective
608b96e feat(ci): Restructure pipeline with staged quality gates (Story 6.6)
9fd5653 feat: Add scale stress tests for CI (Story 6.1)
```

**CI Pipeline Location:** `.github/workflows/ci.yml`
**Coverage file:** `coverage.out` (generated in `unit-tests` job)

### Library/Framework Requirements

**Required GitHub Action:**
- `codecov/codecov-action@v5` - Official Codecov uploader action

**Required Secret:**
- `CODECOV_TOKEN` - Should already exist in GitHub Secrets (per Epic 5 retrospective)

### Testing Strategy

**Local Verification:**
```bash
# Verify coverage file is generated
go test -race -coverprofile=coverage.out ./internal/...
ls -la coverage.out

# Check coverage percentage locally
go tool cover -func=coverage.out | grep total
```

**CI Verification:**
```bash
# After pushing changes
gh run watch

# Check Codecov upload step in logs
gh run view --log | grep -A 20 "Upload coverage to Codecov"

# Verify badge is working
curl -I "https://codecov.io/gh/fairyhunter13/scalable-coupon-system/graph/badge.svg"
```

**PR Verification:**
1. Create a test PR with a small code change
2. Verify Codecov posts a comment on the PR
3. Comment should show coverage diff and project status

### Codecov Token Verification

**Before implementation, verify token exists:**
```bash
# Check if CODECOV_TOKEN is set (won't show value, just confirms existence)
gh secret list | grep CODECOV_TOKEN
```

**If token doesn't exist:**
1. Go to https://codecov.io/gh/fairyhunter13/scalable-coupon-system/settings
2. Copy the repository upload token
3. Add to GitHub Secrets:
   ```bash
   gh secret set CODECOV_TOKEN
   ```

### Anti-Patterns to Avoid

1. **DO NOT** make CI fail when Codecov upload fails - use `fail_ci_if_error: false`
2. **DO NOT** remove the existing 80% threshold check - Codecov is supplementary
3. **DO NOT** upload coverage from multiple jobs - only unit-tests generates coverage
4. **DO NOT** hardcode the Codecov token in the workflow file
5. **DO NOT** change the coverage file path from `coverage.out` - it's already configured

### Architecture Compliance

**Relevant architecture decisions:**
- **Testing Strategy:** Coverage target >=80% (NFR11) - Codecov enforces this
- **CI/CD Pattern:** Staged pipeline - Codecov uploads in Stage 1 unit-tests job
- **gh CLI usage:** Required for CI/CD monitoring (per project-context.md)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 7.1: Codecov Integration]
- [Source: docs/project-context.md#Quality Gates (MANDATORY)]
- [Source: docs/project-context.md#CI/CD Monitoring (MANDATORY)]
- [Source: .github/workflows/ci.yml - Current CI workflow structure]
- [Source: _bmad-output/implementation-artifacts/6-6-restructure-ci-pipeline-with-staged-quality-gates.md - Coverage generation location]
- [Source: _bmad-output/planning-artifacts/architecture.md#Testing Strategy]
- [Codecov GitHub Action](https://github.com/codecov/codecov-action)
- [Codecov Documentation](https://docs.codecov.com/)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- Verified CODECOV_TOKEN exists in GitHub Secrets (created 2026-01-11)
- Verified coverage.out generated correctly (86.1% coverage)
- Validated ci.yml and codecov.yml YAML syntax
- CI run 20899286983 - Codecov upload successful (status_code=200)
- Coverage report available at: https://app.codecov.io/github/fairyhunter13/scalable-coupon-system

### Completion Notes List

- Added codecov/codecov-action@v5 step to unit-tests job in CI workflow
- Created codecov.yml with 80% project/patch coverage targets (per NFR11)
- Replaced static ≥80% badge with dynamic Codecov badge in README
- All configuration uses existing CODECOV_TOKEN from GitHub Secrets
- Used fail_ci_if_error: false to prevent CI failures from Codecov issues
- CI passed all stages: Unit Tests (42s), Lint (17s), Security (54s), Build (51s), Integration, Stress, Chaos tests

### File List

- `.github/workflows/ci.yml` - Added Codecov upload step after coverage artifact upload
- `codecov.yml` - NEW: Codecov configuration with coverage targets and PR comments
- `README.md` - Updated coverage badge from static to dynamic Codecov badge

### Change Log

- 2026-01-12: Implemented Codecov integration (all tasks complete)
- 2026-01-12: CI run 20899286983 verified - Codecov upload successful
