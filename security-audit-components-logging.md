# Security Audit — Vulnerable Components & Security Logging (OWASP A03 + A06)

**Date:** 2026-04-03
**Scope:** All 9 Go modules (8 services + shared), frontend (npm), infra (Docker, observability)
**Methodology:** Static review of dependency manifests, log statements, monitoring config, and CI pipelines

---

## Summary

| Category | Severity | Count |
|----------|----------|-------|
| Critical | | 0 |
| High | | 3 |
| Medium | | 5 |
| Low | | 3 |

---

## High

### 1. containerd v1.7.12 — known CVEs (all modules via testcontainers)

`github.com/containerd/containerd v1.7.12` is an indirect dependency pulled by testcontainers-go across 6 modules (shared, game-server, auth-service, user-service, chat-service, notification-service, rating-service). The 1.7.x line before 1.7.13 has multiple CVEs:

- **CVE-2024-24786** (protobuf unmarshaling infinite loop, via containerd's deps)
- **CVE-2023-47108** (otel/grpc) — already fixed in the OTel versions used here, but containerd bundles older transitive deps

**Impact:** Test-only dependency, not compiled into production binaries since testcontainers is used only in `_test.go` files. Risk is limited to CI/dev environments where a malicious container image could exploit containerd.

**Remediation:** Upgrade `testcontainers-go` to v0.31+ which pulls containerd 1.7.15+.

### 2. No audit trail for admin actions — PENDING

Admin operations in user-service have **zero logging**:

- `handleSetPlayerRole` — role changes (privilege escalation) are not logged
- `handleAddAllowedEmail` / `handleRemoveAllowedEmail` — allowlist changes are not logged
- `handleIssueBan` / `handleLiftBan` — bans/unbans are not logged
- `handleReviewReport` — report reviews are not logged
- `handleListPlayers` / `handleListAllowedEmails` — admin data access is not logged

These are the highest-privilege operations in the system. Without logging, there is no way to detect abuse by a compromised manager/owner account, or to investigate incidents after the fact.

**Files:** `services/user-service/internal/api/api_admin.go`, `api_bans.go`, `api_reports.go`

**Remediation:** Add `slog.Info` calls with structured fields for every admin mutation:
```go
slog.Info("admin: role changed", "target", playerID, "new_role", req.Role, "by", callerID)
slog.Info("admin: ban issued", "target", playerID, "ban_id", ban.ID, "by", callerID)
slog.Info("admin: email added", "email", req.Email, "role", req.Role, "by", callerID)
```

### 3. Authentication failures are not logged — PENDING

The `Require` middleware in `shared/middleware/auth.go` returns 401 on missing/invalid cookie without any log statement (lines 87-93). Same for `RequireRole` returning 403 (lines 114-117). The local `requireRole` in `services/user-service/internal/api/api.go` also returns 403 silently.

This means brute-force attacks, token replay attempts, and privilege escalation attempts leave no trace in logs.

**Files:** `shared/middleware/auth.go` (lines 84-93, 114-117), `services/user-service/internal/api/api.go` (lines 112-127)

**Remediation:** Add `slog.Warn` for auth failures with request metadata (IP, path, method). Do NOT log the token value itself.

---

## Medium

### 4. docker/docker v25.0.3 — outdated (indirect dep via testcontainers)

`github.com/docker/docker v25.0.3+incompatible` is pulled transitively by testcontainers. Docker Engine 25.0.3 has known issues patched in later 25.0.x releases. Same test-only caveat as #1.

**Remediation:** Upgrading testcontainers-go (#1) will also resolve this.

### 5. No `govulncheck` or `npm audit` in CI pipeline — PENDING

The CI pipeline (`.github/workflows/ci.yml`) runs `golangci-lint` and `go test` but does not run `govulncheck`. The frontend job runs `npm ci`, `biome lint`, `tsc`, and `vitest` but does not run `npm audit`. There is no Dependabot or Renovate configuration.

Known vulnerabilities in dependencies are only caught by manual review (like this audit).

**Remediation:**
1. Add `govulncheck ./...` step to the `go-test` job
2. Add `npm audit --audit-level=moderate` step to `frontend-ci` job
3. Enable Dependabot or Renovate for automated dependency PRs

### 6. Alertmanager completely disabled — PENDING

Prometheus alerting is commented out in `infra/prometheus.yaml` (lines 44-47). The Alertmanager config file (`infra/alertmanager/alertmanager.yml`) is empty. Per `infra/roadmap.md`, this is a known TODO.

**Impact:** No automated alerting on error rate spikes, high 4xx/5xx rates, service downtime, or security-relevant anomalies (e.g., sudden spike in 401s). The Grafana dashboards exist for manual observation only.

**Remediation:** At minimum, define alert rules for:
- Error rate > threshold per service
- Authentication failure rate spike (requires #3 first)
- Service down (up == 0)
- High latency p99

### 7. PII logged in auth-service — email in plaintext

`services/auth-service/internal/handler/handler.go` line 149:
```go
slog.Warn("auth: email not allowed", "email", email)
```

The user's email address is logged in plaintext when login is rejected. Under GDPR/privacy considerations, PII should be hashed or masked in logs.

**Remediation:** Log a hash or mask the email: `"email", maskEmail(email)` or just log that an email was rejected without the value.

### 8. npm dependencies use caret ranges (^) — PENDING

All 36 dependencies in `frontend/package.json` use `^` version ranges (e.g., `"react": "^19.2.4"`). While `npm ci` with `package-lock.json` ensures reproducible builds in CI, any `npm install` during development will pull the latest compatible minor/patch, which could introduce vulnerabilities silently.

**Impact:** Low in CI (lock file used), medium in dev environments.

**Remediation:** This is standard npm practice and acceptable IF the lock file is always committed and `npm ci` is used in CI (which it is). Consider adding `npm audit` to CI (#5) as a safety net.

---

## Low

### 9. Prometheus only scrapes game-server and OTel collector directly

`infra/prometheus.yaml` has scrape configs for `otel-collector`, `traefik`, and `game-server`. The other 7 services (auth, user, chat, ws-gateway, match, notification, rating) are commented out with a "template" note. While OTel pushes metrics for all services, the direct scrape provides Go runtime metrics (goroutines, heap, GC) that OTel doesn't emit.

**Impact:** Missing Go runtime visibility for 7 of 8 services. Only affects monitoring depth, not security directly.

**Remediation:** Uncomment and configure scrape jobs for services that expose `/metrics` (ws-gateway, match-service already use `prometheus/client_golang`).

### 10. No security-specific Grafana dashboard

The four provisioned dashboards cover: backend traffic, logs, web vitals, JS errors. None focus on security signals like:
- Authentication failure rates
- 403 response rates by endpoint
- Admin action frequency
- Ban/unban activity

**Remediation:** After implementing #2 and #3, create a security dashboard querying Loki for `auth:` and `admin:` log prefixes.

### 11. Log retention is 72h (development setting)

`infra/loki-config.yaml` sets `retention_period: 72h` with a comment noting "set to 30d+ for prod". For a security incident investigation, 72h is insufficient to trace back the timeline of an attack.

**Impact:** Dev-only concern. Production deployment configs are presumably separate.

**Remediation:** Ensure production Loki config uses 30d+ retention. Document this in deployment runbook.

---

## Done Correctly

- **Structured logging throughout** — all services use `log/slog` with key-value pairs, not `fmt.Printf` or unstructured `log.Println`. Structured logs are forwarded to OTel/Loki via the custom `otelHandler` bridge.

- **No secrets in logs** — JWT tokens, OAuth secrets, and passwords are never logged. The `accessToken` from GitHub OAuth is used transiently and never passed to slog. Cookie values are not logged.

- **Go dependency versions are exact** — `go.mod` files pin exact versions (e.g., `v5.9.1`, `v1.79.2`). Go modules do not support floating ranges, so this is inherently safe.

- **Core dependencies are current** — `pgx/v5 v5.9.1`, `go-redis/v9 v9.18.0`, `golang-jwt/v5 v5.3.1`, `grpc v1.79.2`, `protobuf v1.36.11`, `chi/v5 v5.2.5` are all recent releases with no known CVEs.

- **OTel pipeline is well-structured** — traces, metrics, and logs all flow through the OTel Collector with memory limits, batch processing, and resource attribution. The collector-config has `memory_limiter` as the first processor (correct order).

- **HTTP tracing via otelchi** — all services use `otelchi.Middleware` for distributed tracing of HTTP requests. This provides basic access logging through trace data (method, path, status, duration) even without explicit access log middleware.

- **Session metadata captured** — `CreateSession` stores `UserAgent`, `AcceptLanguage`, and `IPAddress` for each login, enabling post-incident analysis of compromised accounts.

- **Redis Pub/Sub events for bans** — ban/unban actions publish events to `player.banned`/`player.unbanned` channels, providing an async audit trail downstream (though the handler itself doesn't log).

- **Docker images use specific Go version** — Dockerfiles pin `golang:1.26-alpine`, not `golang:latest`.

- **CSP and security headers configured** — `infra/traefik/config/security.yml` sets HSTS, X-Content-Type-Options, X-Frame-Options, CSP, and Permissions-Policy.

- **CI uses locked dependencies** — frontend CI runs `npm ci` (respects lock file), Go uses exact module versions. Builds are reproducible.
