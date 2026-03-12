# Tableforge — Session Handoff

## Rules for every session
- **Read before writing.** Ask for the file, read it, propose changes, confirm, only then write.
- **One file at a time.** Move to the next only after the owner confirms it integrates and compiles.
- **Full file replacement only.** Never partial patches.
- **Propose changes before writing** and wait for confirmation.
- **English only.** All code, comments, godoc, JSDoc, and any written artifact.
- **Split files proactively.** If adding code to an existing file would push it past ~300 lines, or if a clear logical grouping exists, split into multiple files in the same package before adding. See "File splitting candidates" below.

---

## Stack
- **Backend:** Go 1.23, chi, pgx/v5, Redis, JWT auth, WebSocket hub
- **Frontend:** React + TypeScript, Vite, CSS Modules, TanStack Query
- **Infra:** Docker Compose, Caddy (reverse proxy + Cloudflare tunnel)
- **Observability:** OpenTelemetry + Jaeger (traces) + Prometheus (metrics) + Loki/Promtail (logs), all in Grafana
- **Tests:** Playwright E2E (65 tests, all passing), Go unit tests (all passing)

---

## Project status

### What works today
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer, room settings (first_mover_policy, visibility, spectators, turn timeout)
- Matches: TicTacToe with turn timer (Redis TTL + keyspace notifications), surrender, rematch vote
- Spectators: join/leave, count badge, blocked from rematch/moves
- Private rooms: code hidden in lobby, no join button, owner sees code
- WebSockets: hub with Redis pub/sub for multi-instance fanout
- Admin: roles (player/manager/owner), allowed emails, leaderboard (Elo-based, returns `[]store.Rating`)
- Rate limiting via Redis
- Event sourcing: Redis Streams (active) → Postgres `session_events` (finished)
- Session history & replay: event log + board reconstruction slider
- Player presence: Redis SETEX + heartbeat, `presence_update` WS event
- Rating engine: multi-team generalised Elo with dynamic K-factor (fully tested)
- Matchmaking: queue-based with snake draft and spread relaxation (fully tested)
- Simulation CLI: `go run ./cmd/simulate`
- Room chat: store + API + WS complete (`pg_chat.go`, `api_chat.go`), UI pending
- Player mutes: store complete (`pg_mutes.go`), API pending
- Ratings table: store complete (`pg_ratings.go`), not yet wired to runtime
- Pause/resume votes: store complete (`pg_pause.go`), runtime + API pending

---

## Folder structure (current)

```
server/
├── cmd/
│   ├── server/
│   ├── seed/
│   ├── seed-test/
│   └── simulate/
├── db/
│   └── migrations/
│       ├── 001_initial.sql
│       ├── 002_session_events.sql
│       └── 003_chat_ratings_moderation.sql
├── games/
│   └── tictactoe/
├── internal/
│   ├── domain/
│   │   ├── engine/
│   │   ├── lobby/
│   │   ├── matchmaking/
│   │   ├── rating/
│   │   └── runtime/
│   ├── platform/
│   │   ├── api/
│   │   │   ├── api.go              ← router, health, requireRole, helpers
│   │   │   ├── api_lobby.go        ← games + rooms handlers
│   │   │   ├── api_session.go      ← sessions, moves, players, leaderboard
│   │   │   ├── api_chat.go         ← room chat handlers
│   │   │   ├── api_admin.go        ← allowed emails, player roles
│   │   │   ├── api_test.go
│   │   │   └── api_chat_test.go
│   │   ├── auth/
│   │   ├── events/
│   │   ├── mathutil/
│   │   ├── presence/
│   │   ├── randutil/
│   │   ├── ratelimit/
│   │   ├── store/
│   │   │   ├── store.go
│   │   │   ├── pg.go
│   │   │   ├── pg_chat.go
│   │   │   ├── pg_mutes.go
│   │   │   ├── pg_ratings.go
│   │   │   ├── pg_pause.go
│   │   │   ├── pg_test.go
│   │   │   └── pg_chat_test.go
│   │   ├── telemetry/
│   │   └── ws/
│   └── testutil/
```

---

## File splitting candidates

| File | Split into |
|---|---|
| `platform/api/api.go` | already split — add `api_mutes.go`, `api_dm.go`, `api_pause.go`, `api_queue.go` as needed |
| `platform/store/pg.go` | already split — add `pg_players.go`, `pg_rooms.go`, `pg_sessions.go` if it grows past ~300 lines |
| `platform/store/store.go` | same grouping as pg.go — one interface section per domain |
| `domain/runtime/runtime.go` | `runtime_session.go`, `runtime_pause.go`, `runtime_rematch.go` |

---

## Architecture — key decisions (do not revert)
- **`fakestore.go`** implements full `store.Store` interface — keep in sync on every store change
- **`GetRatingLeaderboard` returns `[]store.Rating{}`** (never nil) when empty — `LeaderboardEntry` type removed
- **`GetGameConfig` in FakeStore** returns defaults `{60, 10, 300}` — never `ErrNotFound`
- **Turn timer** uses Redis TTL + keyspace notifications (`--notify-keyspace-events Ex`)
- **Event sourcing** — Redis Streams for active sessions, Postgres `session_events` for finished ones
- **WS hub** uses Redis pub/sub for multi-instance fanout (`NewHubWithRedis`)
- **Spectator access** via `room_settings.allow_spectators` — no SpectatorLink table
- **`state_after`** in moves is `[]byte` → base64 in JSON → `atob()` on frontend
- **Room settings** — generic KV table `room_settings(room_id, key, value)`, defaults on `CreateRoom`
- **`domain/` packages are pure** — no deps on store, ws, or any other internal package
- **Mutes are server-side** — persisted in `player_mutes`, survive across sessions
- **Chat mute toggle** (mute entire room chat) is UI-only local state — not persisted
- **Ratings are decoupled from players** — separate `ratings` table, only updated in ranked matches
- **Room chat is auditable** — persisted in Postgres, never deleted, hideable by manager only
- **Session mode** (`casual` | `ranked`) — rating changes only apply in ranked mode
- **Pause is consent-based** — all present players must vote to pause and to resume
- **No pause timeout** — casual sessions can stay suspended indefinitely
- **Suspended sessions** can be force-closed by a manager
- **`player_id` in request body** — all handlers read `player_id` from body, not JWT context. Established pattern, do not change.
- **Redis vs Postgres for chat/mutes/ratings** — all go directly to Postgres. No caching. Revisit if leaderboard becomes a hot path.
- **Spectators receive chat events** — `EventChatMessage` and `EventChatMessageHidden` are not in `spectatorBlocklist`. Spectators blocked from POST via participant check in handler.
- **`requireRole` middleware in tests** — returns 401 (no auth middleware), so DELETE /messages tests expect 401, not 403/400.
- **`pg_pause.go` tests skipped** — requires active sessions with room_players; covered in runtime integration tests when pause/resume is implemented.

---

## Store — models added in 003

- `SessionMode` — `"casual"` | `"ranked"`
- `RoomMessage` — `ID, RoomID, PlayerID, Content, Reported, Hidden, CreatedAt`
- `DirectMessage` — `ID, SenderID, ReceiverID, Content, ReadAt, Reported, Hidden, CreatedAt`
- `PlayerMute` — `MuterID, MutedID, CreatedAt`
- `Rating` — `PlayerID, MMR, DisplayRating, GamesPlayed, WinStreak, LossStreak, UpdatedAt`
- `GameSession` gains: `Mode SessionMode`, `PauseVotes []string`, `ResumeVotes []string`

### `scanSession` column order in `pg.go` — do not reorder
```
id, room_id, game_id, name, state, mode, move_count, suspend_count,
suspended_at, suspended_reason, pause_votes, resume_votes,
turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
```

### Store methods by file
| File | Methods |
|---|---|
| `pg.go` | players, rooms, sessions, moves, stats |
| `pg_chat.go` | `SaveRoomMessage`, `GetRoomMessages`, `HideRoomMessage`, `ReportRoomMessage`, `SaveDM`, `GetDMHistory`, `MarkDMRead`, `GetUnreadDMCount`, `ReportDM` |
| `pg_mutes.go` | `MutePlayer`, `UnmutePlayer`, `GetMutedPlayers` |
| `pg_ratings.go` | `GetRating`, `UpsertRating`, `GetRatingLeaderboard` |
| `pg_pause.go` | `VotePause`, `ClearPauseVotes`, `VoteResume`, `ClearResumeVotes`, `ForceCloseSession` |

---

## WebSocket events

### Implemented
| Event | Payload |
|---|---|
| `move_applied` | full move result |
| `game_over` | full move result |
| `player_joined` | RoomView |
| `player_left` | `player_id` |
| `owner_changed` | `owner_id` |
| `room_closed` | — |
| `game_started` | session |
| `rematch_vote` | `player_id, votes, total_players` |
| `rematch_ready` | `room_id` |
| `setting_updated` | `key, value` |
| `spectator_joined` | — |
| `spectator_left` | — |
| `presence_update` | list of connected player IDs |
| `chat_message` | `message_id, room_id, player_id, content, timestamp` |
| `chat_message_hidden` | `message_id` |

### Planned
| Event | Direction | Payload |
|---|---|---|
| `pause_vote_update` | server → clients | `votes: []player_id, required: int` |
| `session_suspended` | server → clients | `suspended_at` |
| `resume_vote_update` | server → clients | `votes: []player_id, required: int` |
| `session_resumed` | server → clients | `resumed_at` |
| `dm_received` | server → client | `from, content, message_id, timestamp` |
| `dm_read` | server → client | `message_id` |
| `match_found` | server → client | `room_id, opponent, mmr_diff` |
| `queue_joined` | server → client | `position, estimated_wait_secs` |
| `queue_left` | server → client | — |

---

## Redis — current and planned uses

| Use | Status |
|---|---|
| Rate limiting | ✓ |
| Pub/sub for multi-instance WS | ✓ |
| Turn timer (SETEX + keyspace notifications) | ✓ |
| Player presence (SETEX + heartbeat) | ✓ |
| Event sourcing (Streams) | ✓ |
| Matchmaking queue (sorted set by MMR) | Pending — item 8 |
| Real-time leaderboard (sorted set) | Pending — item 7 |
| Session store | Pending |

---

## API endpoints

### Implemented
```
GET  /api/v1/games
GET  /api/v1/leaderboard?limit=N           ← returns []store.Rating ordered by MMR desc

POST /api/v1/players
GET  /api/v1/players/{playerID}/sessions
GET  /api/v1/players/{playerID}/stats

POST /api/v1/rooms
GET  /api/v1/rooms
GET  /api/v1/rooms/{roomID}
POST /api/v1/rooms/join
POST /api/v1/rooms/{roomID}/leave
POST /api/v1/rooms/{roomID}/start
PUT  /api/v1/rooms/{roomID}/settings/{key}
POST /api/v1/rooms/{roomID}/messages
GET  /api/v1/rooms/{roomID}/messages
POST /api/v1/rooms/{roomID}/messages/{messageID}/report
DEL  /api/v1/rooms/{roomID}/messages/{messageID}   ← manager only

GET  /api/v1/sessions/{sessionID}
GET  /api/v1/sessions/{sessionID}/events
GET  /api/v1/sessions/{sessionID}/history
POST /api/v1/sessions/{sessionID}/move
POST /api/v1/sessions/{sessionID}/surrender
POST /api/v1/sessions/{sessionID}/rematch

GET    /api/v1/admin/allowed-emails
POST   /api/v1/admin/allowed-emails
DELETE /api/v1/admin/allowed-emails/{email}
GET    /api/v1/admin/players
PUT    /api/v1/admin/players/{playerID}/role
```

### Planned
```
POST   /api/v1/players/{playerID}/mute
DELETE /api/v1/players/{playerID}/mute/{mutedID}
GET    /api/v1/players/{playerID}/mutes

POST   /api/v1/sessions/{sessionID}/pause
POST   /api/v1/sessions/{sessionID}/resume
DELETE /api/v1/sessions/{sessionID}            ← manager: force close

POST   /api/v1/players/{playerID}/dm
GET    /api/v1/players/{playerID}/dm/{otherPlayerID}
GET    /api/v1/players/{playerID}/dm/unread
POST   /api/v1/dm/{messageID}/read

POST   /api/v1/queue
DELETE /api/v1/queue

GET    /api/v1/notifications
```

---

## Implementation backlog (ordered)

### Item 3: Mute system — next
- Store: ✅ done (`pg_mutes.go`)
- `api_mutes.go`: `POST /players/{playerID}/mute`, `DELETE /players/{playerID}/mute/{mutedID}`, `GET /players/{playerID}/mutes`
- Tests: `api_mutes_test.go`
- No WS event — mutes are applied client-side using the fetched mute list
- UI: mute button per player in chat panel + local toggle to mute entire room chat (not persisted)

### Item 4: Session pause / resume
- Store: ✅ done (`pg_pause.go`)
- Runtime: `VotePause`, `VoteResume` analogous to `VoteRematch`; reconstruct board from last `state_after` in `session_events` on resume
- `api_pause.go`: `POST /sessions/{sessionID}/pause`, `POST /sessions/{sessionID}/resume`, `DELETE /sessions/{sessionID}` (manager force close)
- WS events: `session_suspended`, `session_resumed`, `pause_vote_update`, `resume_vote_update`
- Tests: `api_pause_test.go` + pg_pause integration tests
- UI: pause button, vote overlay, suspended screen

### Item 5: Direct messages
- Store: ✅ done (`pg_chat.go`)
- `api_dm.go`: full DM endpoint set
- WS: `dm_received`, `dm_read`
- UI: inbox, conversation view, unread badge

### Item 6: Notifications
- `GET /api/v1/notifications` — unread DMs, match found, etc.
- Design TBD

### Item 7: Wire Elo ratings to runtime
- Store: ✅ done (`pg_ratings.go`)
- Runtime: call `rating.Engine.ProcessMatch` after `FinishSession` (ranked mode only)
- `GameSession.Mode` available to distinguish casual vs ranked
- UI: display `DisplayRating` on profile and lobby

### Item 8: Matchmaking queue (ranked mode)
- `POST/DELETE /api/v1/queue`
- WS: `queue_joined`, `match_found`, `queue_left`
- Background ticker running `FindMatches`
- Redis sorted set as queue backing store (multi-instance safe)
- UI: "Find Match" button, queue screen with estimated wait

---

## Known technical debt
- **`rematch_first_mover_policy` is inert** — `VoteRematch` in `runtime.go` never reads it
- **`winner_chooses` / `loser_chooses`** — no UI step, no WS event defined
- **`scanSession` in `pg_timer.go`** (`ListSessionsNeedingTimer`) must be verified against new column order after 003 migration
- **Rematch flow** — `VoteRematch` creates a new session directly; should return to lobby in `waiting`