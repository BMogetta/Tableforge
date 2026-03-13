# Tableforge — Session Handoff

## Rules for every session
- **Read before writing.** Ask for the file, read it, propose changes, confirm, only then write.
- **One file at a time.** Move to the next only after the owner confirms it integrates and compiles.
- **Full file replacement only.** Never partial patches.
- **Propose changes before writing** and wait for confirmation.
- **English only.** All code, comments, godoc, JSDoc, and any written artifact.
- **Split files proactively.** If adding code to an existing file would push it past ~300 lines, or if a clear logical grouping exists, split into multiple files in the same package before adding.

---

## Stack
- **Backend:** Go 1.23, chi, pgx/v5, Redis, JWT auth, WebSocket hub
- **Frontend:** React + TypeScript, Vite, CSS Modules, TanStack Query
- **Infra:** Docker Compose, Caddy (reverse proxy + Cloudflare tunnel)
- **Observability:** OpenTelemetry + Jaeger (traces) + Prometheus (metrics) + Loki/Promtail (logs), all in Grafana
- **Tests:** Playwright E2E (65 tests, all passing), Go unit tests (all passing)

---

## Project status

### What works today (API + backend complete)
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer, room settings
- Matches: TicTacToe with turn timer (Redis TTL + keyspace notifications), surrender, rematch vote
- Spectators: join/leave, count badge, blocked from rematch/moves/pause/resume
- Private rooms: code hidden in lobby, owner sees code
- WebSockets: hub with Redis pub/sub for multi-instance fanout, per-player channels (`BroadcastToPlayer`)
- Admin: roles (player/manager/owner), allowed emails, leaderboard (Elo-based)
- Rate limiting via Redis
- Event sourcing: Redis Streams (active) → Postgres `session_events` (finished)
- Session history & replay: event log + board reconstruction slider
- Player presence: Redis SETEX + heartbeat, `presence_update` WS event
- Rating engine: multi-team generalised Elo with dynamic K-factor (fully tested)
- Elo ratings wired to runtime: `applyRatings` called after `ApplyMove` and `Surrender` in ranked sessions
- Matchmaking: Redis-backed ranked queue, snake draft, spread relaxation, match confirmation flow, progressive ban system
- Room chat: store + API + WS complete (`pg_chat.go`, `api_chat.go`) — **UI pending**
- Player mutes: store + API complete (`pg_mutes.go`, `api_mutes.go`) — **UI pending**
- Session pause/resume: store + runtime + API + WS complete (`pg_pause.go`, `api_pause.go`) — **UI pending**
- Direct messages: store + API + WS complete (`pg_chat.go`, `api_dm.go`) — **UI pending**
- Notifications: store + domain service + API + WS complete (`pg_notifications.go`, `domain/notification/`) — **UI pending**
- Matchmaking queue: Redis-backed, store + API + WS + background ticker complete (`platform/queue/`, `api_queue.go`) — **UI pending**

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
│       ├── 003_chat_ratings_moderation.sql
│       └── 004_notifications.sql
├── games/
│   └── tictactoe/
├── internal/
│   ├── domain/
│   │   ├── engine/
│   │   ├── lobby/
│   │   ├── matchmaking/
│   │   ├── notification/
│   │   │   └── notification.go
│   │   ├── rating/
│   │   └── runtime/
│   │       ├── runtime.go
│   │       ├── runtime_pause.go
│   │       ├── runtime_rating.go
│   │       └── turn_timer.go
│   ├── platform/
│   │   ├── api/
│   │   │   ├── api.go
│   │   │   ├── api_lobby.go
│   │   │   ├── api_session.go
│   │   │   ├── api_chat.go
│   │   │   ├── api_admin.go
│   │   │   ├── api_mutes.go
│   │   │   ├── api_dm.go
│   │   │   ├── api_pause.go
│   │   │   ├── api_queue.go
│   │   │   ├── api_notifications.go
│   │   │   ├── api_test.go
│   │   │   ├── api_chat_test.go
│   │   │   ├── api_mutes_test.go
│   │   │   ├── api_dm_test.go
│   │   │   ├── api_pause_test.go
│   │   │   └── api_notifications_test.go
│   │   ├── auth/
│   │   ├── events/
│   │   ├── mathutil/
│   │   ├── presence/
│   │   ├── queue/
│   │   │   ├── queue.go
│   │   │   └── queue_test.go
│   │   ├── randutil/
│   │   ├── ratelimit/
│   │   ├── store/
│   │   │   ├── store.go
│   │   │   ├── pg.go
│   │   │   ├── pg_chat.go
│   │   │   ├── pg_mutes.go
│   │   │   ├── pg_ratings.go
│   │   │   ├── pg_pause.go
│   │   │   ├── pg_notifications.go
│   │   │   ├── pg_test.go
│   │   │   └── pg_chat_test.go
│   │   ├── telemetry/
│   │   └── ws/
│   │       ├── hub.go
│   │       ├── hub_player.go
│   │       └── ws.go
│   └── testutil/
│       └── fakestore.go
```

---

## Architecture — key decisions (do not revert)
- **`fakestore.go`** implements full `store.Store` interface — keep in sync on every store change
- **`GetRatingLeaderboard` returns `[]store.Rating{}`** (never nil) when empty
- **`GetGameConfig` in FakeStore** returns defaults `{60, 10, 300}` — never `ErrNotFound`
- **Turn timer** uses Redis TTL + keyspace notifications (`--notify-keyspace-events Ex`)
- **Event sourcing** — Redis Streams for active sessions, Postgres `session_events` for finished ones
- **WS hub** uses Redis pub/sub for multi-instance fanout (`NewHubWithRedis`)
- **Per-player WS channel** — `hub_player.go` exposes `BroadcastToPlayer(playerID, event)`; used for DMs, queue events, notifications
- **Spectator access** via `room_settings.allow_spectators` — no SpectatorLink table
- **`state_after`** in moves is `[]byte` → base64 in JSON → `atob()` on frontend
- **Room settings** — generic KV table `room_settings(room_id, key, value)`, defaults on `CreateRoom`
- **`domain/` packages are pure** — no deps on store, ws, or any other internal package
- **Mutes are server-side** — persisted in `player_mutes`, survive across sessions
- **Chat mute toggle** (mute entire room chat) is UI-only local state — not persisted
- **Ratings are decoupled from players** — separate `ratings` table, only updated in ranked sessions
- **`applyRatings` never returns an error** — rating failure must not roll back a completed match; errors are logged only
- **Room chat is auditable** — persisted in Postgres, never deleted, hideable by manager only
- **Session mode** (`casual` | `ranked`) — passed through `lobby.StartGame(ctx, roomID, requesterID, mode)`; rating changes only apply in ranked mode
- **Pause is consent-based** — all present players must vote to pause and to resume
- **No pause timeout** — sessions can stay suspended indefinitely
- **Suspended sessions** can be force-closed by a manager
- **`player_id` in request body** — all handlers read `player_id` from body, not JWT context. Do not change.
- **Redis vs Postgres for chat/mutes/ratings** — all go directly to Postgres. No caching.
- **Spectators receive chat events** — `EventChatMessage` and `EventChatMessageHidden` not in `spectatorBlocklist`
- **`requireRole` middleware in tests** — returns 401 (no auth middleware), so manager-only DELETE tests expect 401
- **Notifications are never deleted** — `read_at` tracks state; read notifications older than 30 days excluded from list results
- **Notification actions expire** — `friend_request` after 7 days, `room_invitation` after 24h, `ban_issued` has no action
- **Queue is Redis-backed** — sorted set `queue:ranked` (score = MMR), distributed lock `queue:lock` (SET NX EX 4s) ensures only one instance runs `FindMatches` per tick
- **Queue ban formula** — `min(5 * 5^(offense-1), 1440)` minutes; threshold: 3 declines within 1 hour
- **Match confirmation window** — 30s TTL on `queue:pending:{matchID}`; both players must explicitly accept
- **Declining a match** drops the player from the queue and applies a penalty; accepting player is re-queued automatically
- **`miniredis`** used for Redis unit tests in `platform/queue/queue_test.go` — no other package has Redis test infrastructure yet

---

## Store — models

### Added in 003
- `SessionMode` — `"casual"` | `"ranked"`
- `RoomMessage`, `DirectMessage`, `PlayerMute`, `Rating`
- `GameSession` gains: `Mode SessionMode`, `PauseVotes []string`, `ResumeVotes []string`

### Added in 005
- `Notification` — `ID, PlayerID, Type, Payload (JSONB), ActionTaken, ActionExpiresAt, ReadAt, CreatedAt`
- `NotificationType` — `"friend_request"` | `"room_invitation"` | `"ban_issued"`
- `BanReason` — `"decline_threshold"` | `"moderator"`
- `NotificationPayloadFriendRequest`, `NotificationPayloadRoomInvitation`, `NotificationPayloadBanIssued`
- `CreateNotificationParams`
- `ErrNotificationActionExpired` in `pg_notifications.go`

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
| `pg_notifications.go` | `CreateNotification`, `GetNotification`, `ListNotifications`, `MarkNotificationRead`, `SetNotificationAction` |

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
| `pause_vote_update` | `votes: []player_id, required: int` |
| `session_suspended` | `suspended_at` |
| `resume_vote_update` | `votes: []player_id, required: int` |
| `session_resumed` | `resumed_at` |
| `dm_received` | `from, content, message_id, timestamp` |
| `dm_read` | `message_id` |
| `queue_joined` | `position, estimated_wait_secs` |
| `queue_left` | `reason` |
| `match_found` | `match_id, quality, timeout` |
| `match_cancelled` | `match_id, reason` |
| `match_ready` | `room_id, session_id` |
| `notification_received` | full `store.Notification` |

### Spectator blocklist (events NOT delivered to spectators)
`pause_vote_update`, `session_suspended`, `resume_vote_update`, `session_resumed`, `rematch_vote`, `rematch_ready`

---

## API endpoints

### Implemented
```
GET  /api/v1/games
GET  /api/v1/leaderboard?limit=N

POST /api/v1/players
GET  /api/v1/players/{playerID}/sessions
GET  /api/v1/players/{playerID}/stats
POST /api/v1/players/{playerID}/mute
DEL  /api/v1/players/{playerID}/mute/{mutedID}
GET  /api/v1/players/{playerID}/mutes
POST /api/v1/players/{playerID}/dm
GET  /api/v1/players/{playerID}/dm/{otherPlayerID}
GET  /api/v1/players/{playerID}/dm/unread
GET  /api/v1/players/{playerID}/notifications?include_read=false

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
POST /api/v1/sessions/{sessionID}/pause
POST /api/v1/sessions/{sessionID}/resume
DEL  /api/v1/sessions/{sessionID}                  ← manager only (force close)

POST /api/v1/dm/{messageID}/read
POST /api/v1/notifications/{notificationID}/read
POST /api/v1/notifications/{notificationID}/accept
POST /api/v1/notifications/{notificationID}/decline

POST /api/v1/queue
DEL  /api/v1/queue
POST /api/v1/queue/accept
POST /api/v1/queue/decline

GET    /api/v1/admin/allowed-emails
POST   /api/v1/admin/allowed-emails
DELETE /api/v1/admin/allowed-emails/{email}
GET    /api/v1/admin/players
PUT    /api/v1/admin/players/{playerID}/role
```

---

## `api.ts` — namespaces implemented
All namespaces use a shared `request<T>(path, init)` helper (no axios).

| Namespace | Methods |
|---|---|
| `sessions` | `get`, `move`, `surrender`, `rematch`, `pause`, `resume`, `forceClose`, `events`, `history` |
| `rooms` | `create`, `list`, `get`, `join`, `leave`, `start`, `updateSetting` |
| `chat` | `send`, `getMessages`, `report`, `hide` |
| `dm` | `send`, `getHistory`, `getUnreadCount`, `markRead`, `report` |
| `mutes` | `mute`, `unmute`, `list` |
| `queue` | `join`, `leave`, `accept`, `decline` |
| `notifications` | `list`, `markRead`, `accept`, `decline` |
| `ratings` | (leaderboard via `GET /leaderboard`) |

### Interfaces exported from `api.ts`
`PlayerMute`, `Rating`, `DirectMessage`, `QueuePosition`, `Notification` (store shape with `payload: unknown`)

---

## Redis — current uses

| Use | Status |
|---|---|
| Rate limiting | ✅ |
| Pub/sub for multi-instance WS | ✅ |
| Turn timer (SETEX + keyspace notifications) | ✅ |
| Player presence (SETEX + heartbeat) | ✅ |
| Event sourcing (Streams) | ✅ |
| Matchmaking queue (sorted set by MMR) | ✅ |
| Queue pending confirmations (hash + TTL) | ✅ |
| Queue ban tracking (string + TTL) | ✅ |
| Queue decline history (list + TTL) | ✅ |
| Distributed matchmaking lock (SET NX EX) | ✅ |

---

## UI debt (backend ready, frontend not started)

| Feature | Backend status | Notes |
|---|---|---|
| Room chat | ✅ complete | Chat panel per room, mute button per player, local room-mute toggle |
| Player mutes | ✅ complete | Mute/unmute from chat panel; mute list in profile |
| Session pause/resume | ✅ complete | Pause button, vote overlay, suspended screen |
| Direct messages | ✅ complete | Inbox, conversation view, unread badge |
| Notifications (bell) | ✅ complete | Bell icon, unread count, inline accept/decline for friend requests and room invitations |
| Matchmaking queue | ✅ complete | "Find Match" button, queue screen with position + wait estimate, match found overlay (accept/decline), ban feedback |
| Elo / DisplayRating | ✅ complete | Show `display_rating` on player profile and in lobby player list |

---

## Known technical debt

### Backend
- **`rematch_first_mover_policy` is inert** — `VoteRematch` in `runtime.go` never reads it; `winner_chooses`/`loser_chooses` have no WS event defined
- **`scanSession` in `pg_timer.go`** (`ListSessionsNeedingTimer`) must be verified against the column order added in 003 migration
- **Rematch flow** — `VoteRematch` creates a new session directly; should return to lobby in `waiting` state
- **Queue confirmation timeout penalisation** — when `queue:pending:*` expires, the key is gone so we cannot distinguish who accepted vs timed out. Both players receive `match_cancelled` and must re-queue; neither is penalised. Fix requires shadow key pattern (see TODO in `queue.go`)
- **Estimated wait time** — `estimatedWait` returns `position * 10s` as a placeholder; should use rolling average of recent match wait times stored in Redis (see TODO in `queue.go`)
- **Queue: group support** — `QueueEntry` holds one player; pre-formed teams not supported (see TODO in `queue.go`)
- **Queue: multi-game ranked** — `rankedGameID` hardcoded to `"tictactoe"`; needs `game_id` field in `POST /queue` and one sorted set per game (see TODO in `queue.go`)
- **Notification friend request hook** — `NotifyFriendRequest` is implemented but nothing calls it yet; friend system endpoints (`POST /friends/request` etc.) not built
- **Notification room invitation hook** — `NotifyRoomInvitation` is implemented but no endpoint sends direct invitations yet; room code sharing is the only current invite mechanism
- **`miniredis` not extracted to `testutil`** — only `platform/queue` has Redis tests; extract a shared helper when a second package needs it
- **`BanDurationForOffense` is exported** for testing but is an internal concern — consider moving the test inline if the function is ever refactored

### Frontend
- All UI items listed in the UI debt table above