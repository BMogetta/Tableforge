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
- **Tests:** Playwright E2E (70 tests, all passing), Go unit tests (all passing)

---

## Project status

### What works today (API + backend complete)
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer, room settings
- Matches: TicTacToe with turn timer (Redis TTL + keyspace notifications), surrender, rematch vote
- Spectators: join/leave, count badge, blocked from rematch/moves/pause/resume
- Private rooms: code hidden in lobby, owner sees code
- WebSockets: hub with Redis pub/sub for multi-instance fanout, per-player channels (`BroadcastToPlayer`), dedicated `/ws/players/{playerID}` endpoint
- Admin: roles (player/manager/owner), allowed emails, leaderboard (Elo-based, per-game)
- Rate limiting via Redis
- Event sourcing: Redis Streams (active) → Postgres `session_events` (finished)
- Session history & replay: event log + board reconstruction slider
- Player presence: Redis SETEX + heartbeat, `presence_update` WS event
- Rating engine: multi-team generalised Elo with dynamic K-factor (fully tested)
- Elo ratings wired to runtime: `applyRatings` called after `ApplyMove` and `Surrender` in ranked sessions; ratings scoped per game (`game_id`)
- Matchmaking: Redis-backed ranked queue, snake draft, spread relaxation, match confirmation flow, progressive ban system
- Room chat: store + API + WS + UI complete (`ChatSidebar` in `Room.tsx`)
- Player mutes: store + API complete — **UI pending**
- Session pause/resume: store + runtime + API + WS complete — **UI pending**
- Direct messages: store + API + WS complete — **UI pending**
- Notifications: store + domain service + API + WS complete; bell icon + badge in `LobbyHeader` — **full notification UI pending**
- Matchmaking queue: Redis-backed, store + API + WS + background ticker + frontend queue status in `NewGamePanel` — player channel WS drives real-time state updates

### Frontend (this session)
- `ws.ts` — fully typed discriminated union for all WS events; `PlayerSocket` class for per-player channel; exponential backoff reconnect (base 2s, cap 30s, max 10 attempts)
- `api.ts` — `Rating` without `mmr`; `leaderboard.get(gameId)` required; `RoomMessage` with `message_id`/`timestamp`; `wsPlayerUrl`; `rooms.messages` + `rooms.sendMessage`
- `store.ts` — queue state (`queueStatus`, `queueJoinedAt`, `matchId`); `playerSocket` with `connectPlayerSocket`/`disconnectPlayerSocket`
- `App.tsx` — connects `playerSocket` on login, disconnects on logout
- `queryClient.ts` — keys `notifications`, `dmUnread`, `roomMessages`
- Lobby refactored into components: `LobbyHeader`, `NewGamePanel`, `OpenRooms`, `LeaderboardPanel`, `RoomCard`
- `LeaderboardPanel` — shows `display_rating`, filtered by `game_id`, top 20
- `LobbyHeader` — bell icon (unread notifications badge), envelope icon (unread DMs badge)
- `NewGamePanel` — tabs Casual/Ranked; queue status (spinner + elapsed time + cancel) driven by `playerSocket` events
- `Room.tsx` + `ChatSidebar` — collapsible chat sidebar, HTTP resync every 30s + WS `chat_message` events
- `chat.spec.ts` — 5 e2e tests covering WS delivery, ordering, collapse/reopen, empty send, HTTP resync

---

## Folder structure (current)

```
server/
├── cmd/
│   ├── server/
│   ├── seed/
│   ├── seed-test/          ← seeds ratings for player1/player2 (leaderboard test)
│   └── simulate/
├── db/
│   └── migrations/
│       ├── 001_initial.sql
│       ├── 002_session_events.sql
│       ├── 003_chat_ratings_moderation.sql
│       ├── 004_notifications.sql
│       └── 005_ratings_game_id.sql
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
│   │   │   ├── api_sessions.go
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
│   │       ├── ws.go
│   │       └── ws_player_handler.go
│   └── testutil/
│       └── fakestore.go
frontend/
├── src/
│   ├── components/
│   │   ├── lobby/
│   │   │   ├── LobbyHeader.tsx + .module.css
│   │   │   ├── NewGamePanel.tsx + .module.css
│   │   │   ├── OpenRooms.tsx + .module.css
│   │   │   ├── LeaderboardPanel.tsx + .module.css
│   │   │   └── RoomCard.tsx + .module.css
│   │   └── room/
│   │       └── ChatSidebar.tsx + .module.css
│   ├── pages/
│   │   ├── Lobby.tsx + .module.css
│   │   ├── Room.tsx + .module.css
│   │   └── Game.tsx + .module.css
│   ├── api.ts
│   ├── ws.ts
│   ├── store.ts
│   └── queryClient.ts
└── tests/
    └── e2e/
        ├── chat.spec.ts
        ├── leaderboard.spec.ts
        └── ...
```

---

## Architecture — key decisions (do not revert)
- **`fakestore.go`** implements full `store.Store` interface — keep in sync on every store change
- **`GetRatingLeaderboard` returns `[]RatingLeaderboardEntry{}`** (never nil) when empty; ordered by `display_rating DESC`
- **`GetGameConfig` in FakeStore** returns defaults `{60, 10, 300}` — never `ErrNotFound`
- **Turn timer** uses Redis TTL + keyspace notifications (`--notify-keyspace-events Ex`)
- **Event sourcing** — Redis Streams for active sessions, Postgres `session_events` for finished ones
- **WS hub** uses Redis pub/sub for multi-instance fanout (`NewHubWithRedis`)
- **Per-player WS channel** — `hub_player.go` exposes `BroadcastToPlayer(playerID, event)`; `ws_player_handler.go` serves `/ws/players/{playerID}`; used for DMs, queue events, notifications; `PlayerSocket` in frontend connects on login
- **Spectator access** via `room_settings.allow_spectators` — no SpectatorLink table
- **`state_after`** in moves is `[]byte` → base64 in JSON → `atob()` on frontend
- **Room settings** — generic KV table `room_settings(room_id, key, value)`, defaults on `CreateRoom`
- **`domain/` packages are pure** — no deps on store, ws, or any other internal package
- **Mutes are server-side** — persisted in `player_mutes`, survive across sessions
- **Chat mute toggle** (mute entire room chat) is UI-only local state — not persisted
- **Ratings are per-game** — `ratings` table has composite PK `(player_id, game_id)`; `game_id` required on all rating endpoints
- **`mmr` never reaches the frontend** — `RatingLeaderboardEntry` excludes it; leaderboard shows `display_rating` only
- **`applyRatings` never returns an error** — rating failure must not roll back a completed match; errors are logged only
- **Room chat is auditable** — persisted in Postgres, never deleted, hideable by manager only
- **`RoomMessage` uses `message_id`/`timestamp`** — both HTTP response and WS payload use these field names (not `id`/`created_at`)
- **Session mode** (`casual` | `ranked`) — passed through `lobby.StartGame(ctx, roomID, requesterID, mode)`; rating changes only apply in ranked mode; `handleStartGame` hardcodes `SessionModeCasual` — ranked only possible via queue flow
- **Pause is consent-based** — all present players must vote to pause and to resume
- **No pause timeout** — sessions can stay suspended indefinitely
- **Suspended sessions** can be force-closed by a manager
- **`player_id` in request body** — all handlers read `player_id` from body, not JWT context. Exception: `GET /notifications` reads `player_id` from query param (GET has no body)
- **Redis vs Postgres for chat/mutes/ratings** — all go directly to Postgres. No caching.
- **Spectators receive chat events** — `EventChatMessage` and `EventChatMessageHidden` not in `spectatorBlocklist`
- **`requireRole` middleware in tests** — returns 401 (no auth middleware), so manager-only DELETE tests expect 401
- **Notifications are never deleted** — `read_at` tracks state; read notifications older than 30 days excluded from list results
- **Notification actions expire** — `friend_request` after 7 days, `room_invitation` after 24h, `ban_issued` has no action
- **Queue is Redis-backed** — sorted set `queue:ranked` (score = MMR), distributed lock `queue:lock` (SET NX EX 4s) ensures only one instance runs `FindMatches` per tick
- **`RankedGameID` is exported** from `queue.go` — used by `fakestore` tests
- **Queue ban formula** — `min(5 * 5^(offense-1), 1440)` minutes; threshold: 3 declines within 1 hour
- **Match confirmation window** — 30s TTL on `queue:pending:{matchID}`; both players must explicitly accept
- **Declining a match** drops the player from the queue and applies a penalty; accepting player is re-queued automatically
- **`miniredis`** used for Redis unit tests in `platform/queue/queue_test.go` — no other package has Redis test infrastructure yet
- **`joinRoom` omitted from `useEffect` deps in `Room.tsx`** — it is a stable Zustand action that never changes identity; including it causes unnecessary socket reconnects
- **Queue state persists across navigation** — stored in Zustand (`queueStatus`, `queueJoinedAt`, `matchId`); player can queue and browse other pages

---

## Store — models

### Added in 003
- `SessionMode` — `"casual"` | `"ranked"`
- `RoomMessage`, `DirectMessage`, `PlayerMute`, `Rating`
- `GameSession` gains: `Mode SessionMode`, `PauseVotes []string`, `ResumeVotes []string`

### Added in 004
- `Notification` — `ID, PlayerID, Type, Payload (JSONB), ActionTaken, ActionExpiresAt, ReadAt, CreatedAt`
- `NotificationType` — `"friend_request"` | `"room_invitation"` | `"ban_issued"`
- `BanReason` — `"decline_threshold"` | `"moderator"`
- `NotificationPayloadFriendRequest`, `NotificationPayloadRoomInvitation`, `NotificationPayloadBanIssued`
- `CreateNotificationParams`
- `ErrNotificationActionExpired` in `pg_notifications.go`

### Added in 005
- `Rating` gains `GameID string`
- `RatingLeaderboardEntry` — `PlayerID, GameID, Username, AvatarURL, DisplayRating, GamesPlayed, WinStreak, LossStreak, UpdatedAt`
- `ratings` composite PK `(player_id, game_id)`; index on `(game_id, display_rating DESC)`

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
| `pg_ratings.go` | `GetRating(ctx, playerID, gameID)`, `UpsertRating`, `GetRatingLeaderboard(ctx, gameID, limit)` |
| `pg_pause.go` | `VotePause`, `ClearPauseVotes`, `VoteResume`, `ClearResumeVotes`, `ForceCloseSession` |
| `pg_notifications.go` | `CreateNotification`, `GetNotification`, `ListNotifications`, `MarkNotificationRead`, `SetNotificationAction` |

---

## WebSocket events

### Implemented
| Event | Channel | Payload |
|---|---|---|
| `move_applied` | room | full move result |
| `game_over` | room | full move result |
| `player_joined` | room | RoomView |
| `player_left` | room | `player_id` |
| `owner_changed` | room | `owner_id` |
| `room_closed` | room | — |
| `game_started` | room | session |
| `rematch_vote` | room | `player_id, votes, total_players` |
| `rematch_ready` | room | `room_id` |
| `setting_updated` | room | `key, value` |
| `spectator_joined` | room | `spectator_count` |
| `spectator_left` | room | `spectator_count` |
| `presence_update` | room | `player_id, online` |
| `chat_message` | room | `message_id, room_id, player_id, content, timestamp` |
| `chat_message_hidden` | room | `message_id` |
| `pause_vote_update` | room | `votes: []player_id, required: int` |
| `session_suspended` | room | `suspended_at` |
| `resume_vote_update` | room | `votes: []player_id, required: int` |
| `session_resumed` | room | `resumed_at` |
| `dm_received` | player | `from, content, message_id, timestamp` |
| `dm_read` | player | `message_id` |
| `queue_joined` | player | `position, estimated_wait_secs, reason?` |
| `queue_left` | player | `reason` |
| `match_found` | player | `match_id, quality, timeout` |
| `match_cancelled` | player | `match_id, reason` |
| `match_ready` | player | `room_id, session_id` |
| `notification_received` | player | full `store.Notification` |

### Spectator blocklist (events NOT delivered to spectators)
`pause_vote_update`, `session_suspended`, `resume_vote_update`, `session_resumed`, `rematch_vote`, `rematch_ready`

---

## API endpoints

### Implemented
```
GET  /api/v1/games
GET  /api/v1/leaderboard?game_id=tictactoe&limit=N   ← game_id required

POST /api/v1/players
GET  /api/v1/players/{playerID}/sessions
GET  /api/v1/players/{playerID}/stats
POST /api/v1/players/{playerID}/mute
DEL  /api/v1/players/{playerID}/mute/{mutedID}
GET  /api/v1/players/{playerID}/mutes
POST /api/v1/players/{playerID}/dm
GET  /api/v1/players/{playerID}/dm/{otherPlayerID}
GET  /api/v1/players/{playerID}/dm/unread
GET  /api/v1/players/{playerID}/notifications?include_read=false&player_id={id}

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

WS /ws/rooms/{roomID}?player_id={id}
WS /ws/players/{playerID}              ← new: player channel for queue/DM/notification events
```

---

## `api.ts` — namespaces implemented
All namespaces use a shared `request<T>(path, init)` helper (no axios).

| Namespace | Methods |
|---|---|
| `auth` | `me`, `logout`, `loginUrl` |
| `players` | `stats`, `sessions` |
| `sessions` | `get`, `move`, `surrender`, `rematch`, `pause`, `resume`, `forceClose`, `events`, `history` |
| `rooms` | `create`, `list`, `get`, `join`, `leave`, `start`, `updateSetting`, `messages`, `sendMessage` |
| `dm` | `send`, `history`, `unreadCount`, `markRead`, `report` |
| `mutes` | `mute`, `unmute`, `list` |
| `queue` | `join`, `leave`, `accept`, `decline` |
| `notifications` | `list`, `markRead`, `accept`, `decline` |
| `leaderboard` | `get(gameId, limit)` — returns `Rating[]` |
| `gameRegistry` | `list` |
| `admin` | `listEmails`, `addEmail`, `removeEmail`, `listPlayers`, `setRole` |

### Interfaces exported from `api.ts`
`Player`, `RoomViewPlayer`, `AllowedEmail`, `Room`, `RoomView`, `GameSession`, `GameResult`, `Move`, `PlayerStats`, `Rating`, `PlayerMute`, `DirectMessage`, `QueuePosition`, `SessionEvent`, `RoomMessage`, `SettingOption`, `LobbySetting`, `GameInfo`, `Notification`, `NotificationType`, `NotificationPayloadFriendRequest`, `NotificationPayloadRoomInvitation`, `NotificationPayloadBanIssued`

### Utility functions exported from `api.ts`
`wsRoomUrl(roomId, playerId)`, `wsPlayerUrl(playerId)`

---

## `queryClient.ts` — key factory
```ts
keys = {
  rooms, room, games, gameConfig,
  leaderboard(gameId?),
  session, player,
  adminEmails, adminPlayers,
  notifications(playerId),
  dmUnread(playerId),
  roomMessages(roomId),
}
```

---

## `store.ts` — Zustand state
```ts
// Auth
player, setPlayer

// Room socket (per-room, created in Room.tsx)
socket, activeRoomId, joinRoom, leaveRoom

// Player socket (persistent, created in App.tsx on login)
playerSocket, connectPlayerSocket, disconnectPlayerSocket

// Spectator
isSpectator, setIsSpectator
spectatorCount, setSpectatorCount

// Presence
presenceMap, setPlayerPresence

// Queue
queueStatus: 'idle' | 'queued' | 'match_found'
queueJoinedAt: number | null
matchId: string | null
setQueued(joinedAt), setMatchFound(matchId), clearQueue()
```

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

## UI debt (backend ready, frontend not started or partial)

| Feature | Backend status | Frontend status | Notes |
|---|---|---|---|
| Player mutes | ✅ complete | ❌ pending | Mute/unmute from chat panel; mute list in profile |
| Session pause/resume | ✅ complete | ❌ pending | Pause button, vote overlay, suspended screen |
| Direct messages | ✅ complete | ❌ pending | Inbox, conversation view, unread badge |
| Notifications full UI | ✅ complete | ⚠️ partial | Badge in `LobbyHeader` done; inline accept/decline panel pending |
| Friends system | ❌ not built | ❌ pending | `NotifyFriendRequest` implemented but no endpoint calls it; friends panel button mocked in Lobby |
| Match found sound | — | ❌ pending | Placeholder sound on `match_found` WS event |

---

## Known technical debt

### Backend
- **`rematch_first_mover_policy` is inert** — `VoteRematch` in `runtime.go` never reads it; `winner_chooses`/`loser_chooses` have no WS event defined
- **`scanSession` in `pg_timer.go`** (`ListSessionsNeedingTimer`) must be verified against the column order added in 003 migration
- **Queue confirmation timeout penalisation** — when `queue:pending:*` expires, the key is gone so we cannot distinguish who accepted vs timed out. Both players receive `match_cancelled`; neither is penalised. Fix requires shadow key pattern (see TODO in `queue.go`)
- **Estimated wait time** — `estimatedWait` returns `position * 10s` as a placeholder; should use rolling average of recent match wait times stored in Redis (see TODO in `queue.go`)
- **Queue: group support** — `QueueEntry` holds one player; pre-formed teams not supported (see TODO in `queue.go`)
- **Queue: multi-game ranked** — `RankedGameID` hardcoded to `"tictactoe"`; needs `game_id` field in `POST /queue` and one sorted set per game (see TODO in `queue.go`)
- **Notification friend request hook** — `NotifyFriendRequest` is implemented but nothing calls it yet; friend system endpoints not built
- **Notification room invitation hook** — `NotifyRoomInvitation` is implemented but no endpoint sends direct invitations yet
- **`miniredis` not extracted to `testutil`** — only `platform/queue` has Redis tests; extract a shared helper when a second package needs it
- **`handleStartGame` hardcodes `SessionModeCasual`** — ranked sessions are only possible via the queue flow; there is no way to start a ranked game manually from the UI

### Frontend
- **WS payload TODOs** — `player_joined`, `player_left`, `dm_received`, `dm_read`, `session_suspended`, `session_resumed`, `notification_received` typed as `unknown` in `ws.ts`; complete when those features get UI
- **WS lifecycle coupled to navigation** — socket created in `Room.tsx`, assumed to exist in `Game.tsx`; direct navigation to `/game/:sessionId` leaves `socket = null`, covered by polling fallback
- **Auth outside React Query** — `auth.me()` called via `useEffect` in `App.tsx`, not React Query; no caching or invalidation
- **Polling + WebSocket redundancy** — `Game.tsx` polls every 3s while WS is connected; intentional safety net; could be disabled while WS is healthy
- **`GameRenderer` uses `switch`** — replace with registry pattern when a second game is added