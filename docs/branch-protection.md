# Branch Protection Configuration

This document describes how to configure branch protection rules for the `main` branch to enforce CI quality gates before merging.

## Required Status Checks

The following CI jobs must pass before any PR can be merged to `main`:

| Job | Purpose |
|-----|---------|
| `Build` | Verify code compiles and Docker image builds |
| `Test` | Run unit, integration, and stress tests with race detection |
| `Lint` | Run golangci-lint and go vet static analysis |
| `Security` | Run gosec and govulncheck security scanning |

## Configuration via GitHub UI

1. Navigate to your repository on GitHub
2. Go to **Settings** > **Branches**
3. Click **Add branch protection rule**
4. Configure the following:

   - **Branch name pattern**: `main`
   - **Protect matching branches**:
     - [x] Require a pull request before merging
     - [x] Require status checks to pass before merging
       - [x] Require branches to be up to date before merging
       - Add required status checks:
         - `Build`
         - `Test`
         - `Lint`
         - `Security`
     - [ ] Require conversation resolution before merging (optional)
     - [ ] Require signed commits (optional)
     - [x] Do not allow bypassing the above settings

5. Click **Create** to save the rule

## Configuration via GitHub CLI (`gh`)

You can also configure branch protection using the `gh` CLI:

```bash
# Enable branch protection with required status checks
gh api \
  --method PUT \
  /repos/{owner}/{repo}/branches/main/protection \
  -f "required_status_checks[strict]=true" \
  -f "required_status_checks[contexts][]=Build" \
  -f "required_status_checks[contexts][]=Test" \
  -f "required_status_checks[contexts][]=Lint" \
  -f "required_status_checks[contexts][]=Security" \
  -f "enforce_admins=false" \
  -f "required_pull_request_reviews=null" \
  -f "restrictions=null"
```

**Note**: Replace `{owner}/{repo}` with your actual repository path (e.g., `fairyhunter13/scalable-coupon-system`).

### View Current Protection Rules

```bash
gh api /repos/{owner}/{repo}/branches/main/protection
```

### Remove Protection Rules (if needed)

```bash
gh api --method DELETE /repos/{owner}/{repo}/branches/main/protection
```

## Verification

After configuring branch protection:

1. Create a test branch and PR
2. Verify CI workflow runs on the PR
3. Attempt to merge before CI completes - should be blocked
4. Wait for all checks to pass
5. Verify merge is now allowed

### Using gh CLI to Check PR Status

```bash
# View PR status checks
gh pr checks

# View specific PR
gh pr view <pr-number>

# Watch workflow execution
gh run watch
```

## Quality Gates Enforced

When branch protection is configured, these quality gates must pass:

- **Build**: Code compiles, Docker image builds
- **Test**: All tests pass (unit, integration, stress) with >= 80% coverage
- **Lint**: Zero golangci-lint errors, zero go vet issues
- **Security**: Zero gosec high/critical findings, zero govulncheck vulnerabilities
- **Race Detection**: Zero race conditions detected

## Troubleshooting

### PR Blocked but CI Passed

1. Ensure all required status checks are listed correctly
2. Check if "Require branches to be up to date" is enabled (may need rebase)
3. Verify the check names match exactly (case-sensitive)

### Status Checks Not Appearing

1. Push a commit to trigger the CI workflow
2. Wait for the first run to complete
3. Status checks should then appear in the branch protection settings

## References

- [GitHub Docs: Protected Branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [GitHub CLI: gh api](https://cli.github.com/manual/gh_api)
