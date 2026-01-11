# Story 3.3: Complete OpenAPI Specification

Status: done

## Story

As a **developer integrating with the API**,
I want **a complete OpenAPI specification with all endpoints and error codes**,
So that **I have full documentation for the API contract**.

## Acceptance Criteria

### AC1: Claim Endpoint Request Schema
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

### AC2: Claim Endpoint Response Codes
**Given** the POST /api/coupons/claim specification
**When** I review the response codes
**Then** it documents:
- 200: Claim successful
- 400: Bad request (invalid input OR out of stock)
- 404: Coupon not found
- 409: Already claimed by user

### AC3: Complete Endpoint Coverage
**Given** the complete openapi.yaml file
**When** I compare it to the project specification
**Then** all three endpoints are documented:
- POST /api/coupons
- POST /api/coupons/claim
- GET /api/coupons/{name}
**And** all request/response schemas match the specification exactly
**And** all error scenarios are documented

### AC4: OpenAPI Validation
**Given** the openapi.yaml file
**When** I validate it with an OpenAPI validator
**Then** it passes validation without errors
**And** it can be used to generate client SDKs

## Tasks / Subtasks

- [x] Task 1: Add Claim Endpoint to OpenAPI Spec (AC: #1, #2)
  - [x] Subtask 1.1: Add `POST /api/coupons/claim` path to `openapi.yaml`
  - [x] Subtask 1.2: Define `ClaimCouponRequest` schema with `user_id` (string, required) and `coupon_name` (string, required)
  - [x] Subtask 1.3: Add `operationId: claimCoupon` and `tags: [Claims]`
  - [x] Subtask 1.4: Document response 200/201 (success - empty body)
  - [x] Subtask 1.5: Document response 400 with examples: `missingUserId`, `missingCouponName`, `outOfStock`
  - [x] Subtask 1.6: Document response 404 for coupon not found
  - [x] Subtask 1.7: Document response 409 for already claimed
  - [x] Subtask 1.8: Document response 500 for internal server error

- [x] Task 2: Add Claims Tag (AC: #3)
  - [x] Subtask 2.1: Add `Claims` tag to the tags section with description "Coupon claim operations"

- [x] Task 3: Add ClaimCouponRequest Schema (AC: #1)
  - [x] Subtask 3.1: Add `ClaimCouponRequest` to components/schemas
  - [x] Subtask 3.2: Define `user_id` as required string
  - [x] Subtask 3.3: Define `coupon_name` as required string
  - [x] Subtask 3.4: Add examples and descriptions

- [x] Task 4: Verify All Endpoints Complete (AC: #3)
  - [x] Subtask 4.1: Verify POST /api/coupons is documented (from Story 2.3)
  - [x] Subtask 4.2: Verify GET /api/coupons/{name} is documented (from Story 2.3)
  - [x] Subtask 4.3: Verify POST /api/coupons/claim is documented (new)
  - [x] Subtask 4.4: Verify all response schemas match project specification EXACTLY

- [x] Task 5: Validate OpenAPI Spec (AC: #4)
  - [x] Subtask 5.1: Run OpenAPI validation (e.g., `npx @redocly/cli lint openapi.yaml` or online validator)
  - [x] Subtask 5.2: Fix any validation errors
  - [x] Subtask 5.3: Verify spec can be used for code generation

## Dev Notes

### CRITICAL: Current OpenAPI State

The existing `openapi.yaml` already documents:
- `POST /api/coupons` (Create coupon) - COMPLETE
- `GET /api/coupons/{name}` (Get coupon details) - COMPLETE

**MISSING:** `POST /api/coupons/claim` - This is the primary task for this story.

### CRITICAL: Strict Requirements from Project Specification

From `docs/requirements/flash-sale-coupon-system-spec.md`:

**Claim Endpoint Contract:**
```
Endpoint: POST /api/coupons/claim
Header: Content-Type: application/json
Request Body:
{
  "user_id": "user_12345",
  "coupon_name": "PROMO_SUPER"
}

Response Codes:
- Success: 200 or 201
- Rejected (Already Claimed): 409 Conflict (Preferred) or 400 Bad Request
- Rejected (No Stock): 400 to 409
```

### CRITICAL: HTTP Status Codes (MANDATORY)

From architecture and project context:

| Scenario | Status Code | Error Message |
|----------|-------------|---------------|
| Claim successful | 200 OK | (empty body) |
| Missing user_id | 400 Bad Request | `invalid request: user_id is required` |
| Missing coupon_name | 400 Bad Request | `invalid request: coupon_name is required` |
| Out of stock | 400 Bad Request | `coupon out of stock` |
| Coupon not found | 404 Not Found | `coupon not found` |
| Already claimed | 409 Conflict | `coupon already claimed by user` |
| Internal error | 500 Internal Server Error | `internal server error` |

### CRITICAL: JSON Field Names (snake_case MANDATORY)

All JSON fields MUST use `snake_case`:
- `user_id` (NOT `userId`)
- `coupon_name` (NOT `couponName`)
- `remaining_amount` (NOT `remainingAmount`)
- `claimed_by` (NOT `claimedBy`)

### CRITICAL: OpenAPI Spec Location

File: `openapi.yaml` (repository root)
Version: OpenAPI 3.0.3 (already set in existing file)

### OpenAPI Claim Endpoint Template

```yaml
  /api/coupons/claim:
    post:
      summary: Claim a coupon for a user
      description: |
        Attempts to claim a coupon for a specific user atomically.
        The operation is concurrency-safe using database transactions with row locking.
      operationId: claimCoupon
      tags:
        - Claims
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ClaimCouponRequest'
            examples:
              standard:
                summary: Standard claim request
                value:
                  user_id: "user_12345"
                  coupon_name: "PROMO_SUPER"
      responses:
        '200':
          description: Coupon claimed successfully (empty response body)
        '400':
          description: Bad request - invalid input or out of stock
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                missingUserId:
                  summary: Missing user_id field
                  value:
                    error: "invalid request: user_id is required"
                missingCouponName:
                  summary: Missing coupon_name field
                  value:
                    error: "invalid request: coupon_name is required"
                outOfStock:
                  summary: Coupon out of stock
                  value:
                    error: "coupon out of stock"
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
        '409':
          description: Conflict - user already claimed this coupon
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                alreadyClaimed:
                  summary: Duplicate claim attempt
                  value:
                    error: "coupon already claimed by user"
        '500':
          description: Internal server error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                serverError:
                  summary: Database or server failure
                  value:
                    error: "internal server error"
```

### ClaimCouponRequest Schema Template

```yaml
    ClaimCouponRequest:
      type: object
      description: Request body for claiming a coupon
      required:
        - user_id
        - coupon_name
      properties:
        user_id:
          type: string
          description: Unique identifier of the user claiming the coupon
          example: "user_12345"
        coupon_name:
          type: string
          description: Name of the coupon to claim
          example: "PROMO_SUPER"
```

### Claims Tag Template

```yaml
  - name: Claims
    description: Coupon claim operations (atomic, concurrency-safe)
```

### Project Structure Notes

**Files to MODIFY:**
- `openapi.yaml` - Add claim endpoint, ClaimCouponRequest schema, Claims tag

**Files NOT to MODIFY:**
- No code changes - this is documentation only
- Existing endpoint specs should remain unchanged

### Validation Commands

After modifying the OpenAPI spec, validate using one of:

```bash
# Option 1: Redocly CLI (recommended)
npx @redocly/cli lint openapi.yaml

# Option 2: Swagger CLI
npx swagger-cli validate openapi.yaml

# Option 3: Online validator
# https://editor.swagger.io/ - paste content and check for errors
```

### Previous Story Learnings

From Epic 2 Story 2.3 (OpenAPI for Coupon Endpoints):
1. **Use OpenAPI 3.0.3** - Already set in existing file
2. **Include examples** - Add example values for all request/response fields
3. **Document ALL error codes** - Every possible HTTP status must be documented
4. **Use $ref for schemas** - Reference component schemas instead of inline definitions
5. **Consistent naming** - operationId uses camelCase, tags use PascalCase

### Cross-Story Dependencies

- **Story 3-1** (Claim Endpoint with Atomic Transaction): Defines the handler behavior and error responses
- **Story 3-2** (Transaction Isolation): Defines the concurrency guarantees (mentioned in description)
- **Story 2-3** (OpenAPI for Coupon Endpoints): Established the OpenAPI structure and patterns

### Existing OpenAPI Structure

The current `openapi.yaml` has:
- `info` section with title, description, version
- `servers` section with localhost:3000
- `tags` section with "Coupons" tag
- `paths` with POST /api/coupons and GET /api/coupons/{name}
- `components/schemas` with CreateCouponRequest, CouponResponse, ErrorResponse

### Web Research Intelligence (OpenAPI 3.0 - 2025)

**OpenAPI Best Practices:**
- Use `operationId` for every operation (enables code generation)
- Include `description` for operations, parameters, and schemas
- Use `examples` (plural) for multiple example scenarios
- Document both success and error responses with examples
- Use semantic versioning in `info.version`

**Claim Endpoint Specifics:**
- 200 vs 201: Spec allows both for successful claim - use 200 (consistent with project-context.md)
- 400 for "out of stock" is correct per spec ("400 to 409" range)
- 409 for "already claimed" is preferred per spec

### References

- [Source: docs/requirements/flash-sale-coupon-system-spec.md#Claim Coupon] - Strict API contract
- [Source: docs/project-context.md#API Response Patterns] - HTTP status codes and error messages
- [Source: _bmad-output/planning-artifacts/architecture.md#API Naming Conventions] - snake_case for JSON
- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.3] - Acceptance criteria
- [Source: openapi.yaml] - Existing OpenAPI structure to extend
- [Source: _bmad-output/implementation-artifacts/3-1-claim-coupon-endpoint-with-atomic-transaction.md] - Handler error responses
- [Source: https://spec.openapis.org/oas/v3.0.3] - OpenAPI 3.0.3 specification

## Senior Developer Review (AI)

**Reviewer:** Claude Opus 4.5
**Date:** 2026-01-11
**Outcome:** APPROVED ✅

### Review Summary

| Category | Result |
|----------|--------|
| Acceptance Criteria | 4/4 PASS |
| Tasks Verified | 5/5 DONE |
| OpenAPI Validation | PASS (swagger-cli) |
| Redocly Lint | PASS (1 expected warning) |

### Issues Found & Fixed

| Severity | Issue | Resolution |
|----------|-------|------------|
| MEDIUM | Missing explicit `security: []` declaration | Added to openapi.yaml line 24 |
| MEDIUM | AC2 wording "200/201" mismatched project-context.md | Updated to "200" only |
| MEDIUM | Debug log dismissed security errors as "stylistic" | Clarified as intentional per architecture |
| LOW | Changelog missing validation note | Added code review entry |

### Verification Commands Run

```bash
npx swagger-cli validate openapi.yaml  # ✅ PASS
npx @redocly/cli lint openapi.yaml     # ✅ PASS (1 warning: localhost URL - expected)
```

### Files Modified During Review

- `openapi.yaml` - Added `security: []` at root level
- `3-3-complete-openapi-specification.md` - AC2 wording, debug log, changelog

---

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

- OpenAPI validation passed with `swagger-cli validate openapi.yaml`
- Redocly lint security errors are INTENTIONAL per architecture decision (no authentication by design per spec)
- Added explicit `security: []` to openapi.yaml to document intentional no-auth design
- Localhost URL warning is expected for development environment

### Completion Notes List

- Added `POST /api/coupons/claim` endpoint with full request/response documentation (Lines 88-163)
- Added `Claims` tag with description "Coupon claim operations (atomic, concurrency-safe)" (Lines 20-21)
- Added `ClaimCouponRequest` schema with `user_id` and `coupon_name` required fields (Lines 275-289)
- Documented all response codes: 200 (success), 400 (invalid input/out of stock), 404 (not found), 409 (already claimed), 500 (server error)
- Verified all three endpoints are documented: POST /api/coupons, GET /api/coupons/{name}, POST /api/coupons/claim
- OpenAPI spec validated successfully with swagger-cli

### File List

- `openapi.yaml` (modified) - Added claim endpoint, ClaimCouponRequest schema, Claims tag

### Change Log

- 2026-01-11: Completed Story 3.3 - Added complete OpenAPI specification for claim endpoint with all response codes and schemas
- 2026-01-11: Code Review - Added explicit `security: []` to openapi.yaml, clarified AC2 wording (200 only per project-context.md), updated debug log references

