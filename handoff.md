# Tableforge — Session Handoff

## Stack
Go 1.23, chi, pgx/v5, Redis, JWT auth, WebSocket hub, Vite/React frontend.
All services run via `docker compose`. E2E tests use Playwright with 3 pre-seeded test players.

## Current state
**65/65 E2E tests passing** (pending 1 presence fix below).

---

## What was built this session

### 1. Session History & Replay (`/sessions/:id/history`)
- `GET /sessions/:id/events` — reads from Redis Stream (active) or Postgres (finished)
- `pages/SessionHistory.tsx` + `SessionHistory.module.css`
- Event log tab: timestamped events, expandable payloads
- Replay tab: slider + board reconstruction from `state_after` (base64 → JSON)
- "View Replay" button in `Game.tsx` post-game (`handleViewReplay` calls `leaveRoom()` then navigates)
- Route: `/sessions/:sessionId/history` in `App.tsx`
- Fix: `SessionHistory` uses `staleTime: 0, gcTime: 0` to avoid stale cache
- Fix: `state_after` is base64-encoded `[]byte` from Go — decode with `atob()` then `JSON.parse()`
- Fix: `isDraw` uses `result?.is_draw` not `result?.status === 'draw'`
- `GameResult` interface in `api.ts` updated to include `is_draw?: boolean`

### 2. Presence (Redis SETEX + heartbeat)
- `internal/presence/presence.go` — `Set`, `Del`, `IsOnline`, `ListOnline`
- `ws/client.go` — presence heartbeat every 15s in `writePump`, `Del` on disconnect in `readPump` defer
- `ws/handler.go` — on participant connect:
  1. `ps.Set` + broadcast `presence_update { player_id, online: true }` to all room clients
  2. Send presence snapshot (all players' current status) directly to `c.send` of the connecting client only
  3. After `readPump` returns: `presence_update { online: false }` broadcast
- `ws/hub.go` — `EventPresenceUpdate EventType = "presence_update"` added
- `store.ts` — `presenceMap: Record<string, boolean>`, `setPlayerPresence`, reset on `joinRoom`/`leaveRoom`
- `ws.ts` — `'presence_update'` added to `WsEventType`
- `Game.tsx` — reads `presenceMap`, derives `opponentId` from `TicTacToeState.players`, shows dot + text
- `Room.tsx` — presence dot per player in player list

### 3. Observability
- Loki + Promtail added to `docker-compose.yml`
- Grafana datasource provisioned for Loki
- Full stack: Jaeger (traces) + Prometheus (metrics) + Loki (logs), all in Grafana

---

## Pending fix (1 failing test)

**Test:** `presence.spec.ts` — "opponent presence text updates correctly"  
**Cause:** Text node not wrapped in `<span data-testid="opponent-presence-text">`, and `data-online` passed as boolean instead of string (React drops `false` boolean attributes).

**Fix in `Game.tsx`:**
```tsx
{!isSpectator && opponentId && (
  <span className={styles.opponentPresence} data-testid="opponent-presence">
    <span
      className={styles.presenceDot}
      data-online={String(opponentOnline)}
      data-testid="opponent-presence-dot"
    />
    <span data-testid="opponent-presence-text">
      {opponentOnline ? 'Opponent online' : 'Opponent offline'}
    </span>
  </span>
)}
```

**Fix in `Room.tsx`** — same issue with boolean attribute:
```tsx
data-online={String(presenceMap[p.id] ?? false)}
```

---

## Pending: new Go package (zip)
User has a Go package built in another chat to integrate. Open with bash_tool, review structure, then integrate carefully respecting existing store interface and module path `github.com/tableforge/server`.

---

## Backlog (prioritized)
1. **Deuda técnica** — `runtime_test.go` and `pg_test.go`: `CreateGameSession` missing 5th arg (`nil`), `GetSessionAndState` returns 4 values now
2. **Session suspension** — `SuspendedAt` exists in schema, `ErrSuspended` in runtime, but no endpoint. Natural follow-up to presence: auto-suspend when opponent offline > X minutes
3. **ELO / rating system**
4. **Matchmaking by rating**
5. **Chat**
6. **Player profiles** (history, stats, avatar)

---

## Key architecture decisions (do not revert)
- `fakestore.go` implements full `store.Store` interface — keep in sync on every store change
- Turn timer uses Redis TTL + keyspace notifications (`--notify-keyspace-events Ex` in docker-compose)
- Event sourcing: Redis Streams (active sessions) → Postgres `session_events` (finished)
- WS hub uses Redis pub/sub for multi-instance fanout (`NewHubWithRedis`)
- Spectator access via `room_settings.allow_spectators`, no SpectatorLink table
- Single migration file: `001_initial.sql`
- `state_after` in moves table is `[]byte` → base64 in JSON → decode with `atob()` on frontend