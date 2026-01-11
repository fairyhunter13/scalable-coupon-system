# Project Context - Scalable Coupon System

## Project Overview

A Flash Sale Coupon System REST API in Golang demonstrating production-grade backend engineering. The system handles coupon creation, claiming, and status queries with guaranteed correctness under high-concurrency scenarios.

## Content Guidelines for AI Agents

### Source Material Restrictions

When generating documentation, code comments, or any public-facing content:

1. **Reference the gitignored source materials** at `project_requirements/` for original context
2. **Content filtering rules** are defined in `secrets/project-requirements/` (encrypted)
3. **Always frame this project** as a portfolio piece and open-source reference implementation

### Approved Project Framing

Use these descriptions:
- "A portfolio project demonstrating concurrent Go patterns"
- "A reference implementation for high-concurrency coupon systems"
- "An open-source example of production-grade Go API development"

### Content Review

Before finalizing any documentation, verify content against the rules in the encrypted secrets directory. When in doubt, keep descriptions technical and focused on the implementation.

---

## Technology Stack (MANDATORY)

| Component | Technology | Version | Rationale |
|-----------|------------|---------|-----------|
| Language | Go | 1.21+ | Required |
| Web Framework | **Fiber** | v2.52.x | Fastest Go framework, zero-allocation |
| Database | PostgreSQL | 15+ | Required |
| DB Driver | **pgx** | v5 | 30-50% faster than GORM, native PostgreSQL |
| Connection Pool | pgxpool | v5 | Built into pgx |
| Logging | **zerolog** | latest | Zero-allocation, matches Fiber philosophy |
| Config | envconfig | v1.4.0 | Simple env→struct |
| Validation | go-playground/validator | v10 | Struct tag validation |
| Testing | testify | latest | Better assertions |
| Integration Testing | dockertest | latest | Fast PostgreSQL container lifecycle |

**CRITICAL:** Do NOT substitute these libraries. The architecture was designed around their specific characteristics.

---

## Architecture Rules (MANDATORY)

### Layer Structure

```
Handler (Fiber) → Service (Business Logic) → Repository (pgx) → PostgreSQL
```

**Transaction boundaries are managed in the Service layer ONLY.**

### Concurrency Pattern

The claim operation MUST use this exact pattern:

```go
// Service layer
func (s *CouponService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    // 1. Lock the row
    coupon, err := s.repo.GetCouponForUpdate(ctx, tx, couponName)

    // 2. Check stock
    if coupon.RemainingAmount <= 0 {
        return ErrNoStock
    }

    // 3. Insert claim (UNIQUE constraint catches duplicates)
    err = s.claimRepo.Insert(ctx, tx, userID, couponName)

    // 4. Decrement stock
    err = s.repo.DecrementStock(ctx, tx, couponName)

    return tx.Commit(ctx)
}
```

### Database Schema

```sql
CREATE TABLE coupons (
    name VARCHAR(255) PRIMARY KEY,
    amount INTEGER NOT NULL CHECK (amount > 0),
    remaining_amount INTEGER NOT NULL CHECK (remaining_amount >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE claims (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, coupon_name)
);

CREATE INDEX idx_claims_coupon_name ON claims(coupon_name);
```

---

## Naming Conventions (MANDATORY)

### Database
- Tables: `snake_case`, plural (`coupons`, `claims`)
- Columns: `snake_case` (`user_id`, `coupon_name`, `remaining_amount`)
- Indexes: `idx_{table}_{column}`

### Go Code
- Packages: `lowercase` (`handler`, `service`, `repository`)
- Files: `snake_case.go` (`coupon_handler.go`, `claim_service.go`)
- Structs/Functions (exported): `PascalCase` (`CouponService`, `CreateCoupon`)
- Variables: `camelCase` (`couponName`, `userID`)

### JSON (API)
- All fields: `snake_case` (`user_id`, `coupon_name`, `remaining_amount`)

### API Endpoints
- Base: `/api/coupons`
- Pattern: `POST /api/coupons`, `GET /api/coupons/:name`, `POST /api/coupons/claim`

---

## API Response Patterns (MANDATORY)

### Success Responses
```json
// GET /api/coupons/:name - 200 OK
{
  "name": "PROMO_SUPER",
  "amount": 100,
  "remaining_amount": 95,
  "claimed_by": ["user_001", "user_002"]
}
```

### Error Responses
```json
// All errors use this simple format
{
  "error": "descriptive error message"
}
```

### HTTP Status Codes
| Scenario | Code | Message |
|----------|------|---------|
| Coupon created | 201 | (empty body or echo) |
| Claim successful | 200 | (empty body) |
| Coupon not found | 404 | `coupon not found` |
| Already claimed | 409 | `coupon already claimed by user` |
| No stock | 400 | `coupon out of stock` |
| Invalid request | 400 | `invalid request: {field} is required` |

---

## Project Structure

```
/
├── cmd/api/main.go                 # Entry point
├── internal/
│   ├── config/config.go            # envconfig struct
│   ├── handler/                    # Fiber HTTP handlers
│   ├── service/                    # Business logic + transactions
│   ├── repository/                 # pgx data access
│   └── model/                      # Domain models + DTOs
├── pkg/database/postgres.go        # pgxpool setup
├── scripts/init.sql                # Database schema
├── tests/
│   ├── integration/                # dockertest-based tests
│   └── stress/                     # Concurrency tests
├── docker-compose.yml
├── Dockerfile
└── openapi.yaml
```

---

## Testing Requirements (MANDATORY)

### Test Organization
- **Unit tests**: Co-located with source (`*_test.go` in same package)
- **Integration tests**: `tests/integration/`
- **Stress tests**: `tests/stress/`

### Critical Test Scenarios
1. **Flash Sale Attack**: 50 concurrent claims → exactly 5 succeed (if stock=5)
2. **Double Dip Attack**: 10 concurrent same-user claims → exactly 1 succeeds

### Commands
```bash
go test -race ./...                    # All tests with race detection
go test ./internal/... -v              # Unit tests
go test ./tests/integration/... -v     # Integration tests
go test ./tests/stress/... -v -count=1 # Stress tests
```

### Coverage Target: ≥80%

---

## Quality Gates (MANDATORY)

All of these MUST pass in CI:
- `golangci-lint run ./...`
- `go vet ./...`
- `gosec ./...`
- `govulncheck ./...`
- `go test -race ./...`

---

## CI/CD Monitoring (MANDATORY)

### GitHub CLI for CI/CD Monitoring

Use `gh` CLI to monitor and interact with GitHub Actions workflows. This is the **preferred method** for verifying CI/CD results during development.

**Required Commands:**

```bash
# List recent workflow runs
gh run list

# Watch a running workflow in real-time
gh run watch

# View details of a specific run
gh run view <run-id>

# View workflow run logs
gh run view <run-id> --log

# View failed job logs only
gh run view <run-id> --log-failed

# Re-run a failed workflow
gh run rerun <run-id>

# Re-run only failed jobs
gh run rerun <run-id> --failed
```

**Workflow Verification Pattern:**

After pushing changes or creating a PR, always verify CI status:

```bash
# 1. Push changes
git push origin <branch>

# 2. Watch the workflow run
gh run watch

# 3. If failed, check logs
gh run view --log-failed

# 4. Fix issues and re-run if needed
gh run rerun --failed
```

**PR Status Check:**

```bash
# View PR checks status
gh pr checks

# View specific PR with CI status
gh pr view <pr-number>
```

**CRITICAL:** Do NOT rely solely on GitHub web UI. Use `gh` CLI for faster feedback loops during development and testing of CI/CD workflows.

---

## Logging Standards

Use zerolog with these standard fields:

```go
log.Info().
    Str("request_id", requestID).
    Str("method", c.Method()).
    Str("path", c.Path()).
    Str("user_id", req.UserID).
    Str("coupon_name", req.CouponName).
    Msg("claim processed")
```

| Level | Use Case |
|-------|----------|
| Debug | SQL queries, request/response bodies (dev only) |
| Info | Request received, successful operations |
| Warn | Expected failures (no stock, duplicate claim) |
| Error | Unexpected failures, database errors |

---

## Source of Truth

- **Technical Specification**: `docs/requirements/flash-sale-coupon-system-spec.md`
- **PRD**: `_bmad-output/planning-artifacts/prd.md`
- **Architecture**: `_bmad-output/planning-artifacts/architecture.md`
- **This File**: Critical rules for AI agent consistency

---

## Anti-Patterns to AVOID

1. **DO NOT** use GORM or any ORM - use pgx directly
2. **DO NOT** manage transactions in Handler or Repository layers
3. **DO NOT** use `net/http` middleware - Fiber uses fasthttp
4. **DO NOT** use `camelCase` for JSON fields - use `snake_case`
5. **DO NOT** return error details in 500 responses - log them, return generic message
6. **DO NOT** skip race detection in tests
7. **DO NOT** embed claims in coupons table - separate tables required
