---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]
inputDocuments:
  - _bmad-output/planning-artifacts/product-brief-scalable-coupon-system-2026-01-11.md
  - docs/requirements/flash-sale-coupon-system-spec.md
documentCounts:
  briefs: 1
  research: 0
  brainstorming: 0
  projectDocs: 1
workflowType: 'prd'
lastStep: 0
---

# Product Requirements Document - scalable-coupon-system

**Author:** Hafiz
**Date:** 2026-01-11

## Executive Summary

A Flash Sale Coupon System REST API in Golang demonstrating senior-level backend engineering competency. The system handles coupon creation, claiming, and status queries with guaranteed correctness under high-concurrency scenarios.

**Core Focus:** Strict compliance with the provided API specification - no additional features, no scope creep. The objective is efficient delivery of a production-grade implementation that passes all required stress tests.

**Target Users:**
- **API Consumers:** E-commerce platforms needing reliable coupon claims under load
- **Developers:** Engineers referencing this as a pattern for concurrent Go APIs
- **Code Reviewers:** Assessing production-grade Golang implementation patterns

### What Makes This Special

1. **Pre-Validated Correctness:** CI/CD pipeline runs stress tests automatically - proof of correctness before code review
2. **Spec-Driven Development:** Exact compliance with API contract, no unnecessary extras
3. **Production Patterns:** Clean architecture, proper transactions, idiomatic Go

## Project Classification

**Technical Type:** api_backend
**Domain:** general
**Complexity:** low
**Project Context:** Greenfield - new project
**Approach:** Strict specification compliance with focus on rapid, quality delivery

## Success Criteria

### User Success

**API Consumer Success:**
- Claim coupons atomically without overselling under any load
- Receive appropriate HTTP status codes for all scenarios (201 success, 409 duplicate, 400 no stock)
- Get accurate real-time coupon status including remaining stock and claim list

**Developer Success:**
- Clone repository and run `docker-compose up --build` successfully on first try
- Understand architecture decisions through clear documentation
- Run and verify stress tests with reproducible results

### Technical Success

**Correctness Requirements (MANDATORY - from spec):**

| Test Scenario | Input | Expected Output | Validation |
|---------------|-------|-----------------|------------|
| Flash Sale Attack | 50 concurrent requests, 5 stock | Exactly 5 successful claims, 0 remaining | Automated stress test |
| Double Dip Attack | 10 concurrent requests, same user | Exactly 1 success, 9 failures | Automated stress test |
| API Compliance | All 3 endpoints | Match exact specification | Integration tests |

**Database Requirements (MANDATORY - from spec):**
- Two distinct tables: `coupons` and `claims`
- No embedding of claims in coupon records
- Unique constraint on `(user_id, coupon_name)` pair
- Atomic transaction for claim process (check eligibility → check stock → insert claim → decrement stock)

**Code Quality Metrics:**

| Metric | Target | Tool | Rationale |
|--------|--------|------|-----------|
| Unit Test Coverage | ≥80% | `go test -cover` | Industry standard for production code |
| Race Detection | Zero races | `go test -race` | Critical for concurrency correctness |
| Linting | Zero errors | `golangci-lint` | De facto standard (Kubernetes, Prometheus use it) |
| Static Analysis | Zero issues | `go vet` | Built-in Go toolchain |
| Security Scan | Zero high/critical | `gosec` | SAST for common vulnerabilities |
| Vulnerability Check | Zero known vulns | `govulncheck` | Go vulnerability database |

**Infrastructure Requirements (from spec):**

| Component | Requirement |
|-----------|-------------|
| Docker Compose | Single command deployment (`docker-compose up --build`) |
| Health Check | PostgreSQL readiness via `pg_isready` |
| Graceful Shutdown | Clean database connection handling on SIGTERM |
| Environment Config | Externalized configuration (not hardcoded) |

### Measurable Outcomes

**CI/CD Pipeline Gates:**

| Stage | Pass Criteria |
|-------|---------------|
| Build | `docker-compose up --build` exits 0 |
| Unit Tests | All pass, ≥80% coverage, zero race conditions |
| Integration Tests | All 3 API endpoints verified against spec |
| Stress Tests | Flash Sale + Double Dip attacks pass consistently |
| Quality Gates | golangci-lint, go vet, gosec, govulncheck all pass |

**Definition of Done:**
- [ ] All stress tests pass 100% of the time (not flaky)
- [ ] README enables clone-to-run in <5 minutes
- [ ] GitHub Actions shows green on all checks

## Product Scope

### MVP - Minimum Viable Product

**Endpoints (Exact Specification):**

| Endpoint | Method | Request | Response |
|----------|--------|---------|----------|
| `/api/coupons` | POST | `{"name": "X", "amount": N}` | 201 Created |
| `/api/coupons/claim` | POST | `{"user_id": "X", "coupon_name": "Y"}` | 200/201 or 409/400 |
| `/api/coupons/{name}` | GET | - | `{"name", "amount", "remaining_amount", "claimed_by"}` |

**Concurrency Implementation:**
- PostgreSQL transactions with appropriate isolation level
- `SELECT FOR UPDATE` or equivalent row locking for stock decrement
- Unique constraint enforcement for duplicate prevention

**Infrastructure:**
- Docker Compose with PostgreSQL service
- Health check ensuring DB ready before API starts
- Graceful shutdown handling
- Structured logging

**Testing:**
- Unit tests with ≥80% coverage
- Integration tests for all endpoints
- Stress tests matching spec scenarios
- Race detection enabled

**Documentation:**
- README with: Prerequisites, How to Run, How to Test, Architecture Notes

### Out of Scope (Explicit Exclusions)

| Feature | Reason |
|---------|--------|
| Authentication/Authorization | Not in spec |
| Rate Limiting | Not in spec |
| Caching (Redis) | Not in spec |
| Pagination | Not in spec |
| Bulk Operations | Not in spec |
| Admin UI | Not in spec |
| Metrics/Monitoring (beyond health) | Not in spec |
| API Versioning | Not in spec |

### Growth Features (Post-MVP, if time permits)

- Prometheus metrics endpoint
- Structured JSON logging with correlation IDs
- OpenAPI/Swagger documentation
- Additional coupon types (percentage discount, expiry dates)

### Vision (Future)

- Full authentication/authorization layer
- Rate limiting and abuse prevention
- Redis caching for read-heavy operations
- Kubernetes deployment manifests

## User Journeys

### Journey 1: FastMart E-Commerce - Flash Sale Survival

FastMart is an Indonesian e-commerce platform preparing for their biggest sale of the year - 11.11. Their previous flash sales were disasters: customers complained about claiming coupons that showed "out of stock" seconds later, and some users mysteriously received multiple coupons while others got none. Their engineering team needs a bulletproof coupon system.

After reviewing the OpenAPI specification, FastMart's backend team integrates the coupon API into their platform. On 11.11 at midnight, 50,000 users simultaneously attempt to claim the "FLASH1111" coupon (limited to 1,000 units). The system processes all requests atomically - exactly 1,000 users receive success responses, the rest receive clear 400 "out of stock" errors. Not a single oversell. Not a single duplicate claim.

When their CTO checks the coupon details endpoint the next morning, she sees exactly 1,000 entries in `claimed_by`, `remaining_amount: 0`, and knows their integration was flawless. FastMart's customer complaints drop to zero, and they're already planning to use the system for their 12.12 sale.

**Requirements Revealed:**
- POST /api/coupons - Create coupon with stock amount
- POST /api/coupons/claim - Atomic claim with proper error responses
- GET /api/coupons/{name} - Real-time status with claimed_by list
- OpenAPI spec for seamless integration
- Clear HTTP status codes (201 success, 409 duplicate, 400 no stock)

---

### Journey 2: Rina the Backend Engineer - Learning Concurrency Patterns

Rina is a mid-level Go developer at a fintech startup. Her team needs to build a high-concurrency booking system, but she's never handled race conditions at scale. She finds this repository while researching "golang concurrent transactions."

She clones the repo and runs `docker-compose up --build`. Within 3 minutes, she has a running API. She opens the README and finds clear architecture notes explaining the row-locking strategy with `SELECT FOR UPDATE`. She examines the OpenAPI spec to understand the exact API contract, then runs the stress tests herself - watching 50 goroutines fire simultaneously and seeing exactly 5 succeed.

The code is clean and readable. She traces the claim flow: check eligibility → check stock → insert claim → decrement stock, all wrapped in a transaction. She copies the pattern to her booking system, adapting it for seat reservations. Two weeks later, her system handles 10,000 concurrent booking attempts flawlessly.

**Requirements Revealed:**
- docker-compose up --build works first try
- README with architecture notes explaining locking strategy
- OpenAPI specification for API understanding
- Readable, well-structured code
- Runnable stress tests for verification
- Clear transaction flow documentation

---

### Journey 3: GitHub Actions - The Automated Gatekeeper

Every push to the repository triggers GitHub Actions. The pipeline spins up, builds the Docker containers, and waits for PostgreSQL to become healthy via `pg_isready`.

First, unit tests run with race detection enabled (`go test -race -cover`). Coverage must exceed 80%. Then integration tests verify each endpoint against the OpenAPI specification - creating coupons, claiming them, retrieving status.

The critical moment: stress tests execute. The Flash Sale Attack fires 50 concurrent requests at a 5-stock coupon. The pipeline counts: exactly 5 successes, exactly 45 failures, exactly 0 remaining stock. The Double Dip Attack fires 10 concurrent requests from the same user. Result: exactly 1 success, exactly 9 rejections.

Finally, quality gates run: golangci-lint, go vet, gosec, govulncheck. All green. The pipeline marks the commit as passing. Any deviation - 6 successful claims, a race condition detected, a security vulnerability found - and the pipeline fails immediately, protecting the main branch.

**Requirements Revealed:**
- Health check for PostgreSQL readiness
- Unit tests with race detection and coverage reporting
- Integration tests validating against OpenAPI spec
- Stress tests with exact count validation
- Quality gate tools (golangci-lint, go vet, gosec, govulncheck)
- Clear pass/fail criteria

---

### Journey 4: Code Review - Production Readiness Assessment

A senior developer discovers this repository while researching concurrent Go patterns. They open the GitHub repository and see a green CI badge and click into the Actions tab - all checks passing, including stress tests.

They read the README: prerequisites are just Docker Desktop, run command is a single line. They clone and run `docker-compose up --build` on their machine. It works. They open the OpenAPI spec and use it to quickly test endpoints with curl.

They review the code: clean architecture, proper error handling, idiomatic Go. They read the ADR explaining why `SELECT FOR UPDATE` was chosen over serializable isolation. They run the stress tests themselves and watch them pass.

The codebase demonstrates mastery of transaction isolation levels, constraint-based integrity, and CI/CD best practices - exactly the patterns they were looking for to apply in their own high-concurrency systems.

**Requirements Revealed:**
- Green CI badge visible in repository
- Single-command deployment
- OpenAPI spec for quick API exploration
- Clean, readable code structure
- Architecture Decision Records (ADRs)
- Self-proving correctness via automated tests

---

### Journey Requirements Summary

| Capability | J1: API Consumer | J2: Developer | J3: CI/CD | J4: Review |
|------------|------------------|---------------|-----------|---------------|
| POST /api/coupons | ✓ | | ✓ | ✓ |
| POST /api/coupons/claim | ✓ | ✓ | ✓ | ✓ |
| GET /api/coupons/{name} | ✓ | | ✓ | ✓ |
| OpenAPI Specification | ✓ | ✓ | ✓ | ✓ |
| Docker Compose deployment | | ✓ | ✓ | ✓ |
| PostgreSQL health check | | | ✓ | |
| Stress tests (Flash Sale) | | ✓ | ✓ | ✓ |
| Stress tests (Double Dip) | | ✓ | ✓ | ✓ |
| Unit tests + coverage | | | ✓ | ✓ |
| Race detection | | | ✓ | |
| Quality gates | | | ✓ | |
| README documentation | | ✓ | | ✓ |
| Architecture notes/ADRs | | ✓ | | ✓ |
| Clear error responses | ✓ | | ✓ | |

### OpenAPI Specification Requirement

**MANDATORY:** An OpenAPI 3.0+ specification file (`openapi.yaml`) must be created that:

1. **Strictly follows project requirements** - exact endpoints, request/response schemas
2. **Documents all response codes** - 201, 200, 400, 409 with descriptions
3. **Includes request body schemas** - validated JSON structures
4. **Provides response examples** - for each success/error scenario

**OpenAPI Endpoints (from spec):**

```yaml
paths:
  /api/coupons:
    post:
      summary: Create new coupon
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name, amount]
              properties:
                name: { type: string }
                amount: { type: integer, minimum: 1 }
      responses:
        '201': { description: Coupon created }

  /api/coupons/claim:
    post:
      summary: Claim coupon for user
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [user_id, coupon_name]
              properties:
                user_id: { type: string }
                coupon_name: { type: string }
      responses:
        '200': { description: Claim successful }
        '201': { description: Claim successful }
        '400': { description: No stock available }
        '409': { description: Already claimed by user }

  /api/coupons/{name}:
    get:
      summary: Get coupon details
      parameters:
        - name: name
          in: path
          required: true
          schema: { type: string }
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  name: { type: string }
                  amount: { type: integer }
                  remaining_amount: { type: integer }
                  claimed_by: { type: array, items: { type: string } }
```

## API Backend Specific Requirements

### Project-Type Overview

This is a REST API backend focused on high-concurrency coupon management. The API prioritizes correctness under load over feature breadth, with atomic operations guaranteed by PostgreSQL transactions.

### Endpoint Specifications

| Endpoint | Method | Purpose | Request Schema | Success Response | Error Responses |
|----------|--------|---------|----------------|------------------|-----------------|
| `/api/coupons` | POST | Create coupon | `{"name": string, "amount": integer}` | 201 Created | 400 Bad Request |
| `/api/coupons/claim` | POST | Claim coupon | `{"user_id": string, "coupon_name": string}` | 200/201 OK | 409 Conflict (duplicate), 400 (no stock) |
| `/api/coupons/{name}` | GET | Get coupon details | Path param: name | 200 OK + JSON body | 404 Not Found |

### Authentication Model

**No authentication required** - per specification, the API operates without auth to focus on demonstrating concurrency patterns. This is intentional for a portfolio/demo project.

### Data Schemas

**Request Schemas:**

```json
// POST /api/coupons
{
  "name": "PROMO_SUPER",    // required, string, unique
  "amount": 100             // required, integer, >= 1
}

// POST /api/coupons/claim
{
  "user_id": "user_12345",  // required, string
  "coupon_name": "PROMO_SUPER"  // required, string
}
```

**Response Schemas:**

```json
// GET /api/coupons/{name}
{
  "name": "PROMO_SUPER",
  "amount": 100,
  "remaining_amount": 95,
  "claimed_by": ["user_001", "user_002", "user_003", "user_004", "user_005"]
}
```

### Error Codes

| HTTP Status | Scenario | Response Body |
|-------------|----------|---------------|
| 201 Created | Coupon created successfully | - |
| 200/201 OK | Claim successful | - |
| 400 Bad Request | Invalid request / No stock available | Error message |
| 404 Not Found | Coupon does not exist | Error message |
| 409 Conflict | User already claimed this coupon | Error message |

### API Documentation

**OpenAPI Specification Required:**
- `openapi.yaml` file in repository root
- Version: OpenAPI 3.0+
- Must match exact endpoint specifications
- Include request/response examples
- Document all error scenarios

### Implementation Considerations

**Concurrency Handling:**
- All claim operations wrapped in database transactions
- `SELECT FOR UPDATE` row locking on coupon stock
- Unique constraint on `(user_id, coupon_name)` prevents duplicates at DB level

**Response Headers:**
- `Content-Type: application/json` for all responses

**Not Implemented (Out of Scope):**
- Rate limiting
- API versioning
- SDK/client libraries
- Pagination
- Authentication/Authorization

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-Solving MVP
- Solve the core problem (concurrent coupon claiming) with exactly the specified features
- No feature additions beyond spec requirements
- Focus on correctness, not breadth

**Resource Requirements:**
- Solo developer (Hafiz)
- Skills: Go, PostgreSQL, Docker, CI/CD
- No external dependencies beyond specified tech stack

### MVP Feature Set (Phase 1) - CURRENT SCOPE

**Core User Journeys Supported:**
1. ✓ API Consumer - Flash sale claiming (all 3 endpoints)
2. ✓ Developer - Clone, run, verify pattern
3. ✓ CI/CD - Automated validation pipeline
4. ✓ Portfolio - Review and evaluation

**Must-Have Capabilities (from spec):**

| Feature | Status | Rationale |
|---------|--------|-----------|
| POST /api/coupons | Required | Core CRUD |
| POST /api/coupons/claim | Required | Core concurrency feature |
| GET /api/coupons/{name} | Required | Status verification |
| PostgreSQL with 2 tables | Required | Data persistence |
| Unique constraint (user_id, coupon_name) | Required | Duplicate prevention |
| Atomic transactions | Required | Race condition prevention |
| Docker Compose deployment | Required | Easy setup |
| Flash Sale stress test (50→5) | Required | Spec validation |
| Double Dip stress test (10→1) | Required | Spec validation |
| README documentation | Required | Spec requirement |
| OpenAPI specification | Required | API documentation |
| CI/CD pipeline | Required | Automated validation |
| ≥80% test coverage | Required | Quality gate |

**Explicitly Out of Scope (MVP):**
- Authentication/Authorization
- Rate limiting
- Caching
- Pagination
- Bulk operations
- Admin UI
- Metrics/Monitoring (beyond health)
- API versioning

### Post-MVP Features (If Time Permits)

**Phase 2 - Nice-to-Have Enhancements:**
- Prometheus metrics endpoint
- Structured JSON logging with correlation IDs
- Swagger UI integration
- Additional error detail in responses

**Phase 3 - Future Vision:**
- Authentication layer
- Rate limiting
- Redis caching
- Kubernetes manifests
- Multiple coupon types

### Risk Mitigation Strategy

**Technical Risks:**

| Risk | Mitigation |
|------|------------|
| Race conditions | SELECT FOR UPDATE + unique constraints + stress tests |
| Flaky tests | Deterministic test design, proper DB cleanup |
| Docker issues | Health checks, proper startup order |

**Market Risks:**
- N/A - Portfolio project, not commercial product

**Resource Risks:**

| Risk | Mitigation |
|------|------------|
| Time constraints | Strict scope - spec only, no extras |
| Complexity creep | Explicit out-of-scope list enforced |

### Scope Commitment

**The MVP is FIXED to the specification:**
- No features will be added
- No "improvements" beyond spec requirements
- Scope changes require explicit spec updates

## Functional Requirements

### Coupon Management

- **FR1:** API Consumer can create a new coupon with a unique name and initial stock amount
- **FR2:** API Consumer can retrieve coupon details including name, original amount, remaining amount, and list of users who claimed it
- **FR3:** System maintains accurate remaining_amount that reflects all successful claims

### Claim Processing

- **FR4:** API Consumer can claim a coupon for a specific user_id
- **FR5:** System prevents the same user from claiming the same coupon more than once
- **FR6:** System prevents claims when remaining stock is zero
- **FR7:** System processes concurrent claim requests atomically (no overselling)
- **FR8:** System returns appropriate HTTP status codes for each claim outcome (success, duplicate, no stock)
- **FR9:** System records claim history with user_id and coupon_name

### Data Persistence

- **FR10:** System stores coupon data (name, amount, remaining_amount) in a dedicated table
- **FR11:** System stores claim history (user_id, coupon_name) in a separate table
- **FR12:** System enforces uniqueness on the (user_id, coupon_name) pair at database level
- **FR13:** System ensures claim operations are atomic using database transactions

### API Documentation

- **FR14:** System provides OpenAPI 3.0+ specification documenting all endpoints
- **FR15:** OpenAPI spec includes request/response schemas for all endpoints
- **FR16:** OpenAPI spec documents all possible HTTP status codes and error scenarios

### Infrastructure & Deployment

- **FR17:** System can be started with a single `docker-compose up --build` command
- **FR18:** System waits for PostgreSQL to be ready before accepting API requests
- **FR19:** System handles shutdown gracefully, completing in-flight requests
- **FR20:** System uses environment variables for configuration (not hardcoded values)

### Testing & Validation

- **FR21:** System includes unit tests covering core business logic
- **FR22:** System includes integration tests verifying all API endpoints
- **FR23:** System includes Flash Sale stress test (50 concurrent requests, 5 stock → exactly 5 claims)
- **FR24:** System includes Double Dip stress test (10 concurrent same-user requests → exactly 1 claim)
- **FR25:** All tests can be run via standard Go test commands

### Documentation

- **FR26:** README documents prerequisites for running the system
- **FR27:** README documents the exact command to start the application
- **FR28:** README documents how to run tests
- **FR29:** README explains database design and locking strategy

### CI/CD Pipeline

- **FR30:** GitHub Actions workflow runs on every push/PR
- **FR31:** Pipeline executes unit tests with coverage reporting
- **FR32:** Pipeline executes integration tests
- **FR33:** Pipeline executes stress tests
- **FR34:** Pipeline runs linting (golangci-lint) and static analysis (go vet)
- **FR35:** Pipeline runs security scanning (gosec, govulncheck)
- **FR36:** Pipeline fails if any quality gate is not met

## Non-Functional Requirements

### Performance & Concurrency

- **NFR1:** System handles 50 concurrent claim requests without race conditions
- **NFR2:** System handles 10 concurrent same-user requests with exactly 1 success
- **NFR3:** API responses complete within reasonable time under concurrent load
- **NFR4:** Database transactions complete atomically without deadlocks
- **NFR5:** No goroutine leaks or resource exhaustion under stress test load

### Reliability

- **NFR6:** Stress tests pass 100% of runs (no flaky tests)
- **NFR7:** Race detector (`go test -race`) reports zero data races
- **NFR8:** System recovers gracefully from database connection issues
- **NFR9:** Health check endpoint accurately reflects system readiness
- **NFR10:** Graceful shutdown completes in-flight requests before termination

### Code Quality

- **NFR11:** Unit test coverage ≥80% of business logic
- **NFR12:** Zero errors from golangci-lint
- **NFR13:** Zero issues from go vet static analysis
- **NFR14:** Zero high/critical findings from gosec security scan
- **NFR15:** Zero known vulnerabilities from govulncheck

### Security (Basic)

- **NFR16:** No hardcoded credentials or secrets in codebase
- **NFR17:** Database connection uses environment variables
- **NFR18:** SQL queries use parameterized statements (no SQL injection)
- **NFR19:** Input validation prevents malformed requests from causing errors

### Maintainability

- **NFR20:** Code follows idiomatic Go conventions
- **NFR21:** Clear separation between handlers, services, and repositories
- **NFR22:** Structured logging for debugging and observability
- **NFR23:** Configuration externalized via environment variables

### Developer Experience

- **NFR24:** Clone-to-running-system in <5 minutes
- **NFR25:** Single command deployment (`docker-compose up --build`)
- **NFR26:** All tests runnable with standard `go test` commands
- **NFR27:** README provides complete setup and usage instructions
