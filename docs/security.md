# Security

## Authentication

GitHub OAuth via `auth-service`. JWT tokens stored in HTTP-only cookies:

- `tf_session` -- access token (15 min TTL)
- `tf_refresh` -- refresh token (7 day TTL)

Single active session enforced via Redis key `session:active:{player_id}`.

## Ownership Checks (Broken Access Control prevention)

Every endpoint that takes `{playerID}` in the URL **must** verify the caller owns the resource:

```go
callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
if callerID != playerID {
    writeError(w, http.StatusForbidden, "forbidden")
    return
}
```

Rules:
- Apply on every handler where `playerID` identifies a private resource (settings, friends, DMs, unread counts)
- For room-scoped endpoints (`{roomID}`), verify participation via `store.IsRoomParticipant`
- If a resource is intentionally public (profiles, achievements), document it with a comment
- Never trust `player_id` from the request body -- always derive from `sharedmw.PlayerIDFromContext` (JWT context)

## Store-Level Authorization

Sensitive SQL queries must scope to the authorized identity:

```sql
-- GOOD: scoped to receiver
WHERE id = $1 AND receiver_id = $2

-- BAD: trusts handler-only check
WHERE id = $1
```

## Injection Prevention

- **SQL**: always parameterized queries (`$1`, `$2`). Never `fmt.Sprintf` for SQL.
- **Redis**: build keys via helper functions (`RoomChannelKey`, `PlayerChannelKey`), never from raw user input.
- **Error responses**: generic messages ("forbidden", "not found"). Never reflect user input or internal details.

## Middleware Stack

All requests go through shared middleware (`shared/middleware/`):

| Middleware | Purpose |
|-----------|---------|
| `auth.go` | JWT verification, context injection (PlayerID, Username, Role) |
| `validate.go` | JSON Schema request validation (returns 400 with structured errors) |
| `body_limit.go` | Request size limits via `http.MaxBytesReader` |
| `recoverer.go` | Panic recovery, logs stack trace, returns generic 500 |

## Traefik Security Headers

Defined in `infra/traefik/config/security.yml`:

- Content-Security-Policy (CSP)
- HSTS with preload
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin
- Robots: noindex, nofollow

## Rate Limiting

Two layers:
1. **Traefik** -- per-service rate limits (configured via labels in docker-compose)
2. **Redis** -- sliding window rate limiter (Lua script) for game moves and auth endpoints

See [api/routing.md](api/routing.md) for per-service rate limit values.
