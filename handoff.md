# Tableforge — Session Handoff

## Critical Rules (always apply)
- All code, comments, and documentation **must be in English**
- CSS modules for all component styles
- `credentials: 'include'` on all fetch calls (cookie auth)
- `.env.example` always shown as markdown, never as a file download
- `expose:` not `ports:` for internal services in prod compose
- No bullet points in prose responses

---

## Project Overview

Tableforge is a multiplayer board game platform. Monorepo structure:
- `server/` — Go backend (chi router, pgx, JWT auth, WebSocket hub)
- `frontend/` — React + TypeScript + Vite + CSS Modules + Zustand + TanStack Query
- `infra/` — Caddy reverse proxy, Docker Compose

---

## Stack

**Backend:** Go 1.23, chi, pgx/v5, golang-jwt, Redis (rate limiting), Docker  
**Frontend:** React 18, TypeScript, Vite, CSS Modules, Zustand, TanStack Query v5, React Router v6  
**Testing:** Playwright (E2E), test auth bypass via `TEST_MODE=true`  
**Infra:** Caddy, Docker Compose, Prometheus + Grafana + Jaeger

---

## Architecture Notes

### Auth
- GitHub OAuth → JWT cookie → middleware loads player from DB on every request
- Role hierarchy: `player < manager < owner`
- `RequireRole` guard in `App.tsx` for frontend route protection
- **Test bypass:** `GET /auth/test-login?player_id=<uuid>` — only active when `TEST_MODE=true`

### WebSocket
- `RoomSocket` in `frontend/src/ws.ts` handles connection, reconnect, and status events
- Emits: `ws_connected`, `ws_reconnecting`, `ws_disconnected` (internal), plus game events
- Socket instance lives in Zustand (`useAppStore`) — created in `Room.tsx` via `joinRoom()`, survives Room→Game navigation
- `Game.tsx` subscribes to the same socket instance without closing it on unmount
- `leaveRoom()` in Zustand closes the socket — should be called when navigating back to lobby (currently NOT called anywhere — socket leaks)
- On page refresh in `/game/:id`, `Game.tsx` calls `joinRoom(session.room_id)` if socket is null

### Rate Limiting
- Redis sliding window, per-IP
- Global: 100 req/min | Auth: 10 req/min | Moves: 60 req/min | Admin: 30 req/min
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- Fail-open on Redis errors

### Turn Timer
- `TurnTimer` in `server/internal/runtime/turn_timer.go`
- Started when a game session is created, reset after every move
- Penalty configured per game in `game_configs` table: `lose_turn` or `lose_game`
- Broadcasts `game_over` or `move_applied` via WebSocket on timeout
- **No persistence** — server restart mid-game loses all active timers. Sessions with `finished_at = NULL` should reschedule on startup (not yet implemented)

### React Query
- `QueryClient` in `frontend/src/queryClient.ts`
- Key factory: `keys.rooms()`, `keys.session(id)`, `keys.leaderboard()`, etc.
- WebSocket events update cache directly via `qc.setQueryData` — no refetch needed
- `Game.tsx` also polls every 3s as a fallback for missed WS events — polling stops when `finished_at` is set
- Zustand retained only for authenticated player global state and socket instance

### GET /sessions/:id Response Shape
Returns `{ session, state, result }` where `result` is non-null only when `finished_at` is set.
Used by `Game.tsx` on mount and on polling to detect game-over state without relying on WS.

---

## Database Migrations

| File | Description |
|------|-------------|
| `001_initial.sql` | Core tables: players, rooms, room_players, game_sessions, moves |
| `002_roles.sql` | `player_role` enum, roles on players and allowed_emails |
| `003_turn_timeout.sql` | `game_configs` table, `turn_timeout_secs` + `last_move_at` on game_sessions |

---

## Key Files

### Backend
| File | Purpose |
|------|---------|
| `internal/store/store.go` | All models and Store interface |
| `internal/store/pg_store.go` | PostgreSQL implementation |
| `internal/auth/middleware.go` | JWT validation, role injection into context |
| `internal/auth/test_auth.go` | Test-only login bypass |
| `internal/api/api.go` | All HTTP routes and handlers |
| `internal/lobby/lobby.go` | Room lifecycle: create, join, leave, start |
| `internal/runtime/runtime.go` | Move validation, application, game-over detection |
| `internal/runtime/turn_timer.go` | Per-session turn countdown, timeout penalty |
| `internal/ratelimit/ratelimit.go` | Redis sliding window rate limiter |
| `internal/ws/hub.go` | WebSocket hub, broadcast by room |
| `cmd/seed/main.go` | Owner bootstrap — waits for first login then promotes |
| `cmd/seed-test/main.go` | Creates two test players, prints JSON with IDs |

### Frontend
| File | Purpose |
|------|---------|
| `src/main.tsx` | App entry, `QueryClientProvider` wrapper |
| `src/queryClient.ts` | Shared `QueryClient` + key factory |
| `src/store.ts` | Zustand — authenticated player + global RoomSocket instance |
| `src/api.ts` | All API functions, typed responses |
| `src/ws.ts` | `RoomSocket` with connection status events |
| `src/App.tsx` | Routes, `RequireAuth`, `RequireRole`, `ErrorBoundary` |
| `src/pages/Lobby.tsx` | Room list, create/join — uses React Query |
| `src/pages/Room.tsx` | Waiting room, WS connection banner |
| `src/pages/Game.tsx` | Game board, moves — uses React Query + WS |
| `src/pages/Admin.tsx` | Email whitelist + player role management + observability iframes |

### Infra
| File | Purpose |
|------|---------|
| `infra/caddy/Caddyfile` | Reverse proxy, security headers, WS upgrade |
| `docker-compose.yml` | All services: server, frontend, postgres, redis, caddy, grafana, jaeger, prometheus |

### Testing
| File | Purpose |
|------|---------|
| `frontend/playwright.config.ts` | Playwright config, setup project for auth |
| `frontend/tests/e2e/auth.setup.ts` | Creates `.auth/player1.json` and `.auth/player2.json` |
| `frontend/tests/e2e/lobby.spec.ts` | Lobby: load, create room, two players join |
| `frontend/tests/e2e/game.spec.ts` | Full TicTacToe game + timeout test |

---

## Makefile Commands

```bash
make up           # start in dev mode
make up-test      # start with TEST_MODE=true (required for Playwright)
make down         # stop containers
make seed         # bootstrap owner (OWNER_EMAIL=... make seed)
make seed-test    # create test players → frontend/tests/e2e/.players.json
make test         # run Playwright tests
make test-ui      # Playwright UI mode
make clean-test   # remove test fixtures and auth state
make clean        # stop containers and wipe DB volumes
```

### Full test workflow
```bash
make up-test
make seed-test
make test
```

---

## Pending Work

### Features
- [ ] **Rematch flow** — backend ready (`UpsertRematchVote`, `ListRematchVotes`), no frontend UI
- [ ] **Spectator mode** — store methods exist, no endpoints or UI
- [ ] **JWT session revocation** — no way to invalidate active sessions on logout
- [ ] **sessionStorage cache for player** — avoid `/auth/me` round-trip on every refresh
- [ ] **Chat** — see notes below

### Known Issues
- [ ] `leaveRoom()` in Zustand is never called — socket stays open when user navigates back to lobby. Should be called from the "Back to Lobby" button in `Game.tsx` and the "Leave" button in `Room.tsx`
- [ ] `TurnTimer` has no persistence — server restart mid-game loses all active timers. On startup, sessions with `finished_at = NULL` and a configured timeout should be rescheduled
- [ ] Page refresh on `/game/:id` reconnects correctly but loses the connection banner state (always shows "connecting" briefly even if socket connects immediately)
- [ ] Turn timeout Playwright test is sensitive to `game_configs.default_timeout_secs` — currently 30s in DB, test timeout set to 35s to accommodate

### Production Hardening
- [ ] DB backups
- [ ] Watchtower rollback strategy
- [ ] Health check endpoints
- [ ] Observability iframes — Grafana works via subpath; Jaeger v2 has limited subpath support, recommend external links

### Testing — Next Playwright Scenarios
- [ ] **Player wins a game** — assert winner sees "You won!" and loser sees "You lost." with correct CSS class applied (currently covered only via the full game test, but worth isolating)
- [ ] **Player leaves room before game starts** — P2 joins, then leaves; P1's start button should become disabled again and player count should update
- [ ] **Reconnection during game** — simulate P1 going offline briefly (close/reopen page), assert game state is restored correctly and moves can resume
- [ ] **Invalid move** — attempt to click an already-filled cell; assert error message appears and turn does not advance
- [ ] **Back to Lobby button** — after game ends, click "Back to Lobby", assert redirect to `/` and socket is closed (`leaveRoom` fix required first)
- [ ] **Rematch flow** — once frontend UI is built, test both players voting for rematch and a new game starting

---

## Chat — Design Notes

The WebSocket infrastructure already supports adding chat with minimal backend changes.

**Decision needed first:** ephemeral (WS only, no persistence) vs persistent (stored in `chat_messages` table, loaded on join). Ephemeral is significantly simpler for an MVP.

**Backend changes needed:**
- Add `EventType = "chat_message"` to `ws/hub.go`
- `readPump` in `ws/client.go` currently discards all incoming client messages — needs to parse and re-broadcast chat messages
- Optional: `POST /rooms/:id/chat` endpoint if server-side validation or persistence is needed

**Frontend changes needed:**
- Subscribe to `chat_message` events in the global socket
- Chat state can live in Zustand (persists across Room→Game) or local to a shared layout component
- UI: input + scrollable message list, shown in both `Room.tsx` and `Game.tsx`

---

## Notes for Next Session

- `game_configs` is seeded in migration `003` with `default_timeout_secs = 30` for tictactoe. The Playwright timeout test uses a 35s assertion timeout to accommodate this. Do not lower it without updating the test.
- `GameRenderer` in `Game.tsx` is a switch on `game_id` — add new cases there for new game renderers.
- `ws.ts` event types `ws_connected`, `ws_reconnecting`, `ws_disconnected` are client-only and never sent by the server.
- For new games, insert a row in `game_configs` matching the `game.ID()` return value, then add a case to `GameRenderer`.
- The global socket pattern: `joinRoom(roomId)` → Zustand stores `RoomSocket` → both `Room.tsx` and `Game.tsx` read from store. Cleanup via `leaveRoom()` is not yet wired to navigation events.