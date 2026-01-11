# Story 2.3: OpenAPI Specification for Coupon Endpoints

Status: done

## Story

As a **developer integrating with the API**,
I want **an OpenAPI specification for coupon endpoints**,
So that **I can understand the API contract and generate client code**.

## Acceptance Criteria

### AC1: OpenAPI Version and Location
**Given** the repository root
**When** I look for the API specification
**Then** I find `openapi.yaml` in the repository root
**And** it uses OpenAPI version 3.0 or higher

### AC2: POST /api/coupons Documentation
**Given** the POST /api/coupons specification
**When** I review the request schema
**Then** it defines `name` as required string
**And** it defines `amount` as required integer with minimum: 1
**And** it documents response codes: 201 (created), 400 (bad request), 409 (conflict)

### AC3: GET /api/coupons/{name} Documentation
**Given** the GET /api/coupons/{name} specification
**When** I review the response schema
**Then** it defines the response object with: name, amount, remaining_amount, claimed_by
**And** `claimed_by` is defined as array of strings
**And** it documents response codes: 200 (success), 404 (not found)

### AC4: Validation Passing
**Given** the openapi.yaml file
**When** I validate it with an OpenAPI validator
**Then** it passes validation without errors

## Tasks / Subtasks

- [x] Task 1: Create OpenAPI Specification File (AC: #1)
  - [x] Subtask 1.1: Create `openapi.yaml` in repository root
  - [x] Subtask 1.2: Add OpenAPI version 3.0.3 header
  - [x] Subtask 1.3: Add info section with title, description, version
  - [x] Subtask 1.4: Add servers section with local development URL

- [x] Task 2: Document POST /api/coupons Endpoint (AC: #2)
  - [x] Subtask 2.1: Add path definition for POST /api/coupons
  - [x] Subtask 2.2: Define request body schema with CreateCouponRequest component
  - [x] Subtask 2.3: Add 201 Created response (empty body)
  - [x] Subtask 2.4: Add 400 Bad Request response with error schema
  - [x] Subtask 2.5: Add 409 Conflict response with error schema

- [x] Task 3: Document GET /api/coupons/{name} Endpoint (AC: #3)
  - [x] Subtask 3.1: Add path definition for GET /api/coupons/{name}
  - [x] Subtask 3.2: Define path parameter `name` as required string
  - [x] Subtask 3.3: Define CouponResponse component schema
  - [x] Subtask 3.4: Add 200 OK response with CouponResponse schema
  - [x] Subtask 3.5: Add 404 Not Found response with error schema

- [x] Task 4: Define Reusable Components (AC: #2, #3, #4)
  - [x] Subtask 4.1: Define CreateCouponRequest schema in components/schemas
  - [x] Subtask 4.2: Define CouponResponse schema in components/schemas
  - [x] Subtask 4.3: Define ErrorResponse schema in components/schemas
  - [x] Subtask 4.4: Use $ref references to DRY up response definitions

- [x] Task 5: Validate OpenAPI Specification (AC: #4)
  - [x] Subtask 5.1: Install and run OpenAPI validator (swagger-cli or spectral)
  - [x] Subtask 5.2: Fix any validation errors
  - [x] Subtask 5.3: Verify YAML syntax is correct
  - [x] Subtask 5.4: Test that spec can be imported in Swagger UI or Postman

## Dev Notes

### CRITICAL: This is a Documentation-Only Story

This story creates the `openapi.yaml` file documenting the API contract. No Go code changes are required. The file documents endpoints that are implemented in Stories 2.1 and 2.2.

**Dependencies:**
- Story 2.1 (Create Coupon Endpoint) - defines POST /api/coupons contract
- Story 2.2 (Get Coupon Details) - defines GET /api/coupons/{name} contract

**Note:** The claim endpoint (POST /api/coupons/claim) will be documented in Story 3.3 after it's implemented.

### CRITICAL: API Contract (from Architecture and Epics)

**POST /api/coupons**
- Request: `{"name": "string", "amount": integer}`
- Responses:
  - 201 Created: Empty body
  - 400 Bad Request: `{"error": "invalid request: name is required"}` (or amount messages)
  - 409 Conflict: `{"error": "coupon already exists"}`

**GET /api/coupons/{name}**
- Path Parameter: `name` (string, required)
- Responses:
  - 200 OK:
    ```json
    {
      "name": "PROMO_SUPER",
      "amount": 100,
      "remaining_amount": 95,
      "claimed_by": ["user_001", "user_002"]
    }
    ```
  - 404 Not Found: `{"error": "coupon not found"}`

### CRITICAL: JSON Field Naming

All JSON fields MUST use `snake_case`:
- `name` (NOT `Name`)
- `amount` (NOT `Amount`)
- `remaining_amount` (NOT `remainingAmount`)
- `claimed_by` (NOT `claimedBy`)
- `error` (NOT `Error`)

### CRITICAL: OpenAPI 3.0.3 Specification

```yaml
openapi: 3.0.3
info:
  title: Scalable Coupon System API
  description: |
    A Flash Sale Coupon System REST API demonstrating production-grade backend engineering.
    Handles coupon creation, claiming, and status queries with guaranteed correctness
    under high-concurrency scenarios.
  version: 1.0.0
  license:
    name: MIT
    url: https://opensource.org/licenses/MIT

servers:
  - url: http://localhost:3000
    description: Local development server

paths:
  /api/coupons:
    post:
      summary: Create a new coupon
      description: Creates a coupon with the specified name and stock amount
      operationId: createCoupon
      tags:
        - Coupons
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateCouponRequest'
            examples:
              standard:
                summary: Standard coupon creation
                value:
                  name: "PROMO_SUPER"
                  amount: 100
      responses:
        '201':
          description: Coupon created successfully
        '400':
          description: Bad request - invalid input
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                missingName:
                  summary: Missing name field
                  value:
                    error: "invalid request: name is required"
                missingAmount:
                  summary: Missing amount field
                  value:
                    error: "invalid request: amount is required"
                invalidAmount:
                  summary: Amount less than 1
                  value:
                    error: "invalid request: amount must be at least 1"
        '409':
          description: Conflict - coupon already exists
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                duplicate:
                  summary: Duplicate coupon name
                  value:
                    error: "coupon already exists"

  /api/coupons/{name}:
    get:
      summary: Get coupon details
      description: Retrieves coupon details including who has claimed it
      operationId: getCoupon
      tags:
        - Coupons
      parameters:
        - name: name
          in: path
          required: true
          description: The unique name of the coupon
          schema:
            type: string
          example: "PROMO_SUPER"
      responses:
        '200':
          description: Coupon details retrieved successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CouponResponse'
              examples:
                withClaims:
                  summary: Coupon with claims
                  value:
                    name: "PROMO_SUPER"
                    amount: 100
                    remaining_amount: 95
                    claimed_by: ["user_001", "user_002", "user_003", "user_004", "user_005"]
                noClaims:
                  summary: Coupon with no claims
                  value:
                    name: "NEW_PROMO"
                    amount: 50
                    remaining_amount: 50
                    claimed_by: []
        '404':
          description: Coupon not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                notFound:
                  summary: Coupon does not exist
                  value:
                    error: "coupon not found"

components:
  schemas:
    CreateCouponRequest:
      type: object
      description: Request body for creating a new coupon
      required:
        - name
        - amount
      properties:
        name:
          type: string
          description: Unique name for the coupon
          example: "PROMO_SUPER"
        amount:
          type: integer
          description: Initial stock amount (must be at least 1)
          minimum: 1
          example: 100

    CouponResponse:
      type: object
      description: Response body for coupon details
      required:
        - name
        - amount
        - remaining_amount
        - claimed_by
      properties:
        name:
          type: string
          description: The unique name of the coupon
          example: "PROMO_SUPER"
        amount:
          type: integer
          description: Original stock amount
          example: 100
        remaining_amount:
          type: integer
          description: Current remaining stock
          example: 95
        claimed_by:
          type: array
          description: List of user IDs who have claimed this coupon
          items:
            type: string
          example: ["user_001", "user_002"]

    ErrorResponse:
      type: object
      description: Standard error response format
      required:
        - error
      properties:
        error:
          type: string
          description: Human-readable error message
          example: "coupon not found"
```

### CRITICAL: Validation Commands

Install validation tools:
```bash
# Option 1: swagger-cli (Node.js)
npm install -g @apidevtools/swagger-cli
swagger-cli validate openapi.yaml

# Option 2: spectral (Node.js - more comprehensive)
npm install -g @stoplight/spectral-cli
spectral lint openapi.yaml

# Option 3: openapi-generator-cli (Java)
npx @openapitools/openapi-generator-cli validate -i openapi.yaml
```

**Expected Output (swagger-cli):**
```
openapi.yaml is valid
```

### Project Structure Notes

**Files to CREATE:**
- `openapi.yaml` - OpenAPI 3.0.3 specification (repository root)

**Files NOT to Modify:**
- No Go code changes required
- No database changes required
- No Docker configuration changes required

### CRITICAL: File Placement

The `openapi.yaml` file MUST be placed in the repository root:
```
scalable-coupon-system/
├── openapi.yaml          <-- CREATE HERE
├── cmd/
├── internal/
├── pkg/
├── scripts/
├── docker-compose.yml
├── Dockerfile
└── README.md
```

### CRITICAL: OpenAPI Best Practices

1. **Use $ref for reusable components** - Avoid duplication of schemas
2. **Include examples** - Makes the spec more useful for developers
3. **Use proper HTTP status codes** - 201 for creation, 200 for retrieval, 4xx for errors
4. **Document all error responses** - Include the exact error message format
5. **Use operationId** - Enables code generation with meaningful method names
6. **Use tags** - Groups related endpoints in documentation viewers

### CRITICAL: Claim Endpoint NOT Included

The claim endpoint (`POST /api/coupons/claim`) is NOT documented in this story. It will be added in Story 3.3 (Complete OpenAPI Specification) after the claim functionality is implemented in Stories 3.1 and 3.2.

**Current scope (this story):**
- POST /api/coupons
- GET /api/coupons/{name}

**Future scope (Story 3.3):**
- POST /api/coupons/claim (with 200/400/404/409 responses)

### Previous Story Learnings

From Stories 2.1 and 2.2:
1. **Error messages must be EXACT** - Use the precise wording from acceptance criteria
2. **JSON uses snake_case** - All field names lowercase with underscores
3. **Empty arrays, not null** - `claimed_by` returns `[]` not `null`
4. **201 for creation, 200 for retrieval** - Different success codes for different operations

### Testing Strategy

**Validation Testing:**
1. Run OpenAPI validator - must pass with no errors
2. Import into Swagger UI - verify renders correctly
3. Import into Postman - verify can generate collection
4. Check all examples match actual API behavior

**Manual Verification:**
1. Compare spec against implemented endpoints (Stories 2.1, 2.2)
2. Verify all error messages match exactly
3. Verify all field names use snake_case
4. Verify response schemas match actual responses

### Web Research Findings

**OpenAPI 3.0.3 (Latest Stable in 3.0.x Line):**
- Released: February 2020
- Widely supported by tooling
- Preferred over 3.1.0 for maximum compatibility

**Recommended Validators:**
- `swagger-cli validate` - Fast, simple validation
- `@stoplight/spectral` - Comprehensive linting with best practices
- `openapi-generator-cli validate` - Validates for code generation compatibility

**Common OpenAPI Errors to Avoid:**
- Missing required properties in schema definitions
- Invalid $ref paths (case-sensitive!)
- Using non-standard formats for types
- Forgetting to mark path parameters as required

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#API & Communication Patterns] - API response patterns
- [Source: _bmad-output/planning-artifacts/architecture.md#Naming Patterns] - snake_case convention
- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.3] - Acceptance criteria
- [Source: _bmad-output/implementation-artifacts/2-1-create-coupon-endpoint.md] - POST /api/coupons contract
- [Source: _bmad-output/implementation-artifacts/2-2-get-coupon-details-endpoint.md] - GET /api/coupons/:name contract
- [Source: docs/project-context.md#API Response Patterns] - Error format

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- Validated OpenAPI spec using `npx @apidevtools/swagger-cli@4 validate openapi.yaml` - passed
- Validated YAML syntax using Python yaml.safe_load - passed

### Completion Notes List

- Created `openapi.yaml` with OpenAPI 3.0.3 specification
- Documented POST /api/coupons with request schema and 201/400/409 responses
- Documented GET /api/coupons/{name} with path parameter and 200/404 responses
- Defined reusable components: CreateCouponRequest, CouponResponse, ErrorResponse
- Used $ref references throughout to DRY up response definitions
- All examples match actual API behavior from Stories 2.1 and 2.2
- JSON fields correctly use snake_case (remaining_amount, claimed_by)
- Validation passed with swagger-cli

### Code Review Fixes Applied (2026-01-11)

- Added `tags` section at root level for API documentation clarity
- Added 500 Internal Server Error responses to both endpoints (matching Go implementation)
- Clarified 201 response description as "(empty response body)"
- Added `format: int32` to all integer fields for better code generation

### File List

- `openapi.yaml` (created, modified) - OpenAPI 3.0.3 specification for coupon endpoints

### Change Log

- 2026-01-11: Created OpenAPI specification documenting POST /api/coupons and GET /api/coupons/{name} endpoints
- 2026-01-11: [Code Review] Added 500 Internal Server Error responses to all endpoints, added tags section, added format: int32 to integer fields, clarified 201 empty response body
