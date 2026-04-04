# Security Audit — Authentication Failures (OWASP A07:2025)

**Date:** 2026-04-03
**Scope:** All backend services + frontend + infrastructure

---

## Critical

### 1. ~~WS room handler accepts arbitrary `player_id`~~ — FIXED

Now uses `PlayerIDFromContext(r.Context())`. Frontend WS URL no longer sends `player_id`.

### 2. ~~WebSocket upgrader disables origin checking (CSWSH)~~ — FIXED

Upgrader now validates `Origin` header against `ALLOWED_ORIGINS` env var. Empty = allow all (dev mode).

---

## High

### 3. No JWT revocation — tokens valid 7 days after ban/logout — PENDING

Requires Redis-backed blocklist + `jti` claim, or short-lived JWTs + refresh tokens. Architectural change, flagged for future sprint.

### 4. ~~No rate limiting on authentication endpoints~~ — FIXED

Traefik rate limit added to auth-service (20 avg, 10 burst).

### 5. Rating-service HTTP endpoints have no authentication — PENDING DECISION

`GET /ratings/{game_id}/leaderboard` and `GET /players/{id}/ratings/{game_id}` are unauthenticated. Likely intentional (public leaderboard). Need explicit decision and documentation.

### 6. gRPC service-to-service calls are unauthenticated — ACCEPTED RISK

All gRPC uses `insecure.NewCredentials()`. Acceptable for Docker-internal network. Document for cloud migration.

---

## Medium

### 7. WebSocket connections persist after JWT expiry — PENDING

No periodic re-validation in ReadPump/WritePump. Tied to JWT revocation (#3).

### 8. ~~Presence endpoint has no authentication~~ — FIXED

`/api/v1/presence` now wrapped with `authMW`.

### 9. ~~Logout does not require authentication~~ — FIXED

`POST /auth/logout` now wrapped with `authMW`.

### 10. Test login protection relies on env var — ACCEPTED RISK

Double-layer protection (route registration + handler check). Monitor `TEST_MODE` in production deployments.

### 11. ~~No `ReadHeaderTimeout` on HTTP servers~~ — FIXED

Added `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` to all 8 services.

---

## Low

### 12. OAuth redirect uses hardcoded root path — NOT FIXING (UX, not security)

### 13. ~~Cookie `Secure` flag conditional on ENV~~ — FIXED

Inverted default: `secure=true` unless `ENV=development`.

### 14. JWT secret in `.env` file on disk — ACCEPTED

`.env` is gitignored. Use secrets manager for production.

### 15. `unsafe-inline` in CSP `script-src` — PENDING

Needs investigation on whether Vite requires it or if nonces can be used.
