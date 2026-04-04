# JWT Refresh Token Architecture

## Problem

Access tokens (JWT) have a 7-day TTL and cannot be revoked. A banned user's JWT remains valid for HTTP API calls until it expires. Stolen tokens cannot be invalidated.

## Solution

Short-lived access tokens (15min) + long-lived refresh tokens (7 days) + server-side revocation.

---

## Token Types

### Access Token (JWT)
- **TTL:** 15 minutes
- **Cookie:** `tf_session` (HttpOnly, Secure, SameSite=Lax, Path=/)
- **Claims:** `sub` (player_id), `role`, `exp`, `jti` (unique token ID)
- **Validation:** signature + expiry only (fast, no DB/Redis lookup)

### Refresh Token
- **Format:** opaque UUID (NOT a JWT — no client-side decoding needed)
- **Storage:** `player_sessions` table (already exists, has `revoked_at` column)
- **Cookie:** `tf_refresh` (HttpOnly, Secure, SameSite=Strict, Path=/auth/refresh)
- **TTL:** 7 days
- **Rotation:** each refresh issues a new refresh token and invalidates the old one

---

## Database Changes

### `player_sessions` table (already exists)

```sql
-- Verify existing columns:
--   id UUID PRIMARY KEY
--   player_id UUID NOT NULL
--   revoked_at TIMESTAMPTZ
--   created_at TIMESTAMPTZ DEFAULT NOW()

-- Add if missing:
ALTER TABLE player_sessions ADD COLUMN IF NOT EXISTS
  expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days';

ALTER TABLE player_sessions ADD COLUMN IF NOT EXISTS
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Index for refresh token lookup
CREATE INDEX IF NOT EXISTS idx_player_sessions_player_id
  ON player_sessions(player_id) WHERE revoked_at IS NULL;
```

---

## API Changes

### auth-service

#### `POST /auth/github/callback` (modify)
After successful OAuth:
1. Create a `player_sessions` row → get `session_id`
2. Issue access token JWT (15min, `jti` = new UUID)
3. Issue refresh token (the `session_id`)
4. Set both cookies

#### `POST /auth/refresh` (new endpoint)
1. Read `tf_refresh` cookie
2. Look up session in `player_sessions` WHERE `id = refresh_token AND revoked_at IS NULL AND expires_at > NOW()`
3. If not found → 401, clear both cookies
4. Check if player is banned (query `bans` table)
5. If banned → revoke session, 403
6. **Rotate:** revoke current session, create new session row
7. Issue new access token + new refresh token cookies
8. Update `last_used_at`

Rate limit: 10 req/min per player (prevent token spray)

#### `POST /auth/logout` (modify)
1. Read `tf_refresh` cookie
2. Revoke the session (`UPDATE player_sessions SET revoked_at = NOW() WHERE id = $1`)
3. Clear both cookies

#### `POST /auth/test-login` (modify)
Same flow as callback — issue both tokens.

### shared/middleware

#### `Require()` (modify)
- Only validate JWT signature + expiry (no change to hot path)
- On expired JWT → return 401 with body `{"error": "token_expired"}` (distinct from `{"error": "unauthorized"}`)
- Frontend uses this distinction to trigger refresh

### user-service

#### Ban flow (modify)
When banning a player:
1. Revoke ALL active sessions: `UPDATE player_sessions SET revoked_at = NOW() WHERE player_id = $1 AND revoked_at IS NULL`
2. Publish `player.session.revoked` (existing Redis Pub/Sub — ws-gateway disconnects WS)
3. Within 15min, the access token expires and can't be refreshed

---

## Frontend Changes

### API interceptor (`frontend/src/lib/api.ts`)

Modify `request()` function:

```typescript
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res = await fetch(`/api/v1${path}`, {
    credentials: 'include',
    ...init,
  })

  // If access token expired, try refresh once
  if (res.status === 401) {
    const body = await res.json().catch(() => null)
    if (body?.error === 'token_expired') {
      const refreshRes = await fetch('/auth/refresh', {
        method: 'POST',
        credentials: 'include',
      })
      if (refreshRes.ok) {
        // Retry original request with new access token
        res = await fetch(`/api/v1${path}`, {
          credentials: 'include',
          ...init,
        })
      } else {
        // Refresh failed — session is dead, redirect to login
        window.location.href = '/login'
        throw new ApiError(401, 'Session expired')
      }
    }
  }

  if (!res.ok) throw new ApiError(res.status, await res.text())
  return res.json()
}
```

### WebSocket reconnection

When WS connection gets a close frame or fails to reconnect:
1. Try `/auth/refresh`
2. If successful, reconnect WS
3. If not, redirect to login

---

## Files to Modify

| File | Change |
|------|--------|
| `shared/middleware/auth.go` | Return `token_expired` error for expired JWTs |
| `services/auth-service/internal/jwt/jwt.go` | Add `jti` claim, reduce TTL to 15min |
| `services/auth-service/internal/handler/handler.go` | Modify callback/logout, add refresh endpoint |
| `services/auth-service/internal/store/store.go` | Add refresh token CRUD (may already exist) |
| `services/auth-service/cmd/server/main.go` | Mount `/auth/refresh` route |
| `services/user-service/internal/api/api_admin.go` | Revoke all sessions on ban |
| `frontend/src/lib/api.ts` | Add refresh interceptor |
| `frontend/src/lib/ws.ts` | Handle WS auth expiry |
| `shared/db/migrations/` | Add `expires_at`, `last_used_at` columns if missing |

## Migration

### Backwards compatibility
- Existing 7-day JWTs will continue to work until they expire (middleware still validates them)
- New logins get 15min access + 7d refresh
- After 7 days, all users are on the new system

### Rollback
- Revert TTL to 7 days in `jwt.go`
- Refresh endpoint becomes a no-op (harmless)
- Frontend interceptor retries silently fail (harmless)

---

## Security Properties

| Scenario | Before | After |
|----------|--------|-------|
| Ban | HTTP works 7 days | HTTP works max 15min |
| Stolen access token | Usable 7 days | Usable max 15min |
| Stolen refresh token | N/A | Detected on rotation (old token revoked) |
| Logout | Cookie cleared, token usable | Session revoked server-side |
| CSRF on refresh | N/A | SameSite=Strict on refresh cookie |
