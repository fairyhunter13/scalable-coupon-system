---
stepsCompleted: [1, 2, 3, 4, 5]
inputDocuments:
  - docs/requirements/flash-sale-coupon-system-spec.md
date: 2026-01-11
author: Hafiz
userDecisions:
  - database: PostgreSQL
---

# Product Brief: scalable-coupon-system

## Executive Summary

This project demonstrates senior-level backend engineering competency through a Flash Sale Coupon System REST API. The implementation prioritizes correctness under high concurrency, production-grade architecture, comprehensive testing, and clear documentation.

**Key Objectives:**
- 100% compliance with API specifications
- Guaranteed data consistency under stress conditions
- Self-validating codebase via CI/CD pipeline
- Production-quality code, documentation, and architecture

---

## Core Vision

### Problem Statement

Build a production-grade Flash Sale Coupon System that correctly handles high-concurrency scenarios. The system must demonstrate mastery of concurrent programming, database transactions, and production-ready API development in Go.

### Problem Impact

- **For API Consumers:** Reliable coupon claiming without overselling or duplicate claims
- **For Developers:** Reference implementation of production-grade Go API patterns
- **For the Codebase:** Demonstrate best practices for concurrent systems

### Why This Problem is Challenging

Generic coupon system implementations typically fail to address:
- Race conditions under flash sale load (50 concurrent requests, 5 stock)
- Double-claim prevention from same user (10 concurrent requests)
- Atomic stock management with proper transaction isolation
- Self-proving correctness through automated testing

### Proposed Solution

A Golang REST API with PostgreSQL backend featuring:
- **Atomic Claim Processing:** Database transactions with proper isolation levels
- **Constraint-Based Integrity:** Unique constraint on (user_id, coupon_name) pair
- **Comprehensive Test Suite:** Unit, integration, and stress tests mirroring real-world scenarios
- **CI/CD Pipeline:** GitHub Actions running full test suite including stress tests
- **Production Patterns:** Clean architecture, structured logging, health checks, graceful shutdown

### Key Differentiators

1. **Pre-Validated Correctness:** GitHub Actions runs stress tests automatically
2. **Architecture Documentation:** Clear ADRs explaining design decisions and trade-offs
3. **Production-Grade Quality:** Code, tests, and docs exceed typical project standards
4. **Transparent Results:** Test results and coverage visible in repository

---

## Target Users

### Primary Users

#### 1. API Consumer

**Profile:**
- E-commerce platform or flash sale system integrating coupon functionality
- High-traffic application requiring reliable coupon claims under load

**Goals:**
- Claim coupons atomically without overselling
- Prevent duplicate claims from same user
- Get accurate stock and claim information

**Success Criteria:**
- Zero overselling under concurrent load
- Exactly one successful claim per user per coupon
- Accurate real-time coupon status

#### 2. Developer

**Profile:**
- Backend engineers referencing this as a pattern for concurrent Go APIs
- Teams evaluating concurrency strategies for similar systems

**Goals:**
- Understand architecture decisions and trade-offs
- Learn production patterns for high-concurrency systems
- Quickly run and verify the system works

**Success Criteria:**
- `docker-compose up --build` works on first try
- Code is readable and demonstrates senior-level patterns
- Documentation answers questions before they arise

### Secondary Users

#### 1. Project Author

**Profile:** Hafiz - Backend engineer building this as a portfolio project

**Goals:**
- Demonstrate senior-level Golang competency
- Deliver production-quality code with comprehensive tests
- Showcase testing discipline and documentation quality

#### 2. Open Source Contributors

**Profile:** Developers who might fork or contribute to this project

**Goals:**
- Understand the codebase quickly
- Make contributions with confidence
- Learn from well-structured Go code

### User Journey

**Developer Journey:**
1. **Clone** - `git clone` the repository
2. **Read** - Scan README for setup instructions and architecture notes
3. **Run** - Execute `docker-compose up --build`
4. **Test** - Run automated stress tests against the API
5. **Review** - Examine code quality, patterns, and test coverage
6. **Use/Learn** - Apply patterns to own projects

**Key Insight:** The developer's journey should be frictionless. Every step should "just work."

---

## Success Metrics

### Technical Correctness (Must Pass)

| Test Scenario | Expected Result | Measurement |
|---------------|-----------------|-------------|
| Flash Sale Attack | Exactly 5 successful claims from 50 concurrent requests (5 stock) | Automated stress test |
| Double Dip Attack | Exactly 1 successful claim from 10 concurrent requests (same user) | Automated stress test |
| API Compliance | All endpoints match exact specification | Integration tests |

### Code Quality Metrics

| Metric | Target | Tool |
|--------|--------|------|
| Unit Test Coverage | > 80% | `go test -cover` |
| Linting | Zero errors | `golangci-lint` |
| Static Analysis | Zero issues | `go vet` |
| Security Scan | Zero high/critical | `gosec` |
| Vulnerability Check | Zero known vulnerabilities | `govulncheck` |

### CI/CD Pipeline Success

| Stage | Validation |
|-------|------------|
| Build | `docker-compose up --build` succeeds |
| Unit Tests | All pass with >80% coverage |
| Integration Tests | All API endpoints verified |
| Stress Tests | Flash Sale + Double Dip attacks pass |
| Lint & Security | All quality gates pass |

### Project Objectives

| Objective | Success Indicator |
|-----------|-------------------|
| Demonstrate Concurrency Mastery | All stress tests pass consistently |
| Production-Grade Quality | Clean architecture, idiomatic Go, production patterns |
| Developer Experience | Clone-to-run in <5 minutes |

### Key Performance Indicators

1. **Correctness KPI:** 100% pass rate on stress tests
2. **Quality KPI:** >80% unit test coverage
3. **Reliability KPI:** Zero race conditions detected
4. **Documentation KPI:** README enables clone-to-run in <5 minutes

---

## MVP Scope

### Core Features (Mandatory)

#### API Endpoints (Exact Specification)

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/api/coupons` | POST | Create new coupon | `201 Created` |
| `/api/coupons/claim` | POST | Claim coupon for user | `200/201` success, `409/400` reject |
| `/api/coupons/{name}` | GET | Get coupon details | JSON with name, amount, remaining_amount, claimed_by |

#### Database Design

- **Coupons Table:** name (unique), amount, remaining_amount
- **Claims Table:** user_id, coupon_name, created_at
- **Constraint:** Unique index on (user_id, coupon_name)
- **Separation:** No embedding of claims in coupon records

#### Concurrency Requirements

- Atomic claim process (check eligibility → check stock → insert claim → decrement stock)
- Database transactions with appropriate isolation level
- Race condition prevention via constraints and transactions

#### Infrastructure (Production Standards)

| Component | Purpose |
|-----------|---------|
| Docker Compose | Single command deployment |
| Health Check Endpoint | Container orchestration readiness |
| Graceful Shutdown | Clean connection handling |
| Structured Logging | Debugging and observability |
| Environment Config | Externalized configuration |

### Out of Scope for MVP

| Category | Excluded |
|----------|----------|
| Authentication | No auth required per spec |
| Authorization | No role-based access |
| Rate Limiting | Not in requirements |
| Caching | Not in requirements |
| Pagination | Not in requirements |
| Bulk Operations | Not in requirements |
| Admin UI | Not in requirements |
| Metrics/Monitoring | Beyond health check |
| API Versioning | Single version only |

### MVP Success Criteria

| Criterion | Validation Method |
|-----------|-------------------|
| Flash Sale Attack Passes | 50 concurrent → exactly 5 claims (5 stock) |
| Double Dip Attack Passes | 10 concurrent same user → exactly 1 claim |
| API Spec Compliance | All endpoints match exact contract |
| Docker Deployment | `docker-compose up --build` works first try |
| Test Coverage | >80% unit test coverage |
| CI/CD Green | All GitHub Actions checks pass |

### Future Vision

This project serves as a foundation that could be extended with:
- Authentication and authorization
- Rate limiting and caching
- Metrics and monitoring dashboards
- Additional coupon types and rules

However, the current scope is intentionally focused on demonstrating concurrency correctness.
