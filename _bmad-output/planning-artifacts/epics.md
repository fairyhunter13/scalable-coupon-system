---
stepsCompleted: [1, 2, 3, 4]
status: complete
completedAt: '2026-01-11'
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - project_requirements/strict requirements
  - project_requirements/Scalable_Coupon_System_Spec.md
---

# scalable-coupon-system - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for scalable-coupon-system, decomposing the requirements from the PRD and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

**Coupon Management**
- FR1: API Consumer can create a new coupon with a unique name and initial stock amount
- FR2: API Consumer can retrieve coupon details including name, original amount, remaining amount, and list of users who claimed it
- FR3: System maintains accurate remaining_amount that reflects all successful claims

**Claim Processing**
- FR4: API Consumer can claim a coupon for a specific user_id
- FR5: System prevents the same user from claiming the same coupon more than once
- FR6: System prevents claims when remaining stock is zero
- FR7: System processes concurrent claim requests atomically (no overselling)
- FR8: System returns appropriate HTTP status codes for each claim outcome (success, duplicate, no stock)
- FR9: System records claim history with user_id and coupon_name

**Data Persistence**
- FR10: System stores coupon data (name, amount, remaining_amount) in a dedicated table
- FR11: System stores claim history (user_id, coupon_name) in a separate table
- FR12: System enforces uniqueness on the (user_id, coupon_name) pair at database level
- FR13: System ensures claim operations are atomic using database transactions

**API Documentation**
- FR14: System provides OpenAPI 3.0+ specification documenting all endpoints
- FR15: OpenAPI spec includes request/response schemas for all endpoints
- FR16: OpenAPI spec documents all possible HTTP status codes and error scenarios

**Infrastructure & Deployment**
- FR17: System can be started with a single `docker-compose up --build` command
- FR18: System waits for PostgreSQL to be ready before accepting API requests
- FR19: System handles shutdown gracefully, completing in-flight requests
- FR20: System uses environment variables for configuration (not hardcoded values)

**Testing & Validation**
- FR21: System includes unit tests covering core business logic
- FR22: System includes integration tests verifying all API endpoints
- FR23: System includes Flash Sale stress test (50 concurrent requests, 5 stock -> exactly 5 claims)
- FR24: System includes Double Dip stress test (10 concurrent same-user requests -> exactly 1 claim)
- FR25: All tests can be run via standard Go test commands

**Documentation**
- FR26: README documents prerequisites for running the system
- FR27: README documents the exact command to start the application
- FR28: README documents how to run tests
- FR29: README explains database design and locking strategy

**CI/CD Pipeline**
- FR30: GitHub Actions workflow runs on every push/PR
- FR31: Pipeline executes unit tests with coverage reporting
- FR32: Pipeline executes integration tests
- FR33: Pipeline executes stress tests
- FR34: Pipeline runs linting (golangci-lint) and static analysis (go vet)
- FR35: Pipeline runs security scanning (gosec, govulncheck)
- FR36: Pipeline fails if any quality gate is not met

### NonFunctional Requirements

**Performance & Concurrency**
- NFR1: System handles 50 concurrent claim requests without race conditions
- NFR2: System handles 10 concurrent same-user requests with exactly 1 success
- NFR3: API responses complete within reasonable time under concurrent load
- NFR4: Database transactions complete atomically without deadlocks
- NFR5: No goroutine leaks or resource exhaustion under stress test load

**Reliability**
- NFR6: Stress tests pass 100% of runs (no flaky tests)
- NFR7: Race detector (`go test -race`) reports zero data races
- NFR8: System recovers gracefully from database connection issues
- NFR9: Health check endpoint accurately reflects system readiness
- NFR10: Graceful shutdown completes in-flight requests before termination

**Code Quality**
- NFR11: Unit test coverage >= 80% of business logic
- NFR12: Zero errors from golangci-lint
- NFR13: Zero issues from go vet static analysis
- NFR14: Zero high/critical findings from gosec security scan
- NFR15: Zero known vulnerabilities from govulncheck

**Security (Basic)**
- NFR16: No hardcoded credentials or secrets in codebase
- NFR17: Database connection uses environment variables
- NFR18: SQL queries use parameterized statements (no SQL injection)
- NFR19: Input validation prevents malformed requests from causing errors

**Maintainability**
- NFR20: Code follows idiomatic Go conventions
- NFR21: Clear separation between handlers, services, and repositories
- NFR22: Structured logging for debugging and observability
- NFR23: Configuration externalized via environment variables

**Developer Experience**
- NFR24: Clone-to-running-system in <5 minutes
- NFR25: Single command deployment (`docker-compose up --build`)
- NFR26: All tests runnable with standard `go test` commands
- NFR27: README provides complete setup and usage instructions

### Additional Requirements

**From Architecture - Starter Template:**
- Project uses Fiber v2 + pgx v5 stack (specified for Epic 1 Story 1)
- Go 1.21+ with Fiber v2.52.x
- pgxpool for connection management with prepared statement caching

**From Architecture - Database Design:**
- Two distinct tables: `coupons` and `claims` (no embedding)
- Unique constraint on (user_id, coupon_name) for duplicate prevention
- SELECT FOR UPDATE row locking for atomic stock decrement
- Read Committed isolation level with explicit row locking

**From Architecture - Layered Structure:**
- Handler layer (Fiber HTTP handlers)
- Service layer (business logic, transaction management)
- Repository layer (pgx data access)
- Configuration via envconfig
- Logging via zerolog (zero-allocation)

**From Architecture - Testing Infrastructure:**
- testify for assertions
- dockertest for integration test database lifecycle
- Co-located unit tests with source files
- Separate tests/ directory for integration and stress tests

**From Architecture - Development Tools:**
- Air for hot reload
- golangci-lint for linting
- gosec for security scanning
- govulncheck for vulnerability checking

**From Project Requirements - API Specification:**
- POST /api/coupons - Create coupon (201 Created)
- POST /api/coupons/claim - Claim coupon (200/201 success, 409 duplicate, 400 no stock)
- GET /api/coupons/{name} - Get coupon details with claimed_by list
- Exact JSON field names: name, amount, remaining_amount, claimed_by, user_id, coupon_name

### FR Coverage Map

**Epic 1: Project Foundation & Developer Experience**
- FR17: System can be started with a single `docker-compose up --build` command
- FR18: System waits for PostgreSQL to be ready before accepting API requests
- FR19: System handles shutdown gracefully, completing in-flight requests
- FR20: System uses environment variables for configuration
- FR26: README documents prerequisites for running the system
- FR27: README documents the exact command to start the application

**Epic 2: Coupon Lifecycle Management**
- FR1: API Consumer can create a new coupon with a unique name and initial stock amount
- FR2: API Consumer can retrieve coupon details including name, original amount, remaining amount, and list of users who claimed it
- FR3: System maintains accurate remaining_amount that reflects all successful claims
- FR10: System stores coupon data (name, amount, remaining_amount) in a dedicated table
- FR11: System stores claim history (user_id, coupon_name) in a separate table
- FR14: System provides OpenAPI 3.0+ specification documenting all endpoints (partial)

**Epic 3: Atomic Claim Processing**
- FR4: API Consumer can claim a coupon for a specific user_id
- FR5: System prevents the same user from claiming the same coupon more than once
- FR6: System prevents claims when remaining stock is zero
- FR7: System processes concurrent claim requests atomically (no overselling)
- FR8: System returns appropriate HTTP status codes for each claim outcome
- FR9: System records claim history with user_id and coupon_name
- FR12: System enforces uniqueness on the (user_id, coupon_name) pair at database level
- FR13: System ensures claim operations are atomic using database transactions
- FR15: OpenAPI spec includes request/response schemas for all endpoints
- FR16: OpenAPI spec documents all possible HTTP status codes and error scenarios

**Epic 4: Testing & Quality Assurance**
- FR21: System includes unit tests covering core business logic
- FR22: System includes integration tests verifying all API endpoints
- FR23: System includes Flash Sale stress test (50 concurrent requests, 5 stock -> exactly 5 claims)
- FR24: System includes Double Dip stress test (10 concurrent same-user requests -> exactly 1 claim)
- FR25: All tests can be run via standard Go test commands
- FR28: README documents how to run tests
- FR29: README explains database design and locking strategy

**Epic 5: CI/CD Pipeline & Production Readiness**
- FR30: GitHub Actions workflow runs on every push/PR
- FR31: Pipeline executes unit tests with coverage reporting
- FR32: Pipeline executes integration tests
- FR33: Pipeline executes stress tests
- FR34: Pipeline runs linting (golangci-lint) and static analysis (go vet)
- FR35: Pipeline runs security scanning (gosec, govulncheck)
- FR36: Pipeline fails if any quality gate is not met

## Epic List

### Epic 1: Project Foundation & Developer Experience
Developer can clone the repo, run `docker-compose up --build`, and have a working API server with health checks and graceful shutdown.

**FRs covered:** FR17, FR18, FR19, FR20, FR26, FR27
**NFRs addressed:** NFR8, NFR9, NFR10, NFR16, NFR17, NFR20, NFR21, NFR22, NFR23, NFR24, NFR25

**Key Deliverables:**
- Go project initialized with Fiber v2 + pgx v5 (per Architecture spec)
- Docker Compose with PostgreSQL service
- Health check endpoint (/health)
- Graceful shutdown handling
- Configuration via environment variables
- Structured logging with zerolog
- README with prerequisites and run command

---

### Epic 2: Coupon Lifecycle Management
API consumer can create coupons and retrieve their full details including stock levels and claim history.

**FRs covered:** FR1, FR2, FR3, FR10, FR11, FR14 (partial)
**NFRs addressed:** NFR18, NFR19

**Key Deliverables:**
- POST /api/coupons - Create coupon (201 Created)
- GET /api/coupons/{name} - Get coupon details with claimed_by list
- Database schema: `coupons` table, `claims` table
- Repository layer for data access
- Service layer for business logic
- OpenAPI spec for coupon endpoints

---

### Epic 3: Atomic Claim Processing
API consumer can claim coupons with guaranteed correctness - no overselling under high concurrency, no duplicate claims, and proper error responses.

**FRs covered:** FR4, FR5, FR6, FR7, FR8, FR9, FR12, FR13, FR15, FR16
**NFRs addressed:** NFR1, NFR2, NFR3, NFR4, NFR5, NFR18

**Key Deliverables:**
- POST /api/coupons/claim - Atomic claim endpoint
- SELECT FOR UPDATE row locking for stock decrement
- Unique constraint on (user_id, coupon_name)
- Transaction management in service layer
- HTTP status codes: 200/201 (success), 409 (duplicate), 400 (no stock)
- Complete OpenAPI specification with all response codes

---

### Epic 4: Testing & Quality Assurance
Developer can run comprehensive tests to verify system correctness, including stress tests that prove concurrency safety.

**FRs covered:** FR21, FR22, FR23, FR24, FR25, FR28, FR29
**NFRs addressed:** NFR6, NFR7, NFR11

**Key Deliverables:**
- Unit tests with >= 80% coverage (testify)
- Integration tests for all endpoints (dockertest)
- Flash Sale stress test: 50 concurrent requests, 5 stock -> exactly 5 claims
- Double Dip stress test: 10 same-user concurrent requests -> exactly 1 claim
- Race detection enabled (`go test -race`)
- README with test instructions and architecture notes

---

### Epic 5: CI/CD Pipeline & Production Readiness
Maintainer has automated quality gates that verify correctness on every push/PR, ensuring no regressions.

**FRs covered:** FR30, FR31, FR32, FR33, FR34, FR35, FR36
**NFRs addressed:** NFR12, NFR13, NFR14, NFR15

**Key Deliverables:**
- GitHub Actions workflow (.github/workflows/ci.yml)
- Automated unit tests with coverage reporting
- Automated integration tests
- Automated stress tests
- Quality gates: golangci-lint, go vet, gosec, govulncheck
- Pipeline fails if any gate not met

---

## Epic 1: Project Foundation & Developer Experience

Developer can clone the repo, run `docker-compose up --build`, and have a working API server with health checks and graceful shutdown.

### Story 1.1: Initialize Go Project with Core Dependencies

As a **developer**,
I want **a properly structured Go project with all core dependencies installed**,
So that **I have a solid foundation to build the coupon system API**.

**Acceptance Criteria:**

**Given** a fresh clone of the repository
**When** I run `go mod tidy`
**Then** all dependencies are resolved without errors
**And** the following dependencies are installed:
- github.com/gofiber/fiber/v2 (latest)
- github.com/jackc/pgx/v5
- github.com/jackc/pgx/v5/pgxpool
- github.com/rs/zerolog
- github.com/kelseyhightower/envconfig

**Given** the project structure
**When** I examine the directory layout
**Then** it follows the architecture specification:
- cmd/api/main.go (entry point)
- internal/config/ (configuration)
- internal/handler/ (HTTP handlers)
- internal/service/ (business logic)
- internal/repository/ (data access)
- internal/model/ (domain models)
- pkg/database/ (pgxpool setup)
- scripts/ (SQL scripts)

**Given** the main.go file
**When** I review the code
**Then** it initializes a Fiber app with basic middleware (Recover, Logger)
**And** it loads configuration from environment variables
**And** it sets up structured logging with zerolog

---

### Story 1.2: Docker Compose with PostgreSQL

As a **developer**,
I want **a Docker Compose setup with PostgreSQL**,
So that **I can start the entire system with a single command**.

**Acceptance Criteria:**

**Given** a fresh clone of the repository
**When** I run `docker-compose up --build`
**Then** the system builds and starts successfully
**And** PostgreSQL container is running on port 5432
**And** the API container waits for PostgreSQL to be ready before starting

**Given** the docker-compose.yml file
**When** I review the configuration
**Then** it defines two services: `postgres` and `api`
**And** PostgreSQL uses a health check with `pg_isready`
**And** the API service depends on postgres with `condition: service_healthy`
**And** environment variables are used for database connection (not hardcoded)

**Given** the Dockerfile
**When** I review the build process
**Then** it uses multi-stage build (builder + runtime)
**And** the final image is minimal (alpine-based or scratch)
**And** the binary is built with CGO_ENABLED=0

**Given** the scripts/init.sql file
**When** PostgreSQL starts
**Then** the database schema is initialized automatically
**And** the `coupons` table is created with columns: name (PK), amount, remaining_amount, created_at
**And** the `claims` table is created with columns: id (PK), user_id, coupon_name (FK), created_at
**And** a unique constraint exists on (user_id, coupon_name)
**And** an index exists on claims(coupon_name)

---

### Story 1.3: Health Check & Graceful Shutdown

As a **developer**,
I want **a health check endpoint and graceful shutdown**,
So that **I can verify the system is ready and ensure clean termination**.

**Acceptance Criteria:**

**Given** the API is running
**When** I send a GET request to `/health`
**Then** I receive a 200 OK response
**And** the response indicates PostgreSQL connection is healthy

**Given** the API is running
**When** PostgreSQL becomes unavailable
**Then** the `/health` endpoint returns a non-200 status
**And** the error is logged with zerolog

**Given** the API is running with active requests
**When** I send SIGTERM to the process
**Then** the server stops accepting new connections
**And** in-flight requests are allowed to complete (up to timeout)
**And** database connections are closed cleanly
**And** a shutdown message is logged

**Given** the application startup
**When** PostgreSQL is not yet ready
**Then** the application retries connection with exponential backoff
**And** startup fails gracefully if PostgreSQL remains unavailable

---

### Story 1.4: README Documentation

As a **developer**,
I want **clear README documentation**,
So that **I can understand how to run the system quickly**.

**Acceptance Criteria:**

**Given** the README.md file
**When** I read the Prerequisites section
**Then** it lists Docker Desktop as the only requirement
**And** it specifies minimum Docker version if applicable

**Given** the README.md file
**When** I read the How to Run section
**Then** it provides the exact command: `docker-compose up --build`
**And** it explains what to expect when the system starts
**And** it lists the API base URL (e.g., http://localhost:3000)

**Given** the README.md file
**When** I read the Quick Start section
**Then** it shows example curl commands to verify the API is working
**And** it references the /health endpoint

**Given** the README.md file
**When** I follow all instructions on a fresh machine
**Then** I can have the system running in under 5 minutes

---

## Epic 2: Coupon Lifecycle Management

API consumer can create coupons and retrieve their full details including stock levels and claim history.

### Story 2.1: Create Coupon Endpoint

As an **API consumer**,
I want **to create a new coupon with a name and stock amount**,
So that **I can set up coupons for flash sales**.

**Acceptance Criteria:**

**Given** a valid request body `{"name": "PROMO_SUPER", "amount": 100}`
**When** I send POST to `/api/coupons`
**Then** I receive 201 Created
**And** the coupon is stored in the database with remaining_amount equal to amount

**Given** a request with missing `name` field
**When** I send POST to `/api/coupons`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: name is required"}`

**Given** a request with missing `amount` field
**When** I send POST to `/api/coupons`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: amount is required"}`

**Given** a request with `amount` less than 1
**When** I send POST to `/api/coupons`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: amount must be at least 1"}`

**Given** a coupon with name "PROMO_SUPER" already exists
**When** I send POST to `/api/coupons` with `{"name": "PROMO_SUPER", "amount": 50}`
**Then** I receive 409 Conflict
**And** the response body contains `{"error": "coupon already exists"}`

**Given** the handler layer
**When** I review the code structure
**Then** it follows the layered architecture: Handler -> Service -> Repository
**And** SQL queries use parameterized statements (no SQL injection)

---

### Story 2.2: Get Coupon Details Endpoint

As an **API consumer**,
I want **to retrieve coupon details including who has claimed it**,
So that **I can monitor coupon status during flash sales**.

**Acceptance Criteria:**

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

**Given** a coupon "PROMO_SUPER" exists with no claims
**When** I send GET to `/api/coupons/PROMO_SUPER`
**Then** I receive 200 OK
**And** the `claimed_by` field is an empty array `[]`

**Given** no coupon named "NONEXISTENT" exists
**When** I send GET to `/api/coupons/NONEXISTENT`
**Then** I receive 404 Not Found
**And** the response body contains `{"error": "coupon not found"}`

**Given** the response JSON
**When** I examine the field names
**Then** all fields use snake_case: `name`, `amount`, `remaining_amount`, `claimed_by`

---

### Story 2.3: OpenAPI Specification for Coupon Endpoints

As a **developer integrating with the API**,
I want **an OpenAPI specification for coupon endpoints**,
So that **I can understand the API contract and generate client code**.

**Acceptance Criteria:**

**Given** the openapi.yaml file in the repository root
**When** I review the specification
**Then** it uses OpenAPI version 3.0 or higher
**And** it documents POST /api/coupons with request/response schemas
**And** it documents GET /api/coupons/{name} with response schema

**Given** the POST /api/coupons specification
**When** I review the request schema
**Then** it defines `name` as required string
**And** it defines `amount` as required integer with minimum: 1
**And** it documents response codes: 201 (created), 400 (bad request), 409 (conflict)

**Given** the GET /api/coupons/{name} specification
**When** I review the response schema
**Then** it defines the response object with: name, amount, remaining_amount, claimed_by
**And** `claimed_by` is defined as array of strings
**And** it documents response codes: 200 (success), 404 (not found)

**Given** the openapi.yaml file
**When** I validate it with an OpenAPI validator
**Then** it passes validation without errors

---

## Epic 3: Atomic Claim Processing

API consumer can claim coupons with guaranteed correctness - no overselling under high concurrency, no duplicate claims, and proper error responses.

### Story 3.1: Claim Coupon Endpoint with Atomic Transaction

As an **API consumer**,
I want **to claim a coupon for a user atomically**,
So that **claims are guaranteed correct even under high concurrency**.

**Acceptance Criteria:**

**Given** a coupon "PROMO_SUPER" exists with remaining_amount=5
**And** user "user_001" has not claimed this coupon
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 200 OK (or 201 Created)
**And** a claim record is inserted with user_id="user_001" and coupon_name="PROMO_SUPER"
**And** the coupon's remaining_amount is decremented to 4

**Given** user "user_001" has already claimed coupon "PROMO_SUPER"
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 409 Conflict
**And** the response body contains `{"error": "coupon already claimed by user"}`
**And** no database changes occur

**Given** a coupon "PROMO_SUPER" exists with remaining_amount=0
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_999", "coupon_name": "PROMO_SUPER"}`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "coupon out of stock"}`
**And** no database changes occur

**Given** no coupon named "NONEXISTENT" exists
**When** I send POST to `/api/coupons/claim` with `{"user_id": "user_001", "coupon_name": "NONEXISTENT"}`
**Then** I receive 404 Not Found
**And** the response body contains `{"error": "coupon not found"}`

**Given** a request with missing `user_id` field
**When** I send POST to `/api/coupons/claim`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: user_id is required"}`

**Given** a request with missing `coupon_name` field
**When** I send POST to `/api/coupons/claim`
**Then** I receive 400 Bad Request
**And** the response body contains `{"error": "invalid request: coupon_name is required"}`

---

### Story 3.2: Transaction Isolation and Row Locking

As an **API consumer**,
I want **claim operations to be atomic and isolated**,
So that **concurrent claims never result in overselling or data corruption**.

**Acceptance Criteria:**

**Given** the claim service implementation
**When** I review the transaction flow
**Then** it follows this exact sequence within a single transaction:
1. BEGIN transaction
2. SELECT ... FROM coupons WHERE name = $1 FOR UPDATE (locks the row)
3. Check remaining_amount > 0 (return error if not)
4. INSERT INTO claims (user_id, coupon_name) VALUES ($1, $2)
5. UPDATE coupons SET remaining_amount = remaining_amount - 1
6. COMMIT transaction

**Given** a claim operation fails at any step
**When** an error occurs
**Then** the entire transaction is rolled back
**And** no partial changes are persisted
**And** the error is logged with request context

**Given** the claims table unique constraint on (user_id, coupon_name)
**When** a duplicate claim is attempted concurrently
**Then** the database constraint violation is caught
**And** the transaction is rolled back
**And** 409 Conflict is returned

**Given** two concurrent claim requests for the last available coupon
**When** both requests attempt to claim simultaneously
**Then** exactly one succeeds with 200/201
**And** exactly one fails with 400 (out of stock)
**And** remaining_amount is exactly 0 (not negative)

**Given** the SELECT FOR UPDATE implementation
**When** multiple transactions attempt to lock the same coupon row
**Then** they are serialized (one waits for the other)
**And** no deadlocks occur under normal operation

---

### Story 3.3: Complete OpenAPI Specification

As a **developer integrating with the API**,
I want **a complete OpenAPI specification with all endpoints and error codes**,
So that **I have full documentation for the API contract**.

**Acceptance Criteria:**

**Given** the openapi.yaml file
**When** I review the POST /api/coupons/claim specification
**Then** it defines the request schema:
```yaml
requestBody:
  content:
    application/json:
      schema:
        type: object
        required: [user_id, coupon_name]
        properties:
          user_id: { type: string }
          coupon_name: { type: string }
```

**Given** the POST /api/coupons/claim specification
**When** I review the response codes
**Then** it documents:
- 200/201: Claim successful
- 400: Bad request (invalid input OR out of stock)
- 404: Coupon not found
- 409: Already claimed by user

**Given** the complete openapi.yaml file
**When** I compare it to the project specification
**Then** all three endpoints are documented:
- POST /api/coupons
- POST /api/coupons/claim
- GET /api/coupons/{name}
**And** all request/response schemas match the specification exactly
**And** all error scenarios are documented

**Given** the openapi.yaml file
**When** I validate it with an OpenAPI validator
**Then** it passes validation without errors
**And** it can be used to generate client SDKs

---

## Epic 4: Testing & Quality Assurance

Developer can run comprehensive tests to verify system correctness, including stress tests that prove concurrency safety.

### Story 4.1: Unit Tests for Core Business Logic

As a **developer**,
I want **unit tests covering the core business logic**,
So that **I can verify correctness and catch regressions early**.

**Acceptance Criteria:**

**Given** the service layer code
**When** I run `go test ./internal/service/... -v`
**Then** all unit tests pass
**And** the tests cover:
- Coupon creation validation (valid/invalid inputs)
- Claim eligibility checking logic
- Error mapping (domain errors to appropriate responses)

**Given** the repository layer code
**When** I run `go test ./internal/repository/... -v`
**Then** all unit tests pass
**And** repository methods are tested with mock database connections

**Given** the unit test suite
**When** I run `go test -cover ./internal/...`
**Then** code coverage is >= 80% for business logic
**And** coverage report identifies any untested paths

**Given** the unit test suite
**When** I run `go test -race ./internal/...`
**Then** zero data races are detected
**And** all tests pass under race detection

**Given** the test file organization
**When** I examine the project structure
**Then** unit tests are co-located with source files (e.g., `coupon_service_test.go`)
**And** tests use testify for assertions

---

### Story 4.2: Integration Tests for All Endpoints

As a **developer**,
I want **integration tests that verify all API endpoints**,
So that **I can confirm the full request/response cycle works correctly**.

**Acceptance Criteria:**

**Given** the integration test suite
**When** I run `go test ./tests/integration/... -v`
**Then** all integration tests pass
**And** a real PostgreSQL database is used (via dockertest)

**Given** the POST /api/coupons integration tests
**When** they execute
**Then** they verify:
- 201 Created for valid coupon creation
- 400 Bad Request for invalid input
- 409 Conflict for duplicate coupon name

**Given** the GET /api/coupons/{name} integration tests
**When** they execute
**Then** they verify:
- 200 OK with correct JSON structure
- 404 Not Found for non-existent coupon
- claimed_by list accuracy after claims

**Given** the POST /api/coupons/claim integration tests
**When** they execute
**Then** they verify:
- 200/201 for successful claim
- 409 Conflict for duplicate claim
- 400 Bad Request for out of stock
- 404 Not Found for non-existent coupon

**Given** the integration test setup
**When** I review `tests/integration/setup_test.go`
**Then** it uses dockertest to spin up PostgreSQL
**And** each test runs with a clean database state
**And** containers are cleaned up after tests complete

---

### Story 4.3: Flash Sale Stress Test

As a **developer**,
I want **a stress test simulating a flash sale attack**,
So that **I can prove the system handles 50 concurrent requests correctly**.

**Acceptance Criteria:**

**Given** a coupon "FLASH_TEST" with amount=5
**When** 50 concurrent goroutines attempt to claim it simultaneously
**Then** exactly 5 claims succeed (200/201 responses)
**And** exactly 45 claims fail (400 out of stock)
**And** remaining_amount is exactly 0
**And** claimed_by contains exactly 5 unique user IDs

**Given** the flash sale stress test
**When** I run `go test ./tests/stress/... -run TestFlashSale -v`
**Then** the test passes consistently (100% of runs)
**And** execution completes within reasonable time (< 30 seconds)

**Given** the stress test implementation
**When** I review the code
**Then** it uses sync.WaitGroup for goroutine coordination
**And** it collects and counts response status codes
**And** it verifies final database state matches expectations

**Given** the stress test
**When** I run it 10 times consecutively
**Then** it passes all 10 runs without flakiness
**And** results are deterministic (exactly 5 successes each time)

---

### Story 4.4: Double Dip Stress Test

As a **developer**,
I want **a stress test simulating duplicate claim attempts**,
So that **I can prove the unique constraint prevents double claims**.

**Acceptance Criteria:**

**Given** a coupon "DOUBLE_TEST" with amount=100
**And** a single user "user_greedy"
**When** 10 concurrent goroutines attempt to claim for "user_greedy" simultaneously
**Then** exactly 1 claim succeeds (200/201 response)
**And** exactly 9 claims fail (409 Conflict)
**And** remaining_amount is exactly 99
**And** claimed_by contains exactly ["user_greedy"]

**Given** the double dip stress test
**When** I run `go test ./tests/stress/... -run TestDoubleDip -v`
**Then** the test passes consistently (100% of runs)
**And** the unique constraint violation is properly handled

**Given** the stress test
**When** I run it 10 times consecutively
**Then** it passes all 10 runs without flakiness
**And** exactly 1 success is recorded each time

**Given** the double dip scenario
**When** I verify the database state
**Then** only one claim record exists for (user_greedy, DOUBLE_TEST)
**And** no duplicate records were ever inserted (even temporarily)

---

### Story 4.5: Complete README with Test Instructions and Architecture Notes

As a **developer**,
I want **complete README documentation including test instructions and architecture notes**,
So that **I can understand how to verify the system and how it works internally**.

**Acceptance Criteria:**

**Given** the README.md file
**When** I read the "How to Test" section
**Then** it documents:
- `go test ./...` - run all tests
- `go test ./internal/...` - run unit tests only
- `go test ./tests/integration/...` - run integration tests
- `go test ./tests/stress/...` - run stress tests
- `go test -race ./...` - run with race detection
- `go test -cover ./...` - run with coverage

**Given** the README.md file
**When** I read the "Architecture Notes" section
**Then** it explains the database design:
- Two tables: `coupons` and `claims` (separation of concerns)
- Unique constraint on (user_id, coupon_name) for duplicate prevention
- Index on claims(coupon_name) for efficient lookups

**Given** the README.md file
**When** I read the "Locking Strategy" subsection
**Then** it explains:
- SELECT FOR UPDATE row locking mechanism
- Transaction flow: lock -> check -> insert -> decrement -> commit
- Why this prevents race conditions and overselling
- Read Committed isolation level with explicit locking

**Given** the README.md file
**When** I read the "Stress Test Results" subsection
**Then** it documents expected outcomes:
- Flash Sale: 50 requests, 5 stock -> exactly 5 claims
- Double Dip: 10 same-user requests -> exactly 1 claim

**Given** the complete README.md
**When** I follow all instructions
**Then** I can run all tests successfully
**And** I understand the system architecture

---

## Epic 5: CI/CD Pipeline & Production Readiness

Maintainer has automated quality gates that verify correctness on every push/PR, ensuring no regressions.

### Story 5.1: GitHub Actions CI Workflow

As a **maintainer**,
I want **a GitHub Actions workflow that runs on every push and PR**,
So that **code quality is automatically verified before merging**.

**Acceptance Criteria:**

**Given** a push to any branch
**When** GitHub Actions triggers
**Then** the CI workflow starts automatically
**And** the workflow runs all defined jobs

**Given** a pull request is opened or updated
**When** GitHub Actions triggers
**Then** the CI workflow runs on the PR
**And** status checks are reported on the PR

**Given** the CI workflow file `.github/workflows/ci.yml`
**When** I review its structure
**Then** it defines these jobs:
- `build`: Builds the Docker image
- `test`: Runs unit, integration, and stress tests
- `lint`: Runs linting and static analysis
- `security`: Runs security scanning

**Given** the `test` job
**When** it executes
**Then** it spins up PostgreSQL service container
**And** it waits for PostgreSQL to be healthy
**And** it runs `go test -race -coverprofile=coverage.out ./...`
**And** it uploads coverage report as artifact

**Given** the `test` job includes stress tests
**When** stress tests execute in CI
**Then** Flash Sale test passes (50 concurrent -> 5 claims)
**And** Double Dip test passes (10 same-user -> 1 claim)
**And** tests complete within CI timeout limits

**Given** the workflow configuration
**When** I review the PostgreSQL service
**Then** it uses the same version as docker-compose (PostgreSQL 15+)
**And** health checks ensure DB is ready before tests run

**Given** the need to monitor CI/CD results
**When** verifying workflow execution
**Then** developers MUST use `gh` CLI commands:
- `gh run list` - list recent workflow runs
- `gh run watch` - watch running workflow in real-time
- `gh run view <run-id> --log` - view workflow logs
- `gh run view --log-failed` - view failed job logs
- `gh run rerun --failed` - re-run failed jobs
**And** `gh pr checks` to verify PR status checks

---

### Story 5.2: Quality Gates - Linting and Static Analysis

As a **maintainer**,
I want **automated linting and static analysis**,
So that **code quality standards are enforced consistently**.

**Acceptance Criteria:**

**Given** the `lint` job in CI
**When** it executes
**Then** it runs golangci-lint with project configuration
**And** it runs go vet on all packages
**And** any errors cause the job to fail

**Given** the `.golangci.yml` configuration file
**When** I review its settings
**Then** it enables recommended linters:
- errcheck (unchecked errors)
- gosimple (simplifications)
- govet (suspicious constructs)
- ineffassign (ineffectual assignments)
- staticcheck (static analysis)
- unused (unused code)

**Given** the codebase
**When** I run `golangci-lint run ./...` locally
**Then** zero errors are reported
**And** output matches CI results

**Given** the codebase
**When** I run `go vet ./...` locally
**Then** zero issues are reported

**Given** a PR with linting errors
**When** the CI workflow runs
**Then** the `lint` job fails
**And** the PR cannot be merged until fixed
**And** error details are visible in the workflow logs

**Given** the Makefile
**When** I review its targets
**Then** it includes:
- `make lint` - runs golangci-lint
- `make vet` - runs go vet
- `make check` - runs all quality checks

---

### Story 5.3: Security Scanning

As a **maintainer**,
I want **automated security scanning**,
So that **vulnerabilities are detected before deployment**.

**Acceptance Criteria:**

**Given** the `security` job in CI
**When** it executes
**Then** it runs gosec on all packages
**And** it runs govulncheck for known vulnerabilities
**And** any high/critical findings cause the job to fail

**Given** the codebase
**When** I run `gosec ./...` locally
**Then** zero high or critical findings are reported
**And** any informational findings are reviewed and acceptable

**Given** the codebase
**When** I run `govulncheck ./...` locally
**Then** zero known vulnerabilities are reported in dependencies
**And** all dependencies are up to date

**Given** the gosec configuration
**When** I review excluded rules (if any)
**Then** each exclusion is documented with justification
**And** no security-critical rules are disabled

**Given** a PR that introduces a vulnerability
**When** the CI workflow runs
**Then** the `security` job fails
**And** the vulnerability details are logged
**And** the PR is blocked until the issue is resolved

**Given** the Makefile
**When** I review its targets
**Then** it includes:
- `make security` - runs gosec and govulncheck
- `make all` - runs build, test, lint, and security

---

### Story 5.4: CI Pipeline Integration and Quality Gates

As a **maintainer**,
I want **all CI jobs integrated with proper quality gates**,
So that **only fully validated code can be merged**.

**Acceptance Criteria:**

**Given** the complete CI workflow
**When** all jobs pass
**Then** the overall workflow status is "success"
**And** a green checkmark appears on the PR/commit

**Given** any single job fails
**When** the workflow completes
**Then** the overall workflow status is "failure"
**And** the specific failed job is clearly identified
**And** error logs are accessible for debugging

**Given** the GitHub repository settings
**When** I review branch protection rules for `main`
**Then** the CI workflow is required to pass before merging
**And** PRs cannot be merged with failing checks

**Given** the CI workflow timing
**When** I measure typical execution time
**Then** the full pipeline completes in < 10 minutes
**And** jobs run in parallel where possible (lint + security)

**Given** the workflow badges
**When** I view the README.md
**Then** a CI status badge is displayed
**And** it reflects the current build status of the main branch

**Given** the complete pipeline
**When** I run a full CI cycle
**Then** these quality gates are enforced:
- Build succeeds
- Unit tests pass with >= 80% coverage
- Integration tests pass
- Stress tests pass (Flash Sale + Double Dip)
- Zero race conditions detected
- Zero golangci-lint errors
- Zero go vet issues
- Zero gosec high/critical findings
- Zero govulncheck vulnerabilities

**Given** the need to verify CI results during development
**When** a developer pushes code or creates a PR
**Then** they MUST use `gh` CLI to monitor workflow execution:
```bash
# Watch workflow in real-time
gh run watch

# Check PR status
gh pr checks

# View failed logs for debugging
gh run view --log-failed
```
**And** NOT rely solely on GitHub web UI for CI/CD feedback

---

### Story 5.5: README Status Badges

As a **developer viewing the repository**,
I want **comprehensive status badges in the README**,
So that **I can immediately see the health of the project at a glance**.

**Acceptance Criteria:**

**Given** the README.md file
**When** I view the top of the document
**Then** I see a comprehensive badge section with the following categories:

**Build & CI Badges:**
- CI/CD pipeline status (GitHub Actions workflow)
- Build status (passing/failing)

**Test Badges:**
- Unit tests status
- Integration tests status
- Stress tests status
- Race detection status (`-race` flag)
- Test coverage percentage (with color coding: green >=80%, yellow >=60%, red <60%)

**Code Quality Badges:**
- Go Report Card (goreportcard.com)
- golangci-lint status
- go vet status

**Security Badges:**
- gosec security scan status
- govulncheck vulnerability status

**Project Info Badges:**
- Go version (minimum required)
- License (MIT or applicable)
- Go Reference (pkg.go.dev documentation link)

**Given** the GitHub Actions workflow
**When** it completes successfully
**Then** all relevant badges automatically update to reflect current status
**And** coverage badge shows actual percentage from coverage report
**And** badge colors reflect status (green=pass, red=fail, yellow=warning)

**Given** the badge implementation
**When** I review the configuration
**Then** badges use a combination of:
- GitHub's native workflow status badges for CI jobs
- shields.io for custom badges (coverage, Go version, license)
- goreportcard.com badge for Go code quality
- pkg.go.dev badge for documentation

**Given** the badge links
**When** I click on any badge
**Then** it navigates to the relevant resource:
- CI badges -> GitHub Actions workflow runs
- Coverage badge -> Coverage report or Codecov
- Go Report Card -> goreportcard.com analysis page
- Go Reference -> pkg.go.dev documentation
- License -> LICENSE file in repository

**Given** the coverage reporting
**When** tests run in CI with `go test -coverprofile=coverage.out`
**Then** coverage percentage is extracted and reported
**And** coverage badge is updated via Codecov, Coveralls, or custom script
**And** coverage threshold of 80% is enforced as quality gate

**Given** the README badge section layout
**When** I view the badges
**Then** they are organized in a clean, readable format:
```markdown
# Project Name

[![CI](badge-url)](link) [![Coverage](badge-url)](link) [![Go Report Card](badge-url)](link)
[![Go Reference](badge-url)](link) [![License](badge-url)](link) [![Go Version](badge-url)](link)
```

**Given** the Go Report Card integration
**When** the repository is public on GitHub
**Then** goreportcard.com automatically analyzes the codebase
**And** provides grades for: gofmt, go vet, gocyclo, golint, ineffassign, license, misspell

**Given** the stress test badges
**When** CI runs stress tests
**Then** separate status indicators show:
- Flash Sale test: 50 concurrent -> 5 claims (pass/fail)
- Double Dip test: 10 same-user -> 1 claim (pass/fail)
