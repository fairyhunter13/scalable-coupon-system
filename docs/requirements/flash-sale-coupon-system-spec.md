# Flash Sale Coupon System - Technical Specification

## Objective

Build a REST API in **Golang** for a "Flash Sale" Coupon System. The system must handle high concurrency, guarantee strict data consistency, and be easy to deploy via Docker.

The API must follow the specifications exactly as described below.

## Tech Stack

| Component | Requirement |
|-----------|-------------|
| Language | Golang |
| Database | PostgreSQL |
| Infrastructure | Docker / Docker Compose |

## Database Constraints & Rules

1. **Separation of Concerns:** Coupon data and Claim history must be in two distinct tables.
2. **No Embedding:** Claim history must NOT be embedded inside the Coupon record.
3. **Uniqueness Rule (CRITICAL):**
   - A `user_id` can claim a specific `coupon_name` **only once**
   - The same `user_id` **can** claim other different coupons
   - Database schema must enforce uniqueness on the pair `(user_id, coupon_name)` to prevent race conditions

---

## API Specifications (Strict Contract)

### 1. Create Coupon

Registers a new coupon into the system.

- **Endpoint:** `POST /api/coupons`
- **Request Body:** `{"name": "PROMO_SUPER", "amount": 100}`
- **Response:** `201 Created`

### 2. Claim Coupon

Attempts to claim a coupon for a specific user.

- **Endpoint:** `POST /api/coupons/claim`
- **Header:** `Content-Type: application/json`
- **Request Body:**
  ```json
  {
    "user_id": "user_12345",
    "coupon_name": "PROMO_SUPER"
  }
  ```
- **Logic & Behavior:**
  - **Check Eligibility:** If user has already claimed this coupon, reject immediately
  - **Check Stock:** If stock is 0, reject
  - **Concurrency Safety:** The process of checking stock, inserting claim, and deducting stock must be **Atomic** (using Database Transactions)
- **Response Codes:**
  - **Success:** `200` or `201`
  - **Rejected (Already Claimed):** `409 Conflict` (Preferred) or `400 Bad Request`
  - **Rejected (No Stock):** `400` to `409`

### 3. Get Coupon Details

- **Endpoint:** `GET /api/coupons/{name}`
- **Response Body:**
  ```json
  {
    "name": "PROMO_SUPER",
    "amount": 100,
    "remaining_amount": 0,
    "claimed_by": ["user_12345", ...]
  }
  ```

---

## Stress Test Scenarios

The system must handle these concurrency scenarios correctly:

### 1. Flash Sale Attack

- **Scenario:** 50 concurrent requests for a coupon with only 5 items in stock
- **Expected Result:** Exactly 5 successful claims, 0 remaining stock
- **Purpose:** Validates atomic stock decrement under high concurrency

### 2. Double Dip Attack

- **Scenario:** 10 concurrent requests from the **SAME** user for the same coupon
- **Expected Result:** Exactly 1 success, 9 failures
- **Purpose:** Validates uniqueness constraint enforcement under race conditions

---

## Documentation Requirements

The repository must include a `README.md` containing:

- **Prerequisites:** What needs to be installed (e.g., Docker Desktop)
- **How to Run:** The exact command to start the application (e.g., `docker-compose up --build`)
- **How to Test:** Instructions on how to run tests or trigger the endpoints
- **Architecture Notes:** Brief explanation of database design and locking strategy
