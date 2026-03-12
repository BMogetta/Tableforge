# Tableforge — Session Handoff

## Rules for the next session
- **Do not write code without reading the current file first.** Ask for the file, read it, propose changes, confirm, only then write.
- **One file at a time.** Move to the next only when the owner confirms the previous one is integrated and compiles.
- **Always deliver the complete file** for full replacement, never partial patches.
- **List proposed changes before writing** and wait for confirmation.
- **All code, comments, and documentation must be in English.** This includes inline comments, godoc, JSDoc, README files, and any other written artifact produced during the session.

---

## Stack
- **Backend:** Go 1.23, chi, pgx/v5, Redis, JWT auth, WebSocket hub
- **Frontend:** React + TypeScript, Vite, CSS Modules, TanStack Query
- **Infra:** Docker Compose, Caddy (reverse proxy + Cloudflare tunnel)
- **Observability:** OpenTelemetry + Jaeger (traces) + Prometheus (metrics) + Loki/Promtail (logs), all in Grafana
- **Tests:** Playwright E2E (65 tests, all passing), Go unit tests (all passing)

---

## Project status

### What works
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer, room settings (first_mover_policy, visibility, spectators, turn timeout)
- Matches: TicTacToe with turn timer (Redis TTL + keyspace notifications), surrender, rematch vote
- Spectators: join/leave, count badge, blocked from rematch/moves
- Private rooms: code hidden in lobby, no join button, owner sees code
- WebSockets: hub with Redis pub/sub for multi-instance fanout
- Admin: roles (player/manager/owner), allowed emails, leaderboard
- Rate limiting via Redis
- Event sourcing: Redis Streams (active) → Postgres `session_events` (finished)
- Session history & replay: event log + board reconstruction slider
- Player presence: Redis SETEX + heartbeat, `presence_update` WS event
- Rating engine: multi-team generalised Elo with dynamic K-factor (fully tested)
- Matchmaking: queue-based with snake draft and spread relaxation (fully tested)
- Simulation CLI: `go run ./cmd/simulate` for validating rating system behaviour

---

## Package inventory

| Package | Path | Status |
|---|---|---|
| `mathutil` | `internal/platform/mathutil` | ✓ done |
| `rating` | `internal/domain/rating` | ✓ done + tested |
| `matchmaking` | `internal/domain/matchmaking` | ✓ done + tested |
| `simulate` | `cmd/simulate` | ✓ done |

### Test coverage — `internal/domain/rating` (15 tests)
- `MatchResult.Validate` — too few teams, invalid rank, empty team, ok
- `Team.EffectiveMMR` — normal, empty
- `DynamicK` — calibration (KMax), veteran (KMin), streak multiplier, upset multiplier
- `Engine` 1v1 — winner gains, zero-sum, draw, upset larger delta, games played, streak update, display rating convergence
- `Engine` multi-team — normalised deltas, zero-sum in 4-team match

### Test coverage — `internal/domain/matchmaking` (12 tests)
- Not enough players
- Basic 1v1, removes matched players, unmatched remain, multiple matches
- Quality gate rejects and accepts
- Spread relaxation allows large gap after wait, rejects without wait
- Snake draft produces equal team averages
- Quality in bounds, equal MMR → quality 1.0
- Concurrent enqueue (smoke test)

---

## Folder structure (current)

```
server/
├── cmd/
│   ├── server/
│   ├── seed/
│   ├── seed-test/
│   └── simulate/          ← ELO simulation CLI
├── db/
│   └── migrations/
│       ├── 001_initial.sql
│       └── 002_session_events.sql
├── games/
│   └── tictactoe/
├── internal/
│   ├── api/
│   ├── auth/
│   ├── domain/
│   │   ├── rating/        ← Elo engine, Player, Team, Config, DynamicK
│   │   └── matchmaking/   ← Queue-based matchmaker with snake draft
│   ├── engine/
│   ├── events/
│   ├── lobby/
│   ├── platform/
│   │   └── mathutil/      ← Sigmoid, Clamp, Mean, Variance
│   ├── presence/
│   ├── randutil/
│   ├── ratelimit/
│   ├── runtime/
│   ├── store/
│   ├── telemetry/
│   ├── testutil/
│   └── ws/
```

---

## Architecture — key decisions (do not revert)

- **`fakestore.go`** implements full `store.Store` interface — keep in sync on every store change
- **`GetGameConfig` in FakeStore** returns defaults `{60, 10, 300}` — never `ErrNotFound`
- **Turn timer** uses Redis TTL + keyspace notifications (`--notify-keyspace-events Ex` in docker-compose redis command)
- **Event sourcing** — Redis Streams for active sessions, Postgres `session_events` for finished ones
- **WS hub** uses Redis pub/sub for multi-instance fanout (`NewHubWithRedis`)
- **Spectator access** via `room_settings.allow_spectators` — no SpectatorLink table
- **Single migration file** — `001_initial.sql` (+ `002_session_events.sql` for event sourcing)
- **`state_after`** in moves table is `[]byte` → base64 in JSON → decode with `atob()` on frontend
- **Room settings** — generic KV table `room_settings(room_id, key, value)`, defaults inserted on `CreateRoom`
- **`randutil` package** — `internal/randutil`, crypto/rand based, used for room codes and seat assignment
- **`domain/` packages are pure** — no deps on store, ws, or any other internal package. Safe to import from anywhere.

---

## Redis — current uses

| Use | Status |
|---|---|
| Rate limiting | ✓ |
| Pub/sub for multi-instance WS | ✓ |
| Turn timer (SETEX + keyspace notifications) | ✓ |
| Player presence (SETEX + heartbeat) | ✓ |
| Event sourcing (Streams) | ✓ |
| Matchmaking queue (sorted set) | Pending — with matchmaking wiring |
| Real-time leaderboard (sorted set) | Pending — with ELO wiring |
| Session store | Pending |
| Idempotency keys | With real traffic |

---

## Known technical debt
- **`rematch_first_mover_policy` is inert** — defined in migration and engine, but `VoteRematch` in `runtime.go` never reads it.
- **`winner_chooses` / `loser_chooses`** — require extra UI step, no WS event or screen defined yet.
- **`scanSession` incomplete** — `store_pg.go` does not scan `turn_timeout_secs` or `last_move_at`.
- **Rematch flow** — `VoteRematch` creates a new session directly. Should return to lobby in `waiting` state instead.

---

## Backlog (prioritized)

### Immediate — to discuss before starting
1. **Wire ELO into match results** — call `Engine.ProcessMatch` after `FinishSession`. Open decisions:
   - Where does `rating.Player` state live? (new `ratings` table vs columns on `players`)
   - Who calls the engine — `runtime`, a new `rating service`, or a post-game hook?
   - Do we update MMR synchronously (in the finish transaction) or async (via event)?
2. **Wire matchmaking** — expose queue endpoints, run `FindMatches` on a ticker, auto-create rooms. Open decisions:
   - Is the matchmaker in-process (single instance) or Redis-backed (multi-instance)?
   - Do matched players get a WS push or poll?

### Features
3. **Session suspension** — `SuspendedAt` exists in schema, `ErrSuspended` in runtime, no endpoint exposed.
4. **Rematch flow refactor** — return to lobby in `waiting` instead of creating session directly.
5. **ELO leaderboard** — `GetLeaderboard` already exists in store; wire to real MMR/DisplayRating.
6. **In-match chat** — new WS event `chat_message`.
7. **Player profiles** — history, stats, avatar.
8. **More games** — engine is pluggable, each game in `games/<n>/`.

### Structural (post-MVP)
- Migrate existing packages into `internal/domain/` and `internal/platform/` structure already started
- Split `api.go` by domain
- Split `store.go` / `store_pg.go` when it exceeds ~150 methods

---

## Future folder structure (post-MVP)
```
server/
├── internal/
│   ├── domain/
│   │   ├── lobby/
│   │   ├── match/
│   │   ├── rating/        ← done
│   │   ├── matchmaking/   ← done
│   │   ├── replay/
│   │   └── player/
│   └── platform/
│       ├── store/
│       ├── ws/
│       ├── auth/
│       ├── ratelimit/
│       ├── randutil/
│       └── mathutil/      ← done
└── games/
    ├── registry.go
    └── tictactoe/
```