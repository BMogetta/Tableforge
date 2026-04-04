# Security Audit — SSRF & Attack Surface (OWASP A09/A10)

**Date:** 2026-04-03
**Scope:** All 8 backend services + Traefik reverse proxy + frontend
**Methodology:** Static code review of HTTP clients, gRPC clients, routers, middleware, WebSocket config, query patterns

---

## Summary

| Category | Finding Count | Critical | High | Medium | Low |
|----------|:---:|:---:|:---:|:---:|:---:|
| SSRF | 1 | 0 | 0 | 1 | 0 |
| Rate Limiting | 2 | 0 | 1 | 1 | 0 |
| Body Size Limits | 1 | 0 | 1 | 0 | 0 |
| Unbounded Queries | 3 | 0 | 0 | 3 | 0 |
| DoS Vectors | 2 | 0 | 0 | 1 | 1 |
| Anti-Automation | 1 | 0 | 0 | 1 | 0 |

---

## High

### 1. No request body size limit on any service

**All services** use `json.NewDecoder(r.Body).Decode(&v)` without calling `http.MaxBytesReader` or `io.LimitReader` first. An attacker can send a multi-gigabyte JSON payload to any POST/PUT/DELETE endpoint, consuming memory until the container OOM-kills.

Go's `http.Server.ReadTimeout` (15s) provides partial protection by limiting transfer duration, but on a fast connection a large payload can still be fully read within that window.

**Affected:** Every service (game-server, auth-service, user-service, chat-service, match-service, notification-service, rating-service).

**Files:**
- `services/game-server/internal/platform/api/api.go:166` (`decodeJSON`)
- `services/user-service/internal/api/api.go:109` (`decodeJSON`)
- `services/chat-service/internal/api/api.go:67` (`decodeJSON`)
- `services/match-service/internal/api/api.go:110` (inline `json.NewDecoder`)
- `services/notification-service/internal/api/api.go:171` (inline `json.NewDecoder`)

**Recommendation:** Add a global chi middleware that wraps `r.Body` with `http.MaxBytesReader(w, r.Body, maxBytes)`. A reasonable default is 1 MB for API endpoints, 4 KB for WebSocket-adjacent handlers. This can be a single shared middleware in `shared/middleware/`.

### 2. No rate limiting on 6 of 8 services

Only **game-server** has application-level rate limiting (Redis sliding window). The remaining services rely solely on Traefik:

| Service | Traefik Rate Limit | App-Level Rate Limit |
|---------|:---:|:---:|
| game-server | 100 req/min, burst 50 | Yes (global 100/min + move-specific 60/min) |
| auth-service | 20 req/min, burst 10 | No |
| user-service | None | No |
| chat-service | None | No |
| ws-gateway | None | No |
| match-service | None | No |
| notification-service | None | No |
| rating-service | None | No |

Traefik rate limits use the default source (client IP from `X-Forwarded-For`), which is reasonable. However, services without Traefik rate limits have **zero throttling**. A single authenticated user can hammer chat-service or user-service endpoints without restriction.

**Recommendation:** Add Traefik-level rate limits to all service routers in `docker-compose.services.yml`. For sensitive write endpoints (send message, send friend request, create report), add per-user rate limits at the application level using the existing `ratelimit` package from game-server (promote it to `shared/middleware/ratelimit/`).

---

## Medium

### 3. GitHub avatar URL stored without validation (SSRF surface)

`auth-service` stores the `avatar_url` field from the GitHub API response directly into the database (`UpsertOAuthParams.AvatarURL`). This URL is later served to the frontend, which renders it as an `<img>` tag.

While there is no server-side fetch of this URL today (mitigating direct SSRF), the risk is:
- GitHub could theoretically return a non-HTTPS URL or a malicious redirect
- If any future service fetches avatar URLs server-side (thumbnailing, caching, image proxy), it becomes a full SSRF
- The CSP limits `img-src` to `self` + `data:` + `https://avatars.githubusercontent.com`, which mitigates client-side exploitation

**File:** `services/auth-service/internal/handler/handler.go:154-155`

**Recommendation:** Validate that the avatar URL matches `https://avatars.githubusercontent.com/*` before storing. This is a defense-in-depth measure.

### 4. DM and room message queries have no LIMIT

`GetRoomMessages` and `GetDMHistory` return all messages without pagination:

```go
// store_messages.go:27 — no LIMIT
SELECT ... FROM room_messages WHERE room_id = $1 ORDER BY created_at ASC

// store_messages.go:112 — no LIMIT
SELECT ... FROM direct_messages WHERE (sender_id = $1 AND receiver_id = $2) OR ...
```

A long-lived room or DM thread could accumulate thousands of messages. Loading them all in a single query wastes memory and bandwidth.

**Files:**
- `services/chat-service/internal/store/store_messages.go:27` (`GetRoomMessages`)
- `services/chat-service/internal/store/store_messages.go:112` (`GetDMHistory`)

**Recommendation:** Add cursor-based or offset pagination (e.g., `LIMIT 100` default, `?before=<id>` cursor). The `ListDMConversations` query is also heavy (3 CTEs with JOINs) but is bounded per-player, so lower priority.

### 5. Admin list endpoints have no LIMIT

Several admin-only queries return all rows without pagination:

- `ListPlayers` — all non-deleted players (`services/user-service/internal/store/store_admin.go:49`)
- `ListAllowedEmails` — all whitelisted emails (`store_admin.go:99`)
- `ListPendingReports` — all pending reports (`store_reports.go:41`)
- `ListReportsByPlayer` — all reports for a player (`store_reports.go:56`)
- `ListWaitingRooms` — all open rooms (`services/game-server/internal/platform/store/pg.go:209`)

While admin endpoints are gated behind `requireRole(manager)`, a compromised manager account or a growing user base can cause these to return very large result sets.

**Recommendation:** Add pagination (`LIMIT $1 OFFSET $2`) to all list endpoints. `ListWaitingRooms` is user-facing and should be paginated first.

### 6. `ListSessionMoves` returns all moves without LIMIT

`GET /api/v1/sessions/{sessionID}/history` calls `ListSessionMoves` which returns every move for a session. For games with hundreds of moves (possible in longer games), this is an unbounded query accessible to any authenticated user.

**File:** `services/game-server/internal/platform/store/pg.go:549`

**Recommendation:** Add a LIMIT parameter or default cap (e.g., 500 moves max).

### 7. WebSocket has no per-IP concurrent connection limit

`ws-gateway` has no cap on how many WebSocket connections a single IP or player can open. An attacker can open hundreds of connections, each consuming goroutines (ReadPump + WritePump), a `send` channel buffer (64 messages), and a Redis subscription.

The `maxMessageSize = 512` bytes limit on incoming messages is good, but the connection count itself is unbounded.

**File:** `services/ws-gateway/internal/handler/handler.go` (no check before `upgrader.Upgrade`)

**Recommendation:** Track connections per player ID in the hub. Reject new connections when a player exceeds a threshold (e.g., 5 concurrent connections). Consider a per-IP limit at the Traefik level as well.

### 8. No anti-automation on account creation

Account creation goes through GitHub OAuth, which provides some natural friction. However, there is no additional protection:
- No CAPTCHA or proof-of-work
- The `allowed_emails` whitelist is the only gate, which is good for now
- Auth-service has a Traefik rate limit (20/min), which provides basic protection

**Current risk:** Low while the whitelist is active. If the whitelist is ever removed for public signup, this becomes a bot registration vector.

**Recommendation:** Document the whitelist as a security control. If public signup is added later, add CAPTCHA or email verification flow.

---

## Low

### 9. Rate limiter fails open on Redis failure

When Redis is unavailable, the rate limiter in `game-server` fails open (allows all traffic):

```go
// middleware.go:75
if err != nil {
    // Redis unavailable — fail open to avoid blocking all traffic.
    next.ServeHTTP(w, r)
    return
}
```

This is a reasonable availability-over-security tradeoff for a game platform, but should be documented and monitored. An attacker who can cause Redis instability (e.g., through connection exhaustion) would bypass all rate limits.

**File:** `services/game-server/internal/platform/ratelimit/middleware.go:75`

**Recommendation:** Add a Prometheus counter for rate-limit bypass events so this can trigger alerts. Consider a local in-memory fallback limiter.

---

## Done Correctly

### SSRF: No user-controlled URLs in server-side requests

All `http.NewRequestWithContext` calls in `auth-service` target hardcoded GitHub API URLs (`github.com/login/oauth/access_token`, `api.github.com/user`, `api.github.com/user/emails`). No endpoint accepts a user-provided URL to fetch.

### SSRF: gRPC targets are environment-configured, not user-controlled

All `grpc.NewClient` calls use addresses from environment variables or hardcoded defaults (e.g., `user-service:9082`, `game-server:9080`). No user input reaches gRPC dial targets.

### SSRF: No file upload endpoints

No service accepts file uploads. Avatar URLs come from GitHub and are stored as strings, not fetched server-side.

### SSRF: OAuth state validation is correct

The OAuth flow uses a cryptographically random state parameter stored in an HttpOnly cookie, validated on callback. No open redirect via `redirect_uri` — the redirect is hardcoded to `/`.

### SSRF: gRPC ports not exposed via Traefik

gRPC ports (9080, 9082, 9085) are internal-network only. Traefik only proxies HTTP ports. The Docker network separation (`data_network` vs `app_network`) adds defense-in-depth.

### Attack surface: Security headers are comprehensive

Traefik applies security headers globally: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Strict-Transport-Security`, `Content-Security-Policy`, `Referrer-Policy`, `Permissions-Policy`. The CSP is restrictive and correctly scoped.

### Attack surface: HTTP server timeouts configured

All services set `ReadHeaderTimeout: 10s`, `ReadTimeout: 15s`, `WriteTimeout: 15s` (ws-gateway: 30s/60s for WS). This prevents slowloris and slow-read attacks.

### Attack surface: WebSocket origin checking implemented

`ws-gateway` validates `Origin` header against `ALLOWED_ORIGINS` env var. In production with `ALLOWED_ORIGINS` set, cross-origin WebSocket hijacking is prevented.

### Attack surface: WebSocket message size limited

`maxMessageSize = 512` bytes in `client.go:22` prevents clients from sending oversized messages. Combined with the read-only protocol (clients only send pong frames), the inbound attack surface is minimal.

### Attack surface: Container resource limits

All containers have memory and CPU limits in `docker-compose.services.yml` (256M / 0.5 CPU typical), preventing any single container from consuming all host resources.

### Attack surface: Presence endpoint capped

`PresenceHandler` caps the `ids` query parameter at 100 entries, preventing abuse of the Redis `MGET` call.

### Attack surface: Leaderboard pagination enforced

`rating-service` enforces `maxLeaderboardLimit = 500` and validates the `limit` parameter, preventing unbounded leaderboard queries.
