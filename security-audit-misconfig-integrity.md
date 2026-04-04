# Security Audit — Misconfiguration & Data Integrity (OWASP A02 + A08)

**Date:** 2026-04-03
**Scope:** All backend services + frontend + infrastructure

---

## Summary Table

| # | Severity | Category | Finding |
|---|----------|----------|---------|
| 1 | High | A02 | Traefik dashboard exposed with `--api.insecure=true` |
| 2 | High | A02 | gRPC reflection enabled unconditionally in production |
| 3 | High | A08 | CD pipeline deploys without requiring CI to pass |
| 4 | Medium | A02 | `ENV=development` hardcoded in docker-compose for game-server |
| 5 | Medium | A02 | Metrics endpoints exposed without authentication |
| 6 | Medium | A02 | Grafana anonymous admin access (monitoring profile) |
| 7 | Medium | A02 | game-server hardcoded default DATABASE_URL with credentials |
| 8 | Medium | A02 | TanStack DevTools rendered unconditionally in production builds |
| 9 | Medium | A02 | `chi/middleware.Recoverer` prints stack traces to HTTP response |
| 10 | Medium | A08 | Docker images tagged `:latest` for monitoring stack (unpinned) |
| 11 | Medium | A08 | Frontend dependencies use caret ranges (`^`) in package.json |
| 12 | Low | A08 | No image signing or provenance attestation in CD pipeline |
| 13 | Low | A02 | `seed-test` Dockerfile runs as root |
| 14 | Low | A02 | No `IdleTimeout` on HTTP servers |
| 15 | Info | A02 | `unsafe-inline` in CSP `script-src` |

---

## High

### 1. Traefik dashboard exposed with `--api.insecure=true`

**File:** `docker-compose.yml`, lines 155-156

Traefik is configured with `--api.dashboard=true` and `--api.insecure=true`, which exposes the Traefik dashboard on port 8080 (mapped to host port 9090). The dashboard shows all routers, services, middlewares, and their configuration — effectively a map of the entire backend architecture. No authentication is required.

**Impact:** An attacker can enumerate all internal service routes, middleware chains, and service discovery information. This is a full architecture disclosure.

**Recommendation:** Either disable the dashboard in production (`--api.dashboard=false`), or remove `--api.insecure=true` and protect it with BasicAuth middleware or an IP allowlist. At minimum, don't expose port 9090 externally.

### 2. gRPC reflection enabled unconditionally in production

**Files:**
- `services/game-server/cmd/server/main.go:114` — `reflection.Register(grpcServer)`
- `services/rating-service/cmd/server/main.go:77` — `reflection.Register(grpcServer)`

gRPC server reflection is registered without any environment check. Reflection allows any gRPC client (e.g., `grpcurl`) to enumerate all available services, methods, and message types on the gRPC port. While these ports are Docker-internal and not exposed via Traefik, they are reachable from any container on `data_network` or `app_network`.

**Impact:** Service enumeration from a compromised container. Combined with the lack of gRPC authentication, an attacker on the Docker network can discover and call any gRPC method.

**Recommendation:** Gate reflection behind an environment check (e.g., only enable when `ENV=development` or `TEST_MODE=true`). User-service and other gRPC servers do not register reflection — follow that pattern.

### 3. CD pipeline deploys without requiring CI to pass

**File:** `.github/workflows/cd.yml`

The CD workflow triggers on `push` to `main` and builds + pushes Docker images to GHCR. It has no `needs:` dependency on any CI job. The CI workflow (`ci.yml`) runs separately on the same trigger. A push to main will build and publish container images regardless of whether tests pass, linting succeeds, or the compose config validates.

**Impact:** Broken, untested, or vulnerable code can be published as container images.

**Recommendation:** Either make CD jobs depend on CI jobs (using `workflow_run` trigger or a shared workflow), or add a branch protection rule requiring CI status checks before merge to main.

---

## Medium

### 4. `ENV=development` hardcoded in docker-compose for game-server

**File:** `docker-compose.yml:107`

The game-server service has `ENV=development` hardcoded. The auth-service uses this variable to decide whether cookies get the `Secure` flag (line 46 in auth-service main.go: `secure := config.Env("ENV", "production") != "development"`). If the same compose file is used in production without overriding this value, session cookies will be sent over plaintext HTTP.

**Impact:** Session hijacking via cookie interception if deployed without override.

**Recommendation:** Remove the hardcoded `ENV=development` from docker-compose.yml or use `${ENV:-development}` so it can be overridden from `.env`. Ensure production deployment always sets `ENV=production`.

### 5. Metrics endpoints exposed without authentication

**Files:**
- `services/game-server/internal/platform/api/api.go:54` — `r.Get("/metrics", ...)`
- `services/ws-gateway/cmd/server/main.go:106` — `r.Get("/metrics", ...)`
- `services/match-service/cmd/server/main.go:141` — `r.Get("/metrics", ...)`

Prometheus metrics endpoints are registered outside the auth middleware group. These endpoints are reachable via Traefik (the game-server's `/metrics` matches the `/api` catch-all, and ws-gateway and match-service routes are similarly accessible). Metrics can reveal request rates, error rates, goroutine counts, memory usage, and custom application metrics.

**Impact:** Information disclosure — an attacker can infer traffic patterns, service health, and internal performance characteristics.

**Recommendation:** Either exclude `/metrics` from Traefik routing (add a PathPrefix rule that blocks it) or put metrics behind auth. Alternatively, expose metrics only on a separate internal port not routed through Traefik, and scrape from the Docker internal network only.

### 6. Grafana anonymous admin access (monitoring profile)

**File:** `docker-compose.monitoring.yml:98-99`

```yaml
GF_AUTH_ANONYMOUS_ENABLED=true
GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
```

While the comment says "In production, put Grafana behind Cloudflare Access", this is still dangerous as the default configuration. Anonymous users get full Admin access, which allows creating/modifying dashboards, configuring data sources, and potentially accessing sensitive data from Loki logs (which may contain request bodies, JWTs in error logs, etc.). If the monitoring profile is accidentally enabled in production without the Cloudflare override, full admin access is open.

**Impact:** Complete Grafana takeover, potential access to logs containing secrets.

**Recommendation:** Default to `GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer` at minimum. Use environment variable interpolation (`${GF_AUTH_ANONYMOUS_ORG_ROLE:-Viewer}`) so production can set it to disabled. Better yet, disable anonymous auth by default and require explicit opt-in for dev.

### 7. game-server hardcoded default DATABASE_URL with credentials

**File:** `services/game-server/cmd/server/main.go:61`

```go
config.Env("DATABASE_URL", "postgres://recess:recess@localhost:5432/recess?sslmode=disable")
```

The game-server is the only service that falls back to a hardcoded DATABASE_URL with credentials (`recess:recess`). All other services use `config.MustEnv("DATABASE_URL")` which panics if the variable is unset. If the game-server is accidentally started without the env var (e.g., during a misconfigured deployment), it would attempt to connect with default credentials.

Note: The crypto audit already covers `sslmode=disable` (finding #3) and redis auth (finding #4). This finding is specifically about the hardcoded credential fallback pattern.

**Impact:** Credential leakage in source code; potential connection to wrong database with default credentials.

**Recommendation:** Switch to `config.MustEnv("DATABASE_URL")` like all other services, eliminating the fallback.

### 8. TanStack DevTools rendered unconditionally in production builds

**File:** `frontend/src/main.tsx:51-67`

The `<TanStackDevtools>` component (with router, query, pacer, and WebSocket panels) is rendered unconditionally. Only the `<DevtoolsCapture />` component is gated by `isDev`. The TanStack DevTools UI is visible in production builds, allowing users to inspect the React Query cache (which contains API responses), router state, and WebSocket events.

**Impact:** Information disclosure — production users can inspect cached API data, routing internals, and real-time WebSocket message flow.

**Recommendation:** Wrap the entire `<TanStackDevtools>` block in the `isDev` conditional, or use lazy loading that only imports the devtools in dev/test mode.

### 9. `chi/middleware.Recoverer` prints stack traces to HTTP response

**All services** use `middleware.Recoverer` from go-chi. By default, this middleware catches panics and writes the stack trace to the HTTP response body as plain text. This exposes internal file paths, function names, and goroutine state to the client.

**Impact:** Information disclosure on panic — internal code paths, Go version, and dependency structure.

**Recommendation:** Use a custom recovery middleware that logs the stack trace server-side but returns a generic 500 JSON error to the client. Alternatively, use `middleware.RecoverWith` with a custom handler or override with `middleware.RequestLogger` + a custom `LogFormatter`.

### 10. Docker images tagged `:latest` for monitoring stack (unpinned)

**File:** `docker-compose.monitoring.yml`

Four images use `:latest` tags:
- `otel/opentelemetry-collector-contrib:latest`
- `grafana/loki:latest`
- `prom/prometheus:latest`
- `grafana/grafana:latest`

Additionally, `edoburu/pgbouncer:latest` in docker-compose.services.yml (production profile).

`:latest` tags are mutable — they can be overwritten at any time. A compromised registry or supply chain attack could replace these images. Even without malicious intent, a breaking change in an upstream image can break the deployment silently.

Note: `grafana/tempo:2.6.1` is properly pinned — follow that pattern.

**Impact:** Supply chain risk — unpinned images can introduce vulnerabilities or breaking changes without notice.

**Recommendation:** Pin all images to specific version tags or SHA256 digests.

### 11. Frontend dependencies use caret ranges (`^`) in package.json

**File:** `frontend/package.json`

All dependencies use caret ranges (e.g., `"react": "^19.2.4"`), which allow automatic minor and patch version bumps. While `package-lock.json` exists and pins exact versions for reproducible installs via `npm ci`, the loose ranges in `package.json` mean that any developer running `npm install` (or removing `package-lock.json`) could pull in different versions.

**Impact:** Low-to-medium — `package-lock.json` mitigates this for CI (`npm ci` is used correctly). Risk is limited to developer workstations or accidental lock file deletion.

**Recommendation:** This is standard npm practice and largely mitigated by the lockfile + `npm ci` in CI. For maximum rigor, consider using exact versions for critical dependencies (React, Zod).

---

## Low

### 12. No image signing or provenance attestation in CD pipeline

**File:** `.github/workflows/cd.yml`

The CD pipeline builds and pushes Docker images to GHCR but does not:
- Sign images with cosign or Notation
- Generate SBOM (Software Bill of Materials)
- Attach provenance attestations

Anyone with GHCR read access can pull images, but there is no way to verify that an image was built from a specific commit by the CI system.

**Impact:** Cannot verify image authenticity. A compromised registry or account could push tampered images.

**Recommendation:** Add cosign signing step after `docker/build-push-action`. Consider enabling buildx provenance attestations (`provenance: true` in the build action).

### 13. `seed-test` Dockerfile runs as root

**File:** `tools/seed-test/Dockerfile`

The seed-test container has no `USER` directive and runs as root. While this is a test utility (not exposed in production), it connects to the database and could be used as a privilege escalation vector if the Docker socket or network is compromised.

**Impact:** Low — test-only container, but inconsistent with the security posture of all other Dockerfiles which create and use a non-root `appuser`.

**Recommendation:** Add `RUN adduser -D -u 1001 appuser` and `USER appuser` for consistency.

### 14. No `IdleTimeout` on HTTP servers

**All services** configure `ReadHeaderTimeout`, `ReadTimeout`, and `WriteTimeout` but do not set `IdleTimeout`. Go's default `IdleTimeout` is 0 (no timeout), meaning idle keep-alive connections persist indefinitely.

**Impact:** Resource exhaustion under slowloris-style attacks that open connections and hold them idle.

**Recommendation:** Set `IdleTimeout: 60 * time.Second` (or similar) on all HTTP servers.

---

## Info

### 15. `unsafe-inline` in CSP `script-src`

**File:** `infra/traefik/config/security.yml:30`

Already tracked in crypto audit (finding #7). Noted here for completeness as it relates to security header misconfiguration. The CSP allows `script-src 'self' 'unsafe-inline'`, which weakens XSS protection.

---

## Done Correctly

- **Non-root containers:** All 8 service Dockerfiles and the frontend Dockerfile create a dedicated `appuser` (UID 1001) and switch to it. The frontend Nginx image uses the built-in `nginx` user.
- **Multi-stage builds:** All Go Dockerfiles use multi-stage builds (builder → alpine), keeping the final image minimal with no compiler, source code, or build tools.
- **Docker resource limits:** All services have memory and CPU limits configured in docker-compose.
- **Health checks:** All services expose `/healthz` and have Docker healthchecks configured with proper intervals, timeouts, retries, and start periods.
- **Structured logging:** Services use `log/slog` (structured logging) rather than `fmt.Println`, preventing accidental data leakage through unstructured log formats.
- **Log rotation:** All Docker services configure `json-file` logging with `max-size: 10m` and `max-file: 3`.
- **Network segmentation:** Docker networks are properly separated (`data_network`, `app_network`, `monitoring_network`) with services only on the networks they need.
- **No ports exposed directly:** Services don't expose ports to the host — all traffic routes through Traefik (except postgres in the dev override, which is bound to `127.0.0.1` only).
- **Error handling:** The game-server error helpers (`writeLobbyError`, `writeRuntimeError`) return generic "internal error" for unmatched errors, not raw error messages. The `writeError` helper uses structured JSON.
- **JSON Schema validation:** All POST/PUT/DELETE endpoints use JSON Schema validation middleware, preventing malformed input.
- **Go dependency pinning:** `go.mod` files use exact versions (Go modules pin by default). `go.sum` files are committed, ensuring integrity verification.
- **Lockfile integrity:** `package-lock.json` exists and CI uses `npm ci` (which fails if lockfile is out of sync).
- **CI permissions:** CI workflows use `permissions: contents: read` (least privilege). CD uses `contents: read` + `packages: write` (minimum for GHCR push).
- **GitHub Actions pinned by major version:** Actions use `@v4`, `@v6`, etc. — standard practice for GitHub Actions (not SHA-pinned, but acceptable trade-off for readability).
- **Concurrency control:** All workflows use `concurrency` groups to prevent parallel runs.
- **WebSocket origin checking:** ws-gateway validates WebSocket upgrade origins against `ALLOWED_ORIGINS` env var. The `checkOrigin` function returns `true` only for explicitly allowed origins when the variable is set.
- **Test-only endpoints gated:** The `/auth/test-login` endpoint is only registered when `TEST_MODE=true`, with a double-check inside the handler.
- **Docker socket mounted read-only:** Traefik mounts the Docker socket as `:ro`.
- **Security headers configured:** Traefik applies `X-Content-Type-Options`, `X-XSS-Protection`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`, HSTS, and CSP via a file-based middleware applied to all routers.
