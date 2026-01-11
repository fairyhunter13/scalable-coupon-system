# Scalable Coupon System

<!-- Build & CI -->
[![CI](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml/badge.svg)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml)
[![Build](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Build&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3ABuild)
[![Version Check](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Version%20Check&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3A%22Version+Consistency+Check%22)

<!-- Tests (Story 7.2 - Per-Category Badges) -->
[![Unit Tests](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Unit%20Tests&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3A%22Unit+Tests+%26+Coverage%22)
[![Integration Tests](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Integration&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3A%22Integration+Tests%22)
[![Stress Tests](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Stress&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3A%22Stress+Tests%22)
[![Chaos Tests](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Chaos&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3A%22Chaos+Tests%22)

<!-- Code Quality -->
[![Lint](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Lint&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3ALint)
[![Go Report Card](https://goreportcard.com/badge/github.com/fairyhunter13/scalable-coupon-system)](https://goreportcard.com/report/github.com/fairyhunter13/scalable-coupon-system)
[![codecov](https://codecov.io/gh/fairyhunter13/scalable-coupon-system/graph/badge.svg)](https://codecov.io/gh/fairyhunter13/scalable-coupon-system)

<!-- Security -->
[![Security](https://img.shields.io/github/actions/workflow/status/fairyhunter13/scalable-coupon-system/ci.yml?label=Security&logo=github)](https://github.com/fairyhunter13/scalable-coupon-system/actions/workflows/ci.yml?query=job%3ASecurity)

<!-- Project Info -->
[![Go Reference](https://pkg.go.dev/badge/github.com/fairyhunter13/scalable-coupon-system.svg)](https://pkg.go.dev/github.com/fairyhunter13/scalable-coupon-system)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A Flash Sale Coupon System REST API demonstrating production-grade Golang backend engineering with atomic claim processing under high concurrency.

## Prerequisites

- **Docker Desktop** (includes Docker Compose V2)
  - Minimum version: Docker 20.10+
  - Docker Compose V2 is included with Docker Desktop

No other dependencies required. The entire system runs in containers.

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/fairyhunter13/scalable-coupon-system.git
   cd scalable-coupon-system
   ```

2. Start the system:
   ```bash
   docker-compose up --build
   ```

3. Verify the API is running:
   ```bash
   curl http://localhost:3000/health
   ```
   Expected response: `{"status":"healthy"}`

## How to Run

### Starting the System

```bash
# Start with build (recommended for first run)
docker-compose up --build

# Start in detached mode
docker-compose up --build -d

# View logs
docker-compose logs -f api
```

### Startup Sequence

1. PostgreSQL container starts and runs health checks
2. API container waits for PostgreSQL to be healthy
3. API starts and connects to the database
4. Health endpoint becomes available at `http://localhost:3000/health`

### Stopping the System

```bash
# Stop services
docker-compose down

# Stop and remove volumes (clears database)
docker-compose down -v
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/coupons` | POST | Create coupon |
| `/api/coupons/{name}` | GET | Get coupon details |
| `/api/coupons/claim` | POST | Claim coupon |

### Example Requests

```bash
# Health check
curl http://localhost:3000/health

# Create coupon (Epic 2)
curl -X POST http://localhost:3000/api/coupons \
  -H "Content-Type: application/json" \
  -d '{"name": "PROMO_SUPER", "amount": 100}'

# Get coupon details (Epic 2)
curl http://localhost:3000/api/coupons/PROMO_SUPER

# Claim coupon (Epic 3)
curl -X POST http://localhost:3000/api/coupons/claim \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}'
```

## Development

### Available Make Commands

```bash
make all           # Run fmt, lint, vet, security, and test
make deps          # Download Go dependencies
make fmt           # Format code
make lint          # Run linter (golangci-lint)
make vet           # Run go vet
make security      # Run security scans (gosec + govulncheck)
make check         # Run all checks (lint + vet + security)
make version-check # Verify Go version consistency across files
make test          # Run tests with coverage
make cover         # Generate coverage HTML report
make build         # Build the application
make docker-build  # Build Docker images
make docker-run    # Start services (detached mode)
make docker-down   # Stop and remove services with volumes
```

### Secrets Management (SOPS)

```bash
make encrypt-requirements  # Encrypt project_requirements/ to secrets/
make decrypt-requirements  # Decrypt secrets/ to project_requirements/
```

See `make help` for full command reference.

## CI/CD Pipeline

The CI pipeline uses a **staged quality gates architecture** to provide fast feedback and prevent wasted CI resources.

### Pipeline Structure

```
┌──────────────────── PUSH/PR TRIGGER ────────────────────┐
│                                                          │
▼                                                          │
┌────────────────────────────────────────────────────────┐ │
│         STAGE 1: Quality Gates (Parallel, ~2-3 min)    │ │
│                                                        │ │
│ ┌────────────┐ ┌────────┐ ┌──────────┐ ┌─────────────┐│ │
│ │ unit-tests │ │  lint  │ │ security │ │version-check││ │
│ │ + 80% cov  │ │golangci│ │gosec+vuln│ │  Go vers.   ││ │
│ └────────────┘ └────────┘ └──────────┘ └─────────────┘│ │
│                                                        │ │
└────────────────────────────────────────────────────────┘ │
                           │                               │
                     ALL MUST PASS                         │
                           │                               │
                           ▼                               │
┌────────────────────────────────────────────────────────┐ │
│         STAGE 2: Database Tests (Parallel, ~5-8 min)   │ │
│                                                        │ │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────┐   │ │
│    │ integration  │  │    stress    │  │  chaos   │   │ │
│    │    tests     │  │    tests     │  │  tests   │   │ │
│    └──────────────┘  └──────────────┘  └──────────┘   │ │
│                                                        │ │
└────────────────────────────────────────────────────────┘ │
                                                          │
└──────────────────────────────────────────────────────────┘
```

### Stage 1: Quality Gates (No Database)

Fast, parallel jobs that catch issues early:

| Job | Purpose | Requirement |
|-----|---------|-------------|
| `unit-tests` | Run `./internal/...` tests with race detection | **≥80% coverage** (enforced) |
| `lint` | golangci-lint + go vet | No linting errors |
| `security` | gosec + govulncheck | No security issues |
| `version-check` | Verify Go version consistency | All files match (major.minor) |

**Why 80% Coverage?**
Per NFR11 requirements, unit test coverage must meet 80% threshold. This is enforced in Stage 1 to catch coverage drops early before running expensive database tests.

### Stage 2: Database Tests (Depends on Stage 1)

These jobs only run after **ALL** Stage 1 jobs pass:

| Job | Purpose | Tests Included |
|-----|---------|---------------|
| `integration-tests` | API endpoint tests | `./tests/integration/...` |
| `stress-tests` | Concurrency scenarios | Flash Sale, Double Dip, Scale tests |
| `chaos-tests` | Resilience and edge cases | DB Resilience, Input Boundary, Transaction Edge Cases, Mixed Load |

**Dependency:** All Stage 2 jobs have `needs: [unit-tests, lint, security, version-check]`

### Benefits of Staged Pipeline

1. **Fast Feedback**: Stage 1 jobs (~2-3 min) catch most issues before slow DB tests
2. **Resource Efficiency**: Stage 2 tests (~5-8 min each) only run when basic quality passes
3. **Clear Failure Points**: If lint fails, you know immediately without waiting for all tests
4. **Parallel Execution**: Jobs within each stage run concurrently

### Version Consistency

The `version-check` job ensures Go version consistency across all configuration files. When updating Go version, you must update **all three files**:

| File | Location | Format |
|------|----------|--------|
| `go.mod` | Line 3 | `go 1.XX.Y` |
| `Dockerfile` | Line 2 | `FROM golang:1.XX-alpine` |
| `.github/workflows/ci.yml` | `env.GO_VERSION` | `'1.XX.Y'` |

**Important:** The check compares major.minor versions only (e.g., `1.25`), so patch versions can differ between files.

**Local verification:**
```bash
# Check version consistency locally before committing
make version-check
```

If versions mismatch, CI will fail with a clear error message showing which files need to be updated.

### Local Development (requires Go 1.25+)

```bash
# Start only PostgreSQL
docker-compose up -d postgres

# Run API locally
go run cmd/api/main.go
```

## How to Test

### Test Commands

| Command | Purpose | Scope |
|---------|---------|-------|
| `go test ./...` | Run all tests | All packages |
| `go test ./internal/...` | Unit tests only | Internal packages |
| `go test ./tests/integration/...` | Integration tests | Database and API tests |
| `go test ./tests/stress/...` | Stress tests | Concurrency scenarios |
| `go test -race ./...` | Run with race detection | All packages (recommended) |
| `go test -cover ./...` | Run with coverage | All packages |
| `make test` | Via Makefile | Runs tests with coverage |

### Running Tests

```bash
# Run all tests
go test ./...

# Run unit tests only (fast, no external dependencies)
go test ./internal/...

# Run integration tests (requires Docker for PostgreSQL)
go test ./tests/integration/... -v

# Run stress tests (concurrency scenarios)
go test ./tests/stress/... -v -count=1

# Run with race detection (catches data races)
go test -race ./...

# Run with coverage report
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Requirements

- **Unit tests**: Run without external dependencies
- **Integration tests**: Require Docker (uses dockertest for PostgreSQL containers)
- **Stress tests**: Require Docker and test concurrent access patterns

## Architecture Notes

### Database Design

The system uses two separate tables following separation of concerns:

```sql
-- Coupons table: stores coupon definitions and stock
CREATE TABLE coupons (
    name VARCHAR(255) PRIMARY KEY,
    amount INTEGER NOT NULL CHECK (amount > 0),
    remaining_amount INTEGER NOT NULL CHECK (remaining_amount >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Claims table: tracks which users claimed which coupons
CREATE TABLE claims (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, coupon_name)
);

CREATE INDEX idx_claims_coupon_name ON claims(coupon_name);
```

**Design Rationale:**

| Design Choice | Purpose |
|---------------|---------|
| Two-table design | Separates coupon definition from claim tracking |
| `UNIQUE(user_id, coupon_name)` | Database-level prevention of duplicate claims |
| `idx_claims_coupon_name` index | Efficient lookup of claims per coupon |
| `remaining_amount` column | Enables atomic stock checking without counting claims |

### Locking Strategy

The system uses **SELECT FOR UPDATE** row locking to prevent race conditions:

```go
// Service layer transaction pattern
func (s *CouponService) ClaimCoupon(ctx context.Context, userID, couponName string) error {
    tx, err := s.pool.Begin(ctx)  // 1. BEGIN transaction
    defer tx.Rollback(ctx)

    // 2. SELECT ... FOR UPDATE (locks the row)
    coupon, err := s.repo.GetCouponForUpdate(ctx, tx, couponName)

    // 3. Check remaining_amount > 0
    if coupon.RemainingAmount <= 0 {
        return ErrNoStock
    }

    // 4. INSERT claim (UNIQUE constraint catches duplicates)
    err = s.claimRepo.Insert(ctx, tx, userID, couponName)

    // 5. UPDATE decrement stock
    err = s.repo.DecrementStock(ctx, tx, couponName)

    return tx.Commit(ctx)  // 6. COMMIT
}
```

**Why This Prevents Race Conditions:**

1. **Row-level locking**: `SELECT FOR UPDATE` locks the coupon row, serializing concurrent access
2. **Atomic check-and-update**: Stock check and decrement happen within the same transaction
3. **Unique constraint**: Database enforces one claim per user per coupon
4. **Read Committed isolation**: PostgreSQL default isolation with explicit locking provides correctness

**Transaction Flow:**

```
Request A ──┐                    Request B ──┐
            │                                │
      BEGIN │                          BEGIN │
            ▼                                │
   SELECT FOR UPDATE (locks row)             │ (waits)
            │                                │
   Check stock > 0                           │ (blocked)
            │                                │
   INSERT claim                              │
            │                                │
   UPDATE decrement                          │
            │                                │
      COMMIT ────────────────────────────────▼
                                   SELECT FOR UPDATE (gets lock)
                                             │
                                   Check stock (now 0 if last claim)
                                             │
                                   Return ErrNoStock (or succeed if stock remains)
```

### Stress Test Results

The stress tests validate correctness under high concurrency:

**Flash Sale Scenario:**

| Parameter | Value |
|-----------|-------|
| Coupon stock | 5 |
| Concurrent requests | 50 |
| Expected successful claims | Exactly 5 |
| Expected "out of stock" errors | Exactly 45 |
| Final remaining_amount | 0 |

```bash
# Run flash sale stress test
go test ./tests/stress/... -run TestFlashSale -v
```

**Double Dip Scenario:**

| Parameter | Value |
|-----------|-------|
| Coupon stock | 100 |
| Same user requests | 10 concurrent |
| Expected successful claims | Exactly 1 |
| Expected "already claimed" errors | Exactly 9 |
| Final remaining_amount | 99 |

```bash
# Run double dip stress test
go test ./tests/stress/... -run TestDoubleDip -v
```

**Verification:**
- Flash Sale: Tests that overselling is impossible under concurrent load
- Double Dip: Tests that duplicate claims are prevented even under concurrent attempts

## Project Structure

```
cmd/api/            # Application entrypoint
internal/
  config/           # Configuration
  handler/          # HTTP handlers
  service/          # Business logic
  repository/       # Database access
  model/            # Domain models
pkg/database/       # Database utilities
scripts/            # SQL scripts
tests/              # Integration and stress tests
```

## Documentation

- [Architecture](_bmad-output/planning-artifacts/architecture.md) - System design and technical decisions
- [Project Context](docs/project-context.md) - Coding standards and patterns

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
