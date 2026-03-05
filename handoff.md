# Tableforge — Session Handoff

## Rules for the next session
- **Do not write code without reading the current file first.** Ask for the file, read it, propose changes, confirm, only then write.
- **One file at a time.** Move to the next only when the owner confirms the previous one is integrated and compiles.
- **Always deliver the complete file** for full replacement, never partial patches.
- **List proposed changes before writing** and wait for confirmation.
- **All code, comments, and documentation must be in English.** This includes inline comments, godoc, JSDoc, README files, and any other written artifact produced during the session.

---

## Stack
- **Backend:** Go, PostgreSQL (pgx), Redis, WebSockets
- **Frontend:** React + TypeScript, Vite, CSS Modules
- **Infra:** Docker Compose, Caddy (reverse proxy + Cloudflare tunnel)
- **Tests:** Playwright E2E

---

## Project status

### What works (base MVP)
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer
- Matches: TicTacToe fully implemented with turn timer, surrender, rematch vote
- WebSockets: hub with Redis pub/sub for multi-instance
- Admin: roles (player/manager/owner), allowed emails, leaderboard
- Spectator: `spectator_links` table exists, endpoints pending
- Rate limiting via Redis
- OpenTelemetry + Prometheus

### Recently integrated (this session) — Go compilation verification pending
All code was written and delivered. The owner must run `go build ./...` to confirm it compiles before the next session.

| File | Destination | Status |
|---|---|---|
| `005_room_settings.sql` | `db/migrations/` | Delivered |
| `lobby_settings.go` | `internal/engine/` | Delivered |
| `tictactoe_lobby_settings.go` | `games/tictactoe/` | Delivered |
| `store.go` | `internal/store/` | Delivered |
| `store_pg.go` | `internal/store/` | Delivered |
| `randutil.go` + `errors.go` | `internal/randutil/` (new package) | Delivered |
| `lobby.go` | `internal/lobby/` | Delivered |
| `api.go` | `internal/api/` | Delivered |
| `hub.go` | `internal/ws/` | Delivered |
| `api.ts` | `frontend/src/` | Delivered |
| `LobbySettings.tsx` + `.module.css` | `frontend/src/components/` | Delivered |
| `Room.tsx` | `frontend/src/pages/` | Delivered |
| `helpers.ts` | `frontend/tests/e2e/` | Delivered ✓ tests pass |
| `lobby.spec.ts` | `frontend/tests/e2e/` | Delivered ✓ tests pass |
| `leaderboard.spec.ts` | `frontend/tests/e2e/` | Delivered ✓ tests pass |

**Tests: 20/20 passing.**

---

## Next session — Tests for new features

The priority is writing E2E tests for everything that was implemented. Existing tests verify that the base system wasn't broken, but there is no coverage for the new features.

### Tests to write

#### `settings.spec.ts` (new file)
1. **Owner can change `first_mover_policy`** — create room, change selector to `fixed`, verify the change is reflected in p2's UI via WS (`setting_updated` event)
2. **`first_mover_seat` appears only when policy is `fixed`** — verify the field is not visible with `random`, appears with `fixed`
3. **`first_mover_seat` disappears when switching back to `random`** — change policy from `fixed` to `random`, verify the field disappears
4. **Non-owner sees settings as read-only** — p2 cannot edit, sees current values
5. **Invalid setting is rejected** — direct API call with invalid value, expect 400
6. **`first_mover_policy: fixed` makes seat 0 go first** — create room with fixed, start match, verify p1 has "Your turn"
7. **`first_mover_policy: random` eventually assigns p2 first** — probabilistic, can be omitted or tested with many iterations

#### `game.spec.ts` — add tests
8. **Turn order respects `first_mover_policy: fixed` with seat 1** — p2 goes first, p1 waits
9. **WS `setting_updated` updates the UI for all clients** — p1 changes setting, p2 sees the change without reloading

#### `settings.spec.ts` — API validations
10. **Non-owner cannot change settings** — direct call with p2's player_id in p1's room, expect 403
11. **Cannot change settings with room `in_progress`** — start match, attempt to change setting, expect 409

---

## Known technical debt

- **`rematch_first_mover_policy` is inert** — defined in migration and engine, but `VoteRematch` in `runtime.go` never reads it. Wire it up when the rematch flow is refactored.
- **`winner_chooses` / `loser_chooses`** — require an extra UI step (winner/loser chooses before starting). No WS event or screen defined yet.
- **`gameConfig.get` in `api.ts`** — calls `/games/{gameId}/config` which does not exist in the backend. Nobody calls it yet but it is waiting to break.
- **`seed-test` does not go through `lobby.Service`** — rooms from the seed have no rows in `room_settings`. Existing tests pass because they don't touch settings, but this must be fixed before writing settings tests that use `setupAndStartGame` without override.
- **`scanSession` incomplete** — `store_pg.go` does not scan `turn_timeout_secs` or `last_move_at`. Pre-existed this session.
- **`TurnTimer` has no Redis persistence** — if the server restarts mid-game, active timers are lost.
- **Rematch flow** — currently `VoteRematch` creates a new session directly. The decision is that it should return to the lobby in `waiting` state instead of starting the match automatically. This refactor is pending.

---

## Backlog (Prioritized)

### Immediate (post settings tests)
1. **E2E tests for settings** — see "Next session" section above
2. **Spectator** — `spectator_links` table exists, `CreateSpectatorLink`/`GetSpectatorLink` in store. Missing: HTTP endpoints, WS channel for spectators, UI. Test file at `spectator.spec.ts` (fully commented out).
3. **Rematch flow refactor** — on rematch vote, return to lobby in `waiting` instead of creating session directly. This unblocks `rematch_first_mover_policy`.

### Future features
4. **In-match chat** — no backend or frontend. New WS event `chat_message` with payload `{player_id, text, timestamp}`.
5. **Friends list** — `friendships` table, invite flow, lobby UI.
6. **ELO system** — `domain/rating`. Real-time leaderboard with Redis sorted sets. Calculated at match end via background job.
7. **Matchmaking** — queue in Redis sorted set by rating. Finds closest player with `ZRANGEBYSCORE`.
8. **Match replay** — events in Redis Stream (`XADD`) while active, persisted to Postgres on completion.
9. **Match pause** — `SuspendSession`/`ResumeSession` exist in store. Business logic and UI missing.
10. **More games** — engine is pluggable. Each game in `games/<name>/`, implements `engine.Game` and optionally `engine.LobbySettingsProvider`.
11. **Custom avatars** — S3-compatible object storage.
12. **Notifications** — email on invite, on match end. Via Asynq (background jobs over Redis).

### Structural refactor (post-MVP)
- **Domain-first structure** — migrate from packages by technical type (`store`, `api`, `runtime`) to packages by domain (`domain/lobby`, `domain/match`, `domain/rating`). Each domain defines its own store interface. `platform/store` implements all. Ideal moment: before adding ELO or matchmaking.
- **Split `api.go`** into files by domain: `api/lobby.go`, `api/match.go`, `api/rating.go`, etc.
- **Split `store.go` / `store_pg.go`** when it exceeds ~150 methods.

### Pending infrastructure
- **Session store in Redis** — sessions currently live in cookies. Redis with TTL avoids session loss on restarts.
- **Player presence** — `SETEX presence:<player_id> 30` renewed by WS heartbeat.
- **Idempotency keys** — `SET idempotency:<key> 1 NX EX 60` for moves and critical operations.
- **Background jobs (Asynq)** — for ELO, emails, replay processing.
- **Full observability** — Grafana + Loki + Tempo/Jaeger. OpenTelemetry and Prometheus already in place.
- **CDN for assets** — Cloudflare already used as tunnel, enable static asset caching.

### Discarded
- **Kafka** — Redis pub/sub + Asynq are sufficient.
- **GraphQL** — REST + WebSockets is the right choice.
- **Microservices** — domain-first monolith provides the necessary isolation without operational overhead.

---

## Architecture — decisions made

### Settings model
- `room_settings(room_id, key, value, updated_at)` — generic KV table
- `first_mover_policy`: `random` | `fixed` | `game_default`
- `rematch_first_mover_policy`: adds `winner_first` | `loser_first` | `winner_chooses` | `loser_chooses`
- `first_mover_seat`: seat index when policy is `fixed` (default `'0'`)
- Defaults inserted in `CreateRoom` via transaction, not in the migration
- Only editable during `waiting` (validated in lobby layer, not in DB)

### Engine interface
- `LobbySettingsProvider` — optional interface that games implement to declare custom settings
- `DefaultLobbySettings()` — platform settings, merged with game-specific ones
- TicTacToe implements the interface, returns defaults unchanged (extensible for board size, etc.)

### randutil package
- `internal/randutil` — everything related to randomness (crypto/rand based)
- `Intn(n int) (int, error)`, `Shuffle[T]`, `Pick[T]`
- Replaces the inline crypto/rand in `generateRoomCode`
- Foundation for future use: shuffle cards, draw card, dice

### Store
- `GetRoomSettings(roomID) (map[string]string, error)`
- `SetRoomSetting(roomID, key, value) error` — upsert with `ON CONFLICT`
- `CreateRoom` uses transaction: insert room + iterate DefaultSettings + commit

### Future folder structure (post-MVP)
```
server/
├── internal/
│   ├── domain/
│   │   ├── lobby/
│   │   ├── match/
│   │   ├── replay/
│   │   ├── rating/
│   │   ├── player/
│   │   └── spectator/
│   ├── engine/
│   ├── platform/
│   │   ├── store/
│   │   ├── ws/
│   │   ├── auth/
│   │   ├── ratelimit/
│   │   └── randutil/
│   └── api/
│       ├── router.go
│       ├── lobby.go
│       ├── match.go
│       └── ...
└── games/
    ├── registry.go
    └── tictactoe/
```

---

## Redis — current and planned uses

| Use | Status |
|---|---|
| Rate limiting | ✓ Implemented |
| Pub/sub for multi-instance WS | ✓ Implemented |
| Session store | Pending (before production) |
| Turn timer persistence (`SETEX`) | Pending (before production) |
| Player presence | With matchmaking |
| Matchmaking queue (sorted set) | With matchmaking |
| Real-time leaderboard (sorted set) | With ELO |
| Replay event sourcing (streams) | With replay |
| Idempotency keys | With real traffic |