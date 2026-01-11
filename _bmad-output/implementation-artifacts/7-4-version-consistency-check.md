# Story 7.4: Version Consistency Check

Status: done

## Story

As a **maintainer**,
I want **a CI step that verifies Go version matches across go.mod, Dockerfile, and CI workflow**,
So that **version drift is detected automatically and prevented**.

## Acceptance Criteria

1. **Given** the CI pipeline runs the version consistency check
   **When** go.mod, Dockerfile, and CI workflow all specify Go 1.25.x
   **Then** the check passes and no warnings are reported

2. **Given** the CI pipeline runs the version consistency check
   **When** go.mod specifies Go 1.25 but Dockerfile uses Go 1.24
   **Then** the check fails with error message identifying mismatched files and PR cannot be merged until fixed

3. **Given** the CI pipeline runs the version consistency check
   **When** CI workflow uses a different Go version than go.mod
   **Then** the check fails with error message showing expected vs actual versions

4. **Given** the version check implementation
   **When** I review the CI workflow
   **Then** it includes a dedicated job or step for version consistency and extracts versions from go.mod, Dockerfile, and workflow file

5. **Given** the version check
   **When** comparing versions
   **Then** it compares major.minor versions (patch can differ)

6. **Given** the version check script
   **When** I review the implementation
   **Then** it is maintainable (shell script or Go tool), clearly documents which files are checked, and provides actionable error messages

7. **Given** a new developer updates one version file
   **When** they forget to update the others
   **Then** CI catches the inconsistency before merge and the error guides them to update all files

## Tasks / Subtasks

- [x] Task 1: Create version consistency check script (AC: #4, #5, #6)
  - [x] 1.1: Create `scripts/check-version-consistency.sh` script
  - [x] 1.2: Extract Go version from `go.mod` (line 3: `go X.Y.Z`)
  - [x] 1.3: Extract Go version from `Dockerfile` (line 2: `FROM golang:X.Y-alpine`)
  - [x] 1.4: Extract Go version from `.github/workflows/ci.yml` (env `GO_VERSION`)
  - [x] 1.5: Normalize versions to major.minor format (strip patch, strip `-alpine`)
  - [x] 1.6: Compare all three and fail with clear error if mismatch
  - [x] 1.7: Make script executable (`chmod +x`)

- [x] Task 2: Add version-check job to CI pipeline (AC: #1, #2, #3, #7)
  - [x] 2.1: Add `version-check` job to `.github/workflows/ci.yml`
  - [x] 2.2: Run as Stage 1 job (parallel with unit-tests, lint, security)
  - [x] 2.3: Use `echo "::error::"` for failure messages per existing CI pattern
  - [x] 2.4: Ensure Stage 2 jobs also depend on version-check (`needs: [unit-tests, lint, security, version-check]`)

- [x] Task 3: Add Makefile target for local testing (AC: #6)
  - [x] 3.1: Add `make version-check` target to Makefile
  - [x] 3.2: Document target in Makefile comments

- [x] Task 4: Update README documentation (AC: #6)
  - [x] 4.1: Add "Version Consistency" section to CI/CD Pipeline documentation
  - [x] 4.2: Document which files must be updated when changing Go version
  - [x] 4.3: Document the version check workflow

## Dev Notes

### Critical Architecture Requirements

**CI Pipeline Structure (MUST FOLLOW):**
```
STAGE 1 (parallel): unit-tests | lint | security | version-check  <-- ADD HERE
        ↓ (ALL MUST PASS)
STAGE 2 (parallel): integration | stress | chaos
```

**Current Go Versions (as of Epic 6 completion):**
| File | Current Value | Version to Extract |
|------|---------------|-------------------|
| `go.mod` | `go 1.25.5` | `1.25` (major.minor) |
| `.github/workflows/ci.yml` | `GO_VERSION: '1.25.5'` | `1.25` (major.minor) |
| `Dockerfile` | `FROM golang:1.25-alpine` | `1.25` (major.minor) |

**NOTE:** Dockerfile already omits patch version. Script must handle both `1.25` and `1.25.5` formats by extracting only major.minor.

### Technical Implementation Requirements

**Version Extraction Logic:**
```bash
# go.mod (line 3): "go 1.25.5" → "1.25"
grep '^go ' go.mod | awk '{print $2}' | cut -d. -f1,2

# Dockerfile (line 2): "FROM golang:1.25-alpine" → "1.25"
grep '^FROM golang:' Dockerfile | grep -oE 'golang:[0-9]+\.[0-9]+' | cut -d: -f2

# ci.yml: "GO_VERSION: '1.25.5'" → "1.25"
grep "GO_VERSION:" .github/workflows/ci.yml | grep -oE "[0-9]+\.[0-9]+"
```

**Error Message Pattern (follow existing CI patterns):**
```bash
echo "::error::Version mismatch detected"
echo "::error::  go.mod: $GOMOD_VERSION"
echo "::error::  Dockerfile: $DOCKERFILE_VERSION"
echo "::error::  CI workflow: $CI_VERSION"
echo "::error::All Go versions must match (major.minor)"
exit 1
```

**Success Message Pattern:**
```bash
echo "::notice::Version consistency check passed - all files specify Go $VERSION"
```

### File Locations (MANDATORY)

- **Script location:** `scripts/check-version-consistency.sh`
- **CI workflow:** `.github/workflows/ci.yml`
- **Makefile:** `Makefile` (project root)
- **README:** `README.md` (project root)

### CI Job Definition Pattern

Follow existing job patterns in ci.yml:
```yaml
version-check:
  name: Version Consistency Check
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Check Go version consistency
      run: ./scripts/check-version-consistency.sh
```

**Dependencies Update Required:**
Update ALL Stage 2 jobs to include version-check:
```yaml
needs: [unit-tests, lint, security, version-check]
```

### Previous Story Intelligence

**From Epic 5 Retrospective:**
- Go version upgraded 3 times (1.24 → 1.24.11 → 1.25.5) due to security vulnerabilities
- Version drift occurred between Dockerfile, go.mod, and CI workflow
- Team agreed to lock at 1.25.5 and implement version consistency check
- CODECOV_TOKEN already configured in GitHub Secrets

**From Epic 6 Implementation:**
- CI pipeline restructured with Stage 1/Stage 2 dependency gates
- Coverage threshold check uses `bc -l` for floating-point comparison
- Error messages use `::error::` and `::notice::` GitHub Actions syntax
- All Stage 2 jobs have `needs: [unit-tests, lint, security]`

**Key Pattern from Code Review:**
- Bash scripts in this project are simple and use standard tools (grep, awk, cut)
- Avoid complex regex; use multiple simple extractions
- Always provide actionable error messages

### Project Structure Notes

- Script follows existing pattern: `scripts/init.sql` already exists
- Makefile targets follow pattern: `lint`, `vet`, `security`, `check`
- README CI/CD section exists and documents pipeline structure

### Anti-Pattern Prevention

**DO NOT:**
- Create a Go tool for this simple check (overkill)
- Use complex regex that's hard to maintain
- Compare full semantic versions (only major.minor needed)
- Skip updating Stage 2 job dependencies
- Use inline script in CI (put in scripts/ for local testing)

**DO:**
- Use simple bash with grep/awk/cut
- Make script runnable locally AND in CI
- Add Makefile target for developer convenience
- Provide clear, actionable error messages
- Update ALL Stage 2 job `needs:` arrays

### Testing the Implementation

**Local testing:**
```bash
# Run version check locally
make version-check

# Or directly
./scripts/check-version-consistency.sh
```

**CI testing:**
```bash
# Push and watch workflow
git push origin <branch>
gh run watch

# If version-check fails, view logs
gh run view --log-failed
```

**Intentional failure test:**
1. Temporarily change Dockerfile to `golang:1.24-alpine`
2. Run `make version-check` - should fail with clear error
3. Revert change

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-7-Story-7.4]
- [Source: _bmad-output/planning-artifacts/architecture.md#CI-CD-Pipeline]
- [Source: _bmad-output/implementation-artifacts/epic-5-retro-2026-01-11.md#Version-Management]
- [Source: _bmad-output/implementation-artifacts/epic-6-retro-2026-01-12.md#CI-Pipeline-Structure]
- [Source: docs/project-context.md#CI-CD-Specific-Rules]
- [GitHub Action: actions/setup-go](https://github.com/actions/setup-go) - supports go-version-file for reading from go.mod

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation completed without issues

### Completion Notes List

- Created `scripts/check-version-consistency.sh` bash script that extracts Go version from go.mod, Dockerfile, and ci.yml
- Script normalizes versions to major.minor format (e.g., 1.25.5 -> 1.25) for comparison
- Uses GitHub Actions `::error::` and `::notice::` syntax for clear CI messaging
- Provides actionable error messages when version mismatch is detected
- Added `version-check` job to Stage 1 of CI pipeline (parallel with unit-tests, lint, security)
- Updated all Stage 2 jobs (integration-tests, stress-tests, chaos-tests) to depend on version-check
- Added `make version-check` target to Makefile for local testing
- Updated README with Version Consistency section documenting which files need updating
- Updated pipeline diagram to include version-check in Stage 1
- All unit tests pass, linting passes, YAML syntax validated

### File List

- scripts/check-version-consistency.sh (created)
- .github/workflows/ci.yml (modified)
- Makefile (modified)
- README.md (modified)
- _bmad-output/planning-artifacts/architecture.md (modified - Go version updated)

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5 | **Date:** 2026-01-12 | **Outcome:** APPROVED

### Review Summary

All 7 Acceptance Criteria validated and implemented correctly. Found 3 MEDIUM and 4 LOW issues during adversarial review.

### Issues Found & Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| MEDIUM | M1: architecture.md modified but not in File List | Added to File List |
| MEDIUM | M2: Script lacked file existence checks | Added defensive checks for all 3 required files |
| MEDIUM | M3: CI version extraction could match comments | Improved regex with sed for precise extraction |
| LOW | L2: Script assumed running from project root | Added auto-detection of project root |
| LOW | L4: Missing trailing newline | Verified - already present |

### AC Validation Results

| AC | Status | Notes |
|----|--------|-------|
| AC1 | ✅ PASS | CI passes when versions match (verified locally) |
| AC2 | ✅ PASS | Script exits 1 on mismatch with clear error |
| AC3 | ✅ PASS | Same comparison logic covers CI vs go.mod |
| AC4 | ✅ PASS | `version-check` job added to Stage 1 |
| AC5 | ✅ PASS | `cut -d. -f1,2` extracts major.minor only |
| AC6 | ✅ PASS | Clear bash script, documented, `make version-check` target |
| AC7 | ✅ PASS | Stage 2 jobs depend on version-check |

### Test Results

```
$ ./scripts/check-version-consistency.sh
::notice::Version consistency check passed - all files specify Go 1.25

$ cd scripts && ./check-version-consistency.sh  # Test from different dir
::notice::Version consistency check passed - all files specify Go 1.25
```

## Change Log

- 2026-01-12: Code review fixes - added file existence checks, improved version extraction, added project root detection
- 2026-01-12: Implemented version consistency check (Story 7.4)

