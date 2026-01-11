# Story 2.2: Get Coupon Details Endpoint

Status: done

## Story

As an **API consumer**,
I want **to retrieve coupon details including who has claimed it**,
So that **I can monitor coupon status during flash sales**.

## Acceptance Criteria

### AC1: Retrieve Coupon with Claims
**Given** a coupon "PROMO_SUPER" exists with amount=100, remaining_amount=95
**And** users ["user_001", "user_002", "user_003", "user_004", "user_005"] have claimed it
**When** I send GET to `/api/coupons/PROMO_SUPER`
**Then** I receive 200 OK
**And** the response body is:
```json
{
  "name": "PROMO_SUPER",
  "amount": 100,
  "remaining_amount": 95,
  "claimed_by": ["user_001", "user_002", "user_003", "user_004", "user_005"]
}
```

### AC2: Retrieve Coupon with No Claims
**Given** a coupon "PROMO_SUPER" exists with no claims
**When** I send GET to `/api/coupons/PROMO_SUPER`
**Then** I receive 200 OK
**And** the `claimed_by` field is an empty array `[]`

### AC3: Coupon Not Found
**Given** no coupon named "NONEXISTENT" exists
**When** I send GET to `/api/coupons/NONEXISTENT`
**Then** I receive 404 Not Found
**And** the response body contains `{"error": "coupon not found"}`

### AC4: JSON Field Naming
**Given** the response JSON
**When** I examine the field names
**Then** all fields use snake_case: `name`, `amount`, `remaining_amount`, `claimed_by`

## Tasks / Subtasks

- [x] Task 1: Create Coupon Domain Model (AC: #1, #2, #4)
  - [x] Subtask 1.1: Create `Coupon` struct in `internal/model/coupon.go` with proper JSON tags
  - [x] Subtask 1.2: Create `CouponResponse` DTO with `claimed_by` field for API response

- [x] Task 2: Implement Repository Layer (AC: #1, #2, #3)
  - [x] Subtask 2.1: Implement `GetByName(ctx, name) (*Coupon, error)` in `CouponRepository`
  - [x] Subtask 2.2: Create `ClaimRepository` with `GetUsersByCoupon(ctx, couponName) ([]string, error)`
  - [x] Subtask 2.3: Use pgx parameterized queries (NO SQL injection)
  - [x] Subtask 2.4: Return wrapped errors for proper error handling in service layer

- [x] Task 3: Implement Service Layer (AC: #1, #2, #3)
  - [x] Subtask 3.1: Implement `GetCouponByName(ctx, name) (*CouponResponse, error)` in `CouponService`
  - [x] Subtask 3.2: Combine coupon data with claims list
  - [x] Subtask 3.3: Return `ErrCouponNotFound` when coupon doesn't exist
  - [x] Subtask 3.4: Return empty `[]string{}` (not nil) when no claims exist

- [x] Task 4: Implement Handler Layer (AC: #1, #2, #3, #4)
  - [x] Subtask 4.1: Create `CouponHandler` struct with service dependency
  - [x] Subtask 4.2: Implement `GetCoupon(c *fiber.Ctx) error` handler
  - [x] Subtask 4.3: Register route `GET /api/coupons/:name`
  - [x] Subtask 4.4: Map service errors to HTTP status codes (404 for not found)
  - [x] Subtask 4.5: Return proper JSON response with snake_case fields

- [x] Task 5: Write Unit Tests (AC: #1, #2, #3)
  - [x] Subtask 5.1: Write service layer unit tests (mock repository)
  - [x] Subtask 5.2: Write handler layer unit tests (mock service)
  - [x] Subtask 5.3: Test successful retrieval with claims
  - [x] Subtask 5.4: Test successful retrieval with empty claims
  - [x] Subtask 5.5: Test coupon not found scenario
  - [x] Subtask 5.6: Verify ≥80% coverage for new code

- [x] Task 6: Integration with Main Application
  - [x] Subtask 6.1: Wire repository, service, handler in main.go
  - [x] Subtask 6.2: Register GET /api/coupons/:name route
  - [x] Subtask 6.3: Verify endpoint works with curl

## Dev Notes

### CRITICAL: Dependency on Story 2.1

Story 2.1 (Create Coupon Endpoint) MUST be completed first because:
1. The `CouponRepository` structure and interface will be defined there
2. The `CouponService` constructor and pool dependency will be established there
3. The `CouponHandler` registration pattern will be established there
4. The model structs may be partially created there

**If Story 2.1 is not yet done:** You may need to create the full infrastructure yourself, but follow the patterns that would be established in 2.1.

### CRITICAL: Architecture Patterns

**Layer Structure (from architecture.md):**
```
Handler (Fiber) → Service (Business Logic) → Repository (pgx) → PostgreSQL
```

**File Organization:**
- `internal/model/coupon.go` - Domain models and DTOs
- `internal/repository/coupon_repository.go` - Coupon data access
- `internal/repository/claim_repository.go` - Claim data access (NEW)
- `internal/service/coupon_service.go` - Business logic
- `internal/handler/coupon_handler.go` - HTTP handlers

### CRITICAL: Model Definitions

```go
// internal/model/coupon.go

// Coupon represents the domain model for a coupon
type Coupon struct {
    Name            string    `json:"name"`
    Amount          int       `json:"amount"`
    RemainingAmount int       `json:"remaining_amount"`
    CreatedAt       time.Time `json:"-"` // Not exposed in API
}

// CouponResponse is the API response DTO for GET /api/coupons/:name
type CouponResponse struct {
    Name            string   `json:"name"`
    Amount          int      `json:"amount"`
    RemainingAmount int      `json:"remaining_amount"`
    ClaimedBy       []string `json:"claimed_by"`
}
```

### CRITICAL: Repository Implementation

```go
// internal/repository/coupon_repository.go

type CouponRepository interface {
    GetByName(ctx context.Context, name string) (*model.Coupon, error)
    // Insert, GetForUpdate, DecrementStock will be in Story 2.1 and 3.x
}

type couponRepository struct {
    pool *pgxpool.Pool
}

func NewCouponRepository(pool *pgxpool.Pool) CouponRepository {
    return &couponRepository{pool: pool}
}

func (r *couponRepository) GetByName(ctx context.Context, name string) (*model.Coupon, error) {
    query := `SELECT name, amount, remaining_amount, created_at
              FROM coupons WHERE name = $1`

    var coupon model.Coupon
    err := r.pool.QueryRow(ctx, query, name).Scan(
        &coupon.Name,
        &coupon.Amount,
        &coupon.RemainingAmount,
        &coupon.CreatedAt,
    )
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, nil // Not found - let service handle
        }
        return nil, fmt.Errorf("get coupon by name %s: %w", name, err)
    }
    return &coupon, nil
}
```

```go
// internal/repository/claim_repository.go

type ClaimRepository interface {
    GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error)
    // Insert will be in Story 3.x
}

type claimRepository struct {
    pool *pgxpool.Pool
}

func NewClaimRepository(pool *pgxpool.Pool) ClaimRepository {
    return &claimRepository{pool: pool}
}

func (r *claimRepository) GetUsersByCoupon(ctx context.Context, couponName string) ([]string, error) {
    query := `SELECT user_id FROM claims WHERE coupon_name = $1 ORDER BY created_at`

    rows, err := r.pool.Query(ctx, query, couponName)
    if err != nil {
        return nil, fmt.Errorf("get claims for coupon %s: %w", couponName, err)
    }
    defer rows.Close()

    var users []string
    for rows.Next() {
        var userID string
        if err := rows.Scan(&userID); err != nil {
            return nil, fmt.Errorf("scan claim user_id: %w", err)
        }
        users = append(users, userID)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate claims rows: %w", err)
    }

    // Return empty slice, not nil
    if users == nil {
        users = []string{}
    }

    return users, nil
}
```

### CRITICAL: Service Implementation

```go
// internal/service/coupon_service.go

var ErrCouponNotFound = errors.New("coupon not found")

type CouponService interface {
    GetByName(ctx context.Context, name string) (*model.CouponResponse, error)
    // Create will be in Story 2.1, Claim will be in Story 3.x
}

type couponService struct {
    couponRepo repository.CouponRepository
    claimRepo  repository.ClaimRepository
    pool       *pgxpool.Pool
}

func NewCouponService(
    pool *pgxpool.Pool,
    couponRepo repository.CouponRepository,
    claimRepo repository.ClaimRepository,
) CouponService {
    return &couponService{
        pool:       pool,
        couponRepo: couponRepo,
        claimRepo:  claimRepo,
    }
}

func (s *couponService) GetByName(ctx context.Context, name string) (*model.CouponResponse, error) {
    coupon, err := s.couponRepo.GetByName(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("get coupon: %w", err)
    }
    if coupon == nil {
        return nil, ErrCouponNotFound
    }

    claimedBy, err := s.claimRepo.GetUsersByCoupon(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("get claims: %w", err)
    }

    return &model.CouponResponse{
        Name:            coupon.Name,
        Amount:          coupon.Amount,
        RemainingAmount: coupon.RemainingAmount,
        ClaimedBy:       claimedBy,
    }, nil
}
```

### CRITICAL: Handler Implementation

```go
// internal/handler/coupon_handler.go

type CouponHandler struct {
    service service.CouponService
}

func NewCouponHandler(svc service.CouponService) *CouponHandler {
    return &CouponHandler{service: svc}
}

func (h *CouponHandler) GetCoupon(c *fiber.Ctx) error {
    name := c.Params("name")
    if name == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request: name is required",
        })
    }

    coupon, err := h.service.GetByName(c.Context(), name)
    if err != nil {
        if errors.Is(err, service.ErrCouponNotFound) {
            return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
                "error": "coupon not found",
            })
        }
        log.Error().Err(err).Str("coupon_name", name).Msg("failed to get coupon")
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "internal server error",
        })
    }

    return c.JSON(coupon)
}
```

### CRITICAL: Route Registration

In `cmd/api/main.go`, add route registration:
```go
// Create repositories
couponRepo := repository.NewCouponRepository(pool)
claimRepo := repository.NewClaimRepository(pool)

// Create service
couponService := service.NewCouponService(pool, couponRepo, claimRepo)

// Create handler
couponHandler := handler.NewCouponHandler(couponService)

// Register routes
api := app.Group("/api")
api.Get("/coupons/:name", couponHandler.GetCoupon)
```

### CRITICAL: Error Handling Pattern

```go
// Repository layer - wrap errors with context
return nil, fmt.Errorf("get coupon by name %s: %w", name, err)

// Service layer - translate to domain errors
if coupon == nil {
    return nil, ErrCouponNotFound
}

// Handler layer - map to HTTP status codes
if errors.Is(err, service.ErrCouponNotFound) {
    return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
        "error": "coupon not found",
    })
}
```

### CRITICAL: Logging Pattern (zerolog)

```go
// Successful operation
log.Info().
    Str("coupon_name", name).
    Int("remaining_amount", coupon.RemainingAmount).
    Int("claims_count", len(coupon.ClaimedBy)).
    Msg("coupon retrieved")

// Error (handler level only)
log.Error().Err(err).Str("coupon_name", name).Msg("failed to get coupon")
```

### CRITICAL: JSON Response Format

**MANDATORY:** Use `snake_case` for ALL JSON fields:
- `name` - NOT `Name`
- `amount` - NOT `Amount`
- `remaining_amount` - NOT `remainingAmount` or `RemainingAmount`
- `claimed_by` - NOT `claimedBy` or `ClaimedBy`

Use struct tags:
```go
type CouponResponse struct {
    Name            string   `json:"name"`
    Amount          int      `json:"amount"`
    RemainingAmount int      `json:"remaining_amount"`
    ClaimedBy       []string `json:"claimed_by"`
}
```

### CRITICAL: Empty Array vs Null

When there are no claims, return an empty array `[]`, NOT `null`:
```json
{
  "name": "NEW_PROMO",
  "amount": 100,
  "remaining_amount": 100,
  "claimed_by": []
}
```

Ensure the slice is initialized:
```go
if users == nil {
    users = []string{}
}
```

### Project Structure Notes

**Files to CREATE or MODIFY:**
- `internal/model/coupon.go` - Add Coupon and CouponResponse structs
- `internal/repository/coupon_repository.go` - Implement GetByName method
- `internal/repository/claim_repository.go` - NEW file with GetUsersByCoupon
- `internal/service/coupon_service.go` - Implement GetByName method
- `internal/service/errors.go` - NEW file with domain errors (ErrCouponNotFound)
- `internal/handler/coupon_handler.go` - Implement GetCoupon handler
- `cmd/api/main.go` - Wire dependencies and register route

**Files to NOT MODIFY:**
- `scripts/init.sql` - Schema already correct
- `internal/handler/health_handler.go` - Health check unchanged
- `docker-compose.yml` - Infrastructure unchanged

### Testing Requirements

**Unit Tests (co-located):**
- `internal/service/coupon_service_test.go` - Mock repository, test GetByName
- `internal/handler/coupon_handler_test.go` - Mock service, test GetCoupon

**Test Scenarios:**
1. Get coupon with claims → 200 OK with claimed_by populated
2. Get coupon without claims → 200 OK with claimed_by as empty array
3. Get non-existent coupon → 404 Not Found
4. Verify JSON field names are snake_case

**Coverage Target:** ≥80% for new code

### Previous Story Learnings (from Epic 1 Retrospective)

1. **Validate paths before implementation** - Ensure all import paths exist
2. **Write tests alongside code** - Not as afterthought
3. **Use interface patterns** for testability (like Pinger pattern in health handler)
4. **Adversarial code review** catches issues - expect review feedback

### Web Research Not Required

This story uses standard pgx patterns and Fiber routing. No external API integrations or version-specific library features require research.

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture] - Database schema
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns] - Error handling, logging
- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.2] - Acceptance criteria
- [Source: docs/project-context.md#Architecture Rules] - Layer structure, transaction patterns
- [Source: docs/project-context.md#API Response Patterns] - JSON format
- [Source: scripts/init.sql] - Database schema (coupons, claims tables)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

No debug issues encountered during implementation.

### Completion Notes List

- Implemented GET /api/coupons/:name endpoint with full layered architecture
- Added CouponResponse DTO with snake_case JSON fields (name, amount, remaining_amount, claimed_by)
- Created ClaimRepository for retrieving users who claimed a coupon
- Extended CouponRepository with GetByName method using parameterized queries
- Extended CouponService with GetByName method combining coupon data with claims
- Added ErrCouponNotFound domain error for proper error handling
- Implemented GetCoupon handler with proper HTTP status code mapping (200 OK, 404 Not Found, 500 Internal Server Error)
- Returns empty array [] (not null) when no claims exist
- All acceptance criteria verified through unit and integration tests
- Test coverage: service 100%, handler 86%, repository 94.3%
- All tests pass with race detection enabled

### Change Log

- 2026-01-11: Implemented Story 2.2 - Get Coupon Details Endpoint
- 2026-01-11: Code Review - Fixed 4 issues (1 HIGH, 3 MEDIUM), handler coverage improved to 87%

### Senior Developer Review (AI)

**Review Date:** 2026-01-11
**Reviewer:** Claude Opus 4.5 (code-review workflow)
**Outcome:** APPROVED (after fixes)

**Issues Found & Fixed:**
1. [HIGH] Added missing empty name validation in GetCoupon handler (`internal/handler/coupon_handler.go:97-101`)
2. [MEDIUM] Added success logging for coupon retrieval per project-context.md standards (`internal/handler/coupon_handler.go:116-120`)
3. [MEDIUM] Added unit test for empty coupon name parameter (`internal/handler/coupon_handler_test.go:378-398`)
4. [MEDIUM] Clarified ClaimRepository documentation for error vs success return values (`internal/repository/claim_repository.go:32-34`)

**Remaining LOW Issues (deferred):**
- L1: Inconsistent interface naming (ClaimPoolInterface vs PoolInterface) - cosmetic
- L2: No model package tests - DTOs are simple, low risk

**Test Coverage After Review:**
- handler: 87% (up from 86%)
- service: 100%
- repository: 94.3%
- config: 80%

**All Acceptance Criteria Verified:** AC1, AC2, AC3, AC4 ✓

### File List

- internal/model/coupon.go (modified: added CouponResponse DTO, updated CreatedAt tag)
- internal/repository/coupon_repository.go (modified: added GetByName method, QueryRow to PoolInterface)
- internal/repository/claim_repository.go (new: ClaimRepository with GetUsersByCoupon; review: clarified docs)
- internal/repository/claim_repository_test.go (new: unit tests for ClaimRepository)
- internal/repository/coupon_repository_test.go (modified: added GetByName tests, updated mock)
- internal/service/errors.go (modified: added ErrCouponNotFound)
- internal/service/coupon_service.go (modified: added ClaimRepositoryInterface, GetByName method)
- internal/service/coupon_service_test.go (modified: added GetByName tests, updated mocks)
- internal/handler/coupon_handler.go (modified: added GetCoupon handler, extended interface; review: added empty name validation, success logging)
- internal/handler/coupon_handler_test.go (modified: added GetCoupon tests; review: added empty name test)
- cmd/api/main.go (modified: wired ClaimRepository, registered GET route)
- tests/integration/coupon_integration_test.go (modified: added GetCoupon integration tests)
