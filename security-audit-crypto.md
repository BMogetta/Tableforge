# Security Audit — Cryptographic Failures (OWASP A04:2025)

**Date:** 2026-04-03
**Scope:** All backend services + frontend + infrastructure

---

## Critical

### 1. `.env` with real secrets on disk — ACCEPTED

`.env` is gitignored and NOT committed. Rotate credentials before production. Use secrets manager for deployment.

---

## High

### 2. ~~Hardcoded database credentials in docker-compose~~ — FIXED

All `DATABASE_URL` values now use `${POSTGRES_PASSWORD}` interpolation instead of hardcoded `recess`.

---

## Medium

### 3. PostgreSQL `sslmode=disable` — PENDING (production deployment)

Acceptable in Docker dev. Change to `sslmode=require` for production. Make configurable via env var.

### 4. Redis without authentication or TLS — PENDING

Needs coordinated rollout: add `requirepass`, update `REDIS_URL` in all services. Deferred to production hardening.

### 5-6. No JWT revocation + 7-day TTL — PENDING

Same as auth audit #3. Requires architectural change (Redis blocklist + refresh tokens or short TTL).

### 7. `unsafe-inline` in CSP `script-src` — PENDING

Same as auth audit #15. Needs Vite compatibility investigation.

---

## Low

### 8. gRPC using plaintext — ACCEPTED RISK (Docker-internal)

### 9. ~~HTTP servers lack timeout configuration~~ — FIXED

All 8 services now have `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`.

### 10. ~~No HSTS header~~ — FIXED

Added `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` in Traefik security config.

### 11. `math/rand` in bot MCTS — N/A (appropriate usage)
