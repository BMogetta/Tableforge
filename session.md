# Tableforge — Session Handoff
## Critical Rules (always apply)
- All code, comments, and documentation **must be in English**
- CSS modules for all component styles
- `credentials: 'include'` on all fetch calls (cookie auth)
- `.env.example` always shown as markdown, never as a file download
- `expose:` not `ports:` for internal services in prod compose
- No bullet points in prose responses
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
- `Room.tsx` shows a connection status banner based on socket state

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

### React Query
- `QueryClient` in `frontend/src/queryClient.ts`
- Key factory: `keys.rooms()`, `keys.session(id)`, `keys.leaderboard()`, etc.
- WebSocket events update cache directly via `qc.setQueryData` — no refetch
- Zustand retained only for authenticated player global state

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
| `src/store.ts` | Zustand — authenticated player only |
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

### From original backlog
- [ ] **Rematch flow** — backend ready (`UpsertRematchVote`, `ListRematchVotes`), no frontend UI
- [ ] **Spectator mode** — store methods exist, no endpoints or UI
- [ ] **JWT session revocation** — no way to invalidate active sessions on logout
- [ ] **sessionStorage cache for player** — avoid `/auth/me` round-trip on every refresh

### Production hardening
- [ ] DB backups
- [ ] Watchtower rollback strategy
- [ ] Health check endpoints
- [ ] Observability iframes — Grafana works via subpath; Jaeger v2 has limited subpath support, recommend external links

### Testing
- [ ] Playwright tests not yet verified running end-to-end (setup complete, needs first run)
- [ ] Turn timeout Playwright test requires `default_timeout_secs=10` in `game_configs` for tictactoe
- [ ] No backend unit tests yet

### Nice to have
- [ ] Turn countdown timer UI (show remaining seconds to player)
- [ ] `turn_timeout_secs` field exposed in room creation UI (frontend form)
- [ ] `gameConfig.get()` called in Lobby to show timeout info per game

---

## Seeder Behavior

`cmd/seed/main.go` — blocking, waits up to 10 minutes for owner to log in:
1. Adds `OWNER_EMAIL` to `allowed_emails` with `role = owner`
2. Polls every 5s for the player record (created on first GitHub login)
3. Once found, promotes to owner and exits

In `docker-compose.yml` set `restart: "no"` for the seeder service.

---

## Turn Timeout Flow

1. Room created with optional `turn_timeout_secs`
2. `lobby.StartGame` resolves effective timeout: room value → clamped to game config min/max → game default
3. `runtime.Service.StartSession(session)` called from `handleStartGame` → schedules `TurnTimer`
4. After every move: `TurnTimer.Schedule(session)` resets the timer
5. On timeout: `TurnTimer.onTimeout` fires, reads `game_configs.timeout_penalty`
   - `lose_game`: calls `FinishSession`, creates `GameResult` with `ended_by = timeout`, broadcasts `game_over`
   - `lose_turn`: advances `CurrentPlayerID` to next player, broadcasts `move_applied`, reschedules timer
6. Normal game end: `TurnTimer.Cancel(sessionID)` called before `FinishSession`

---

## Error Boundary

`App.tsx` wraps all routes in `ErrorBoundary` (class component).  
On render error: shows game-themed error screen with "Try Again" (resets boundary) and "Go to Lobby" (hard redirect).

---

## Notes for Next Session

- `game_configs` is seeded with tictactoe defaults in migration `003`. For new games, insert a row in `game_configs` matching the `game.ID()` return value.
- `GameRenderer` in `Game.tsx` is a switch on `game_id` — add new cases there for new game renderers.
- `ws.ts` event types include `ws_connected/ws_reconnecting/ws_disconnected` — these are client-only and not sent by the server.
- The `TurnTimer` has no persistence — if the server restarts mid-game, active timers are lost. On restart, sessions with `finished_at = NULL` and a configured timeout should reschedule. This is not yet implemented.