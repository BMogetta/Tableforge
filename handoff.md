# Tableforge — Session Handoff

## What Is This Project

Tableforge is a multiplayer board game platform. Players authenticate via GitHub OAuth, create or join rooms, and play turn-based games in real time via WebSockets. The only game currently implemented is TicTacToe, but the engine is pluggable.

**Stack:**
- Backend: Go, Chi router, PostgreSQL (pgx), Redis (rate limiting), OpenTelemetry
- Frontend: React + TypeScript, Vite, React Query, Zustand, React Router, CSS Modules
- Infrastructure: Docker Compose, Caddy (reverse proxy), Cloudflare Tunnel for TLS
- Tests: Playwright E2E (no unit tests yet)

---

## Architecture Overview

```
Browser → Caddy (:80) → frontend (nginx :3000)
                      → game-server (:8080)
                           ├── /api/v1/*     HTTP handlers
                           ├── /auth/*       GitHub OAuth
                           └── /ws/rooms/:id WebSocket hub
```

**WebSocket flow:** One socket per room, opened in `Room.tsx`, kept alive through `Room → Game` navigation via Zustand store. Closed only when `leaveRoom()` is called.

**State management:** Zustand holds `player`, `socket`, and `pendingRematch`. React Query manages server state (sessions, rooms, leaderboard).

**Navigation from WS events:** Use `RematchNavigator` component in `App.tsx` that watches `pendingRematch` in Zustand and calls `navigate()` from within the React tree — never call `navigate()` directly from a socket handler.

---

## What Was Implemented (This Session)

### Socket Leak Fix
- `leaveRoom()` now called before every navigation away from Room/Game pages
- Closes the WebSocket cleanly

### Surrender Flow
- `POST /api/v1/sessions/:id/surrender` — records forfeit, opponent wins
- `SurrenderModal` component with confirm/cancel
- `← Lobby` button mid-game opens modal; post-game navigates directly

### Rematch Flow
- `POST /api/v1/sessions/:id/rematch` — vote system, creates new session when all vote
- `rematch_vote` WS event updates vote counter for non-voting player
- `rematch_started` WS event carries new session ID + vote counts
- Navigation via `setPendingRematch` → `RematchNavigator` in `App.tsx`
- State reset on `sessionId` change via `useEffect([sessionId])`

### Room Ownership Transfer
- `LeaveRoom` in `lobby.go` now returns `LeaveResult{NewOwnerID, RoomClosed}`
- If owner leaves and others remain: `UpdateRoomOwner` to earliest `joined_at`
- If last player leaves: `DeleteRoom` (marks room as `finished`, clears players)
- WS events: `owner_changed` (with `owner_id`) and `room_closed`
- `Room.tsx` handles both events: updates `ownerId` state, refreshes view, redirects on close
- `RoomView.players` now returns `[]RoomViewPlayer` (full Player fields + seat + joined_at) instead of `[]RoomPlayer`

### Room Cleanup After Game Ends
- `ApplyMove`, `Surrender`, and `TurnTimer.applyLoseGame` now call `UpdateRoomStatus(finished)` after `FinishSession`
- Finished rooms no longer appear in `ListWaitingRooms`

### Rate Limiting in Tests
- `TEST_MODE=true` env var disables Redis rate limiter in game-server
- `VITE_TEST_MODE=true` build arg enables `/test/error` route in frontend

### Test Infrastructure
- Shared helpers extracted to `frontend/tests/e2e/helpers.ts`: `createPlayerContexts`, `setupAndStartGame`, `playFullGame`, `PLAYER1_STATE`, `PLAYER2_STATE`
- Playwright projects split: `game-tests` → `leaderboard-tests` (sequential dependency), `chromium-parallel` (auth tests, parallel), 2 workers

### New E2E Tests Added
- `game.spec.ts`: rematch, surrender, back to lobby, occupied cell disabled
- `lobby.spec.ts`: player leaving disables start, owner transfer, last player closes room, room disappears after game
- `leaderboard.spec.ts`: wins increment after completed game
- `auth.spec.ts`: unauthenticated redirect, admin access denied, error boundary
- `spectator.spec.ts`: fully commented, blocked pending feature

### Bug Fixes
- `ErrorBoundary` moved inside `BrowserRouter` so navigate works correctly
- `data-testid="leaderboard-row"` added to `<tr>` in `LeaderboardTable`
- `data-testid="player-username"` added to username span in Lobby header
- `data-testid="leaderboard-table"` already present, `leaderboard-row` was missing
- CSP updated in Caddyfile: added `fonts.googleapis.com` to `style-src`, `fonts.gstatic.com` to `font-src`

---

## Current Test Status

**20 tests, all passing.**

Projects:
- `setup` → `game-tests` (game.spec + lobby.spec, serial)
- `game-tests` → `leaderboard-tests` (serial dependency)
- `setup` → `chromium-parallel` (auth.spec, parallel)

Run tests:
```bash
make down && make up-test
docker compose exec game-server /bin/seed-test > frontend/tests/e2e/.players.json
make test
```

---

## Backlog (Prioritized)

### In Progress / Next Up
1. **First mover policy** — configurable per room: `random`, `fixed` (seat 0), `winner_first`, `loser_first`, `winner_chooses`, `loser_chooses`. Affects `StartGame` and `VoteRematch` in lobby.go. Frontend needs UI in Room.tsx to expose choice when policy is `winner_chooses`.

2. **Spectator mode** — `spectator_links` table exists, `CreateSpectatorLink`/`GetSpectatorLink` in store, `allow_spectators bool` on Room. Missing: HTTP endpoints, WS spectator channel, frontend UI. Test file exists at `spectator.spec.ts` (fully commented).

3. **In-game chat** — no backend or frontend yet. Likely a new WS event type `chat_message` with payload `{player_id, text, timestamp}`.

4. **Friends list** — no backend or frontend yet. Requires a new `friendships` table, invite flow, and lobby UI.

### Known Issues
- `TurnTimer` has no persistence — server restart mid-game loses active timers
- Page refresh on `/game/:id` shows "Connecting..." banner briefly even if socket connects immediately
- Reconnection test not written yet — blocked by the banner flash issue

### Cosmetic / Low Priority
- CSP violation for Google Fonts appears in Playwright logs (fixed in Caddy, may still show in some contexts)
- Open rooms don't disappear from lobby in real time when a game starts — they disappear on the next 10s poll

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `internal/lobby/lobby.go` | Room lifecycle: create, join, leave, start, ownership transfer |
| `internal/runtime/runtime.go` | Move validation, game over, surrender, rematch voting |
| `internal/runtime/turn_timer.go` | Per-session turn countdown, timeout penalties |
| `internal/ws/hub.go` | WebSocket hub, broadcast, event type constants |
| `internal/api/api.go` | HTTP handlers, routes |
| `internal/store/store.go` | Store interface + all models |
| `frontend/src/store.ts` | Zustand: player, socket, pendingRematch |
| `frontend/src/pages/Game.tsx` | Game UI, socket events, rematch, surrender |
| `frontend/src/pages/Room.tsx` | Room UI, socket events, ownership |
| `frontend/src/App.tsx` | Routes, ErrorBoundary, RematchNavigator |
| `frontend/tests/e2e/helpers.ts` | Shared test helpers |

---

## Code Style Guidelines

**Go:**
- Errors wrapped with `fmt.Errorf("FunctionName: context: %w", err)`
- Store methods are thin — no business logic, just SQL
- Business logic lives in `lobby.go` or `runtime.go`
- New WS event types added as constants in `hub.go`
- HTTP handlers are standalone functions `func handleX(dep1, dep2) http.HandlerFunc`
- Soft deletes preferred over hard deletes for auditing; use `finished` status for rooms/sessions

**TypeScript/React:**
- All code and comments in English
- CSS Modules for component styles, global utility classes in `global.css`
- `data-testid` on every interactive or observable element
- Never call `navigate()` from a WebSocket event handler — use `setPendingRematch` pattern or similar Zustand state, handled by a always-mounted navigator component
- State that depends on `sessionId` must reset via `useEffect([sessionId])`
- Socket cleanup: always return `() => off()` from useEffect, never `() => off`
- React Query for server state, Zustand for client/socket state — don't mix

**Tests:**
- Shared helpers in `helpers.ts`, never import from other spec files
- Every new interactive element needs a `data-testid`
- Use `toPass({ timeout })` for async DOM polling, never `waitForTimeout` unless polling an external system (e.g. lobby 10s poll)
- Tests use pre-seeded players from `seed-test` — always run `make seed-test` before `make test` in a fresh environment
- Rematch and reconnection tests require `--headed --debug` for investigation