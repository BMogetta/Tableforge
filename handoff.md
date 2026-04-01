# Recess — Session Handoff

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
- **Frontend:** React + TypeScript, Vite, CSS Modules, TanStack Query, `@tanstack/react-pacer`
- **Infra:** Docker Compose, Caddy (reverse proxy + Cloudflare tunnel)
- **Observability:** OpenTelemetry + Jaeger (traces) + Prometheus (metrics) + Loki/Promtail (logs), all in Grafana
- **Tests:** Playwright E2E (70+ tests, all passing), Go unit tests (all passing), Vitest unit tests (all passing)

---

## Project status

### What works today (API + backend complete)
- Auth via GitHub OAuth + test-login for Playwright
- Lobby: create room, join by code, leave, ownership transfer, room settings
- Matches: TicTacToe and Love Letter with turn timer (Redis TTL + keyspace notifications), surrender, rematch vote
- Spectators: join/leave, count badge, blocked from rematch/moves/pause/resume
- Private rooms: code hidden in lobby, owner sees code
- WebSockets: hub with Redis pub/sub for multi-instance fanout, per-player channels (`BroadcastToPlayer`), dedicated `/ws/players/{playerID}` endpoint; `BroadcastToRoom` for per-client filtered state (Love Letter hands)
- Admin: roles (player/manager/owner), allowed emails, leaderboard (Elo-based, per-game)
- Rate limiting via Redis — returns JSON `{"error":"rate limit exceeded","reset_at":N}` on 429
- Event sourcing: Redis Streams (active) → Postgres `session_events` (finished)
- Session history & replay: event log + board reconstruction slider
- Player presence: Redis SETEX + heartbeat, `presence_update` WS event
- Rating engine: multi-team generalised Elo with dynamic K-factor (fully tested)
- Elo ratings wired to runtime: `applyRatings` called after `ApplyMove` and `Surrender` in ranked sessions; ratings scoped per game (`game_id`)
- Matchmaking: Redis-backed ranked queue, snake draft, spread relaxation, match confirmation flow, progressive ban system
- Room chat: store + API + WS + UI complete (`ChatSidebar` in `Room.tsx`)
- Player mutes: store + API complete — **UI pending**
- Session pause/resume: store + runtime + API + WS + **UI complete**
- Direct messages: store + API + WS complete — **UI pending**
- Notifications: store + domain service + API + WS complete; bell icon + badge in `LobbyHeader` — **full notification UI pending**
- Matchmaking queue: Redis-backed, store + API + WS + background ticker + frontend queue status in `NewGamePanel` — player channel WS drives real-time state updates
- **Bot system: Sessions A–F complete** — fill bots, MCTS, TicTacToe adapter, runtime integration, UI
- **Player settings: store + API + frontend complete** — persisted as JSONB, merged over defaults at read time, debounced sync via `@tanstack/react-pacer`
- **Error-as-value pattern: implemented** — `src/helpers/errors.ts`, `ErrorMessage` component, `Toast` system; `Room.tsx` and `Game.tsx` migrated; remaining pages pending

### Bot system (Sessions A–F complete)
- `internal/bot/game.go` — `BotGameState`, `BotMove`, `BotAdapter` interfaces; `RolloutPolicy` on `BotAdapter`
- `internal/bot/state.go` — `concreteState`, `Clone()` via JSON round-trip, `MoveFromPayload`/`BotMove.Payload()`
- `internal/bot/config.go` — `BotConfig`, `PersonalityProfile`, built-in profiles (easy/medium/hard/aggressive)
- `internal/bot/player.go` — `BotPlayer{ID, GameID, Adapter, Config, Search SearchFunc}`, `DecideMove()`
- `internal/bot/mcts/` — IS-MCTS: `node.go`, `mcts.go`, `Search()`, `RandomRolloutPolicy()`
- `internal/bot/adapter/registry.go` — `New(gameID)` returns adapter or error
- `internal/bot/adapter/tictactoe/` — full `BotAdapter` implementation, bot-vs-bot test
- `internal/domain/runtime/runtime_bot.go` — `botRegistry` (sync.RWMutex), `RegisterBot`, `UnregisterBot`, `MaybeFireBot`; called from `handleMove` after broadcast and from `handleStartGame` for first-move bots
- `internal/platform/api/api_bots.go` — `handleListBotProfiles`, `handleAddBot`, `handleRemoveBot`
- Fill bots: ephemeral, `is_bot=TRUE`, username `"bot:<profile>:<8-char-uuid>"`
- Bot registry is in-memory only — lost on server restart
- `VoteRematch` auto-votes for all registered bots to prevent rematch deadlock

### Frontend (this session)
- `src/helpers/errors.ts` — `Result<S,E>`, `ok`, `error`, `AppError`, `ApiErrorReason`, `toAppError`, `catchToAppError`, `generateCode`; dev shows raw message, prod shows friendly message + `ERR_XXXX` code
- `src/components/ui/Toast.tsx` + `.module.css` — `ToastProvider`, `useToast`, `showError/showWarning/showInfo`; max 3 toasts, 4s auto-dismiss, progress bar
- `src/components/ui/ErrorMessage.tsx` + `.module.css` — inline error display; dev shows reason badge + status + raw message, prod shows friendly message + code
- `src/components/ui/Settings.tsx` + `.module.css` — full settings panel; toggle, select, volume rows; debounced backend sync via `useDebouncedCallback`; optimistic update + localStorage cache
- `src/pages/RateLimited.tsx` — 60s countdown, auto-redirect to `/`, "Try Now" button
- `src/pages/Room.tsx` — migrated to error-as-value; `data-testid` on bot UI elements (`add-bot-btn`, `add-bot-select`, `bot-row-{id}`, `remove-bot-btn-{id}`)
- `src/pages/Game.tsx` — migrated to error-as-value; full pause/resume UI: pause button in header, vote overlay, suspended screen with resume voting; syncs `suspended_at` from initial fetch for "return tomorrow" case
- `src/pages/Game.module.css` — added `.suspended`, `.voteOverlay`, `.voteTitle`, `.voteCount`, `.voteWaiting`, `.suspendedScreen`, `.suspendedIcon`, `.suspendedTitle`, `.suspendedBody`
- `src/store.ts` — settings slice: `settings: ResolvedSettings`, `hydrateSettings`, `updateSetting`, `setSettings`; `ResolvedSettings = Required<PlayerSettingMap>`
- `src/api.ts` — `GameSession` fully typed (all backend fields); `PlayerSettingMap`, `PlayerSettings`, `DEFAULT_SETTINGS`, `playerSettings` namespace (`get`, `update`); `request<T>` handles 204 (no body) and redirects 429 to `/rate-limited`
- `src/App.tsx` — `ToastProvider` wraps `ErrorBoundary`; settings loaded on mount (localStorage cache first, backend fetch in background); `RateLimited` route added
- `src/components/lobby/LobbyHeader.tsx` — settings gear icon button + modal (top-right, click-outside to close)

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
│       ├── 004_notifications.sql
│       ├── 005_ratings_game_id.sql
│       ├── 006_bot_players.sql         ← is_bot column on players
│       └── 007_player_settings.sql     ← player_settings JSONB table
├── games/
│   ├── tictactoe/
│   └── loveletter/
│       ├── loveletter.go
│       ├── loveletter_cards.go
│       └── loveletter_settings.go
├── internal/
│   ├── bot/
│   │   ├── game.go ← BotGameState, BotMove, BotAdapter interfaces
│   │   ├── state.go ← concreteState, Clone, MoveFromPayload
│   │   ├── config.go ← BotConfig, PersonalityProfile, profiles
│   │   ├── player.go
│   │   ├── state_test.go
│   │   ├── config_test.go
│   │   ├── player_test.go
│   │   ├── adapter/
│   │   │   ├── registry.go
│   │   │   └── tictactoe/
│   │   │       ├── adapter.go
│   │   │       └── adapter_test.go
│   │   └── mcts/
│   │       ├── node.go
│   │       ├── mcts.go
│   │       ├── export_test.go
│   │       ├── node_test.go
│   │       └── engine_test.go
│   ├── domain/
│   │   ├── engine/
│   │   ├── lobby/
│   │   ├── matchmaking/
│   │   ├── notification/
│   │   ├── rating/
│   │   └── runtime/
│   │       ├── runtime.go
│   │       ├── runtime_bot.go
│   │       ├── runtime_pause.go
│   │       ├── runtime_rating.go
│   │       ├── runtime_bot_test.go     ← registry, MaybeFireBot, VotePause, VoteResume
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
│   │   │   ├── api_bots.go
│   │   │   ├── api_player_settings.go
│   │   │   ├── api_test.go
│   │   │   ├── api_chat_test.go
│   │   │   ├── api_mutes_test.go
│   │   │   ├── api_dm_test.go
│   │   │   ├── api_pause_test.go
│   │   │   ├── api_notifications_test.go
│   │   │   ├── api_bots_test.go
│   │   │   └── api_player_settings_test.go
│   │   ├── auth/
│   │   ├── events/
│   │   ├── mathutil/
│   │   ├── presence/
│   │   ├── queue/
│   │   │   ├── queue.go
│   │   │   └── queue_test.go
│   │   ├── randutil/
│   │   ├── ratelimit/
│   │   │   ├── ratelimit.go            ← 429 now returns JSON
│   │   │   ├── clock.go
│   │   │   ├── script.go
│   │   │   └── ratelimit_test.go
│   │   ├── store/
│   │   │   ├── store.go                ← PlayerSettings, PlayerSettingMap, DefaultPlayerSettings
│   │   │   ├── pg.go
│   │   │   ├── pg_chat.go
│   │   │   ├── pg_mutes.go
│   │   │   ├── pg_ratings.go
│   │   │   ├── pg_pause.go
│   │   │   ├── pg_notifications.go
│   │   │   ├── pg_player_settings.go   ← GetPlayerSettings, UpsertPlayerSettings, mergeSettings
│   │   │   ├── pg_test.go
│   │   │   ├── pg_chat_test.go
│   │   │   └── pg_player_settings_test.go
│   │   ├── telemetry/
│   │   └── ws/
│   │       ├── hub.go
│   │       ├── hub_player.go
│   │       ├── ws.go
│   │       └── ws_player_handler.go
│   └── testutil/
│       └── fakestore.go                ← PlayerSettingsMap field, GetPlayerSettings, UpsertPlayerSettings
frontend/
├── src/
│   ├── helpers/
│   │   ├── errors.ts                   ← Result, ok, error, AppError, toAppError, catchToAppError
│   │   └── errors.test.ts
│   ├── components/
│   │   ├── SurrenderModal.tsx + .module.css
│   │   ├── TicTacToe.tsx + .module.css
│   │   ├── ui/
│   │   │   ├── Toast.tsx + .module.css
│   │   │   ├── ErrorMessage.tsx + .module.css
│   │   │   ├── Settings.tsx + .module.css
│   │   │   └── __tests__/
│   │   │       ├── Toast.test.tsx
│   │   │       ├── ErrorMessage.test.tsx
│   │   │       └── Settings.test.tsx
│   │   ├── lobby/
│   │   │   ├── LobbyHeader.tsx + .module.css   ← settings gear + modal
│   │   │   ├── NewGamePanel.tsx + .module.css
│   │   │   ├── OpenRooms.tsx + .module.css
│   │   │   ├── LeaderboardPanel.tsx + .module.css
│   │   │   └── RoomCard.tsx + .module.css
│   │   ├── room/
│   │   │   ├── RoomSettings.tsx + .module.css
│   │   │   └── ChatSidebar.tsx + .module.css
│   │   └── loveletter/
│   │       ├── LoveLetter.tsx + .module.css
│   │       ├── CardDisplay.tsx + .module.css
│   │       ├── HandDisplay.tsx + .module.css
│   │       ├── TargetPicker.tsx + .module.css
│   │       ├── ChancellorModal.tsx + .module.css
│   │       ├── PlayerBoard.tsx + .module.css
│   │       ├── RoundSummary.tsx + .module.css
│   │       └── __tests__/
│   │           ├── CardDisplay.test.tsx
│   │           ├── HandDisplay.test.tsx
│   │           ├── TargetPicker.test.tsx
│   │           ├── ChancellorModal.test.tsx
│   │           └── PlayerBoard.test.tsx
│   ├── test/
│   │   └── setup.ts
│   ├── pages/
│   │   ├── Lobby.tsx + .module.css
│   │   ├── Room.tsx + .module.css      ← error-as-value, bot testids
│   │   ├── Game.tsx + .module.css      ← error-as-value, pause/resume UI
│   │   ├── RateLimited.tsx             ← new, 60s countdown
│   │   └── __tests__/
│   │       └── RateLimited.test.tsx
│   ├── api.ts                          ← GameSession fully typed, playerSettings, DEFAULT_SETTINGS
│   ├── store.ts                        ← settings slice added
│   ├── store.settings.test.ts
│   ├── ws.ts
│   └── queryClient.ts
└── tests/
    └── e2e/
        ├── bot.spec.ts                 ← new: bot full game, remove/replace bot
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
- **`BroadcastToRoom`** — `hub.go` exposes `BroadcastToRoom(roomID, playerIDs, eventFn, spectatorEvent)`; calls `eventFn(playerID)` per non-spectator client in single-instance mode; publishes to each player's Redis channel in multi-instance mode; spectator payload goes to room channel with `filteredEnvelope{S: true}`; used by `BroadcastMove` for games implementing `StateFilter`
- **`BroadcastFiltered`** — `hub.go` exposes `BroadcastFiltered(roomID, playerEvent, spectatorEvent)`; used for uniform player payloads with a different spectator payload; `playerEvent.Payload == nil` skips non-spectators
- **`StateFilter` interface** — optional `engine.Game` extension; if implemented, `BroadcastMove` calls `FilterState(state, playerID)` per player via `BroadcastToRoom`, and `FilterState(state, "")` for spectators; TicTacToe does not implement it; Love Letter does
- **`TurnTimeoutHandler` interface** — optional `engine.Game` extension; if implemented, `turn_timer.go` delegates timeout to `ApplyMove` with the game-provided payload instead of applying platform-level penalty directly; Love Letter returns `{"card":"penalty_lose"}`
- **`GAME_RENDERERS` registry** — `Game.tsx` uses `Record<string, RendererComponent>` instead of a switch; renderers defined as named components outside `Game` to avoid remount on render
- **E2E game selection** — all `create-room-btn` usages in E2E helpers click `game-option-tictactoe` first; default game is non-deterministic when multiple games are registered
- **Bot package lives in `internal/bot/`** — not in `domain/` (pure business logic) nor `platform/` (infrastructure); it is an application layer that depends on `domain/engine` but not on store, ws, or api
- **`BotMove` is a JSON string** — serialized form of `map[string]any` payload; comparable, usable as map key; round-trips via `MoveFromPayload` / `BotMove.Payload()`
- **`Clone()` uses JSON round-trip** — same encoding the runtime uses for Postgres persistence; correct by definition; optimize with hand-written clone per game if profiling shows bottleneck
- **Bot `PlayerID` is `uuid.UUID`** — bots are registered as real `store.Player` rows (`is_bot=TRUE`) so `AddPlayerToRoom` works unchanged; username convention `"bot:<profile>:<8-char-uuid>"`
- **`bot.ErrNoMoves` defined in `internal/bot`** — re-exported as `mcts.ErrNoMoves` to avoid import cycle
- **`SearchFunc` injected into `BotPlayer`** — breaks `bot` ↔ `mcts` import cycle
- **`MaybeFireBot` called in two places** — `handleMove` (after human move broadcast) and `handleStartGame` (for first-move bots); both pass `currentPlayerID` from game state
- **Bot registry is in-memory only** — lost on server restart; re-registered via `POST /rooms/{id}/bots`
- **`VoteRematch` auto-votes for registered bots** — prevents rematch deadlock when bot is in room
- **`player_settings` stored as JSONB** — single row per player; application layer merges stored values over `DefaultPlayerSettings()` at read time; nil pointer fields mean "use default"
- **Settings cache key** `'tf:settings'` in localStorage — hydrated immediately on app start; backend fetch runs in background to pick up cross-device changes
- **Settings debounce** — `useDebouncedCallback` from `@tanstack/react-pacer` with 600ms wait; optimistic update in store + localStorage write are immediate
- **`ResolvedSettings = Required<PlayerSettingMap>`** — all keys guaranteed non-optional after merging; store always holds a `ResolvedSettings`
- **Error-as-value pattern** — `Result<S,E>` tuple `[E,null]|[null,S]`; `reason` discriminant enables exhaustive switch; `catchToAppError(e)` converts unknown caught values; new code must use this pattern; migrate existing code when touching it
- **Dev vs prod error display** — dev: raw server message; prod: friendly message + deterministic `ERR_XXXX` code (first 4 chars of `btoa(reason+status)`)
- **Toast for background/transient errors** — errors with no inline DOM location (remove bot, refresh failure) go to toast; action errors with a clear location use `ErrorMessage` inline
- **`request<T>` 204 handling** — returns `undefined as T` for 204/empty responses instead of calling `.json()`
- **`request<T>` 429 handling** — sets `window.location.href = '/rate-limited'` after ending the OTel span; `throw` after redirect is unreachable but kept for type safety
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
- **`RoomMessage` uses `message_id`/`timestamp`** — both HTTP response and WS payload use these field names
- **Session mode** (`casual` | `ranked`) — `handleStartGame` hardcodes `SessionModeCasual`; ranked only possible via queue flow
- **Pause is consent-based** — all present players must vote to pause and to resume
- **No pause timeout** — sessions can stay suspended indefinitely
- **Suspended sessions** can be force-closed by a manager
- **`player_id` in request body** — all handlers read `player_id` from body, not JWT context
- **Redis vs Postgres for chat/mutes/ratings** — all go directly to Postgres. No caching.
- **Spectators receive chat events** — `EventChatMessage` and `EventChatMessageHidden` not in `spectatorBlocklist`
- **`requireRole` middleware in tests** — returns 401 (no auth middleware), so manager-only DELETE tests expect 401
- **Notifications are never deleted** — `read_at` tracks state; read notifications older than 30 days excluded
- **Queue is Redis-backed** — sorted set `queue:ranked`, distributed lock `queue:lock`
- **`RankedGameID` is exported** from `queue.go` — used by `fakestore` tests
- **Queue ban formula** — `min(5 * 5^(offense-1), 1440)` minutes; threshold: 3 declines within 1 hour
- **Match confirmation window** — 30s TTL on `queue:pending:{matchID}`; both players must explicitly accept
- **Declining a match** drops the player from the queue and applies a penalty; accepting player is re-queued automatically
- **`miniredis` not extracted to `testutil`**
- **`joinRoom` omitted from `useEffect` deps in `Room.tsx`** — stable Zustand action
- **Queue state persists across navigation** — stored in Zustand
- **Love Letter deck has 21 cards** — updated edition
- **Love Letter draw model** — active player always holds 2 cards
- **Love Letter `private_reveals`** — cleared at start of next `applyStandardMove`
- **Love Letter Handmaid expiry** — protection removed from next player at start of `advanceTurn`
- **Love Letter `penalty_lose`** — synthetic card; bypasses validation; eliminates active player; used by `TurnTimeoutHandler`
- **Love Letter state persists in Postgres** — Redis caching deferred

---

## Bot system — status

| Session | Status |
|---|---|
| A — Interfaces & config | ✅ complete |
| B — Core MCTS | ✅ complete |
| C — TicTacToe adapter | ✅ complete |
| D — Love Letter adapter | ❌ pending |
| E — BotPlayer & integration | ✅ complete |
| F — Config & API + frontend | ✅ complete |

---

## Store — models

### Added in 006
- `players.is_bot BOOLEAN NOT NULL DEFAULT FALSE`
- `Store.CreateBotPlayer(ctx, username)` — inserts with `is_bot=TRUE`

### Added in 007
```sql
CREATE TABLE player_settings (
    player_id  UUID PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    settings   JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
- `PlayerSettings{PlayerID, Settings PlayerSettingMap, UpdatedAt}`
- `PlayerSettingMap` — pointer fields for all settings (nil = use default)
- `DefaultPlayerSettings()` — canonical defaults; theme=dark, language=en, all notifications on, all volumes 1.0
- `Store.GetPlayerSettings(ctx, playerID)` — returns stored merged over defaults; returns all defaults if no row exists
- `Store.UpsertPlayerSettings(ctx, playerID, settings)` — atomic ON CONFLICT DO UPDATE
- `mergeSettings(defaults, stored)` — in `pg_player_settings.go`; only non-nil stored fields override defaults

### `scanPlayer` column order in `pg.go` — do not reorder
```
id, username, avatar_url, role, is_bot, created_at, deleted_at
```

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
| `pg_player_settings.go` | `GetPlayerSettings`, `UpsertPlayerSettings`, `mergeSettings` |

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

### Spectator blocklist
`pause_vote_update`, `session_suspended`, `resume_vote_update`, `session_resumed`, `rematch_vote`, `rematch_ready`

---

## API endpoints

### Implemented
```
GET  /api/v1/games
GET  /api/v1/leaderboard?game_id=tictactoe&limit=N ← game_id required

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
GET  /api/v1/players/{playerID}/settings
PUT  /api/v1/players/{playerID}/settings

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
POST /api/v1/rooms/{roomID}/bots
DEL  /api/v1/rooms/{roomID}/bots/{botID}

GET  /api/v1/sessions/{sessionID}
GET  /api/v1/sessions/{sessionID}/events
GET  /api/v1/sessions/{sessionID}/history
POST /api/v1/sessions/{sessionID}/move
POST /api/v1/sessions/{sessionID}/surrender
POST /api/v1/sessions/{sessionID}/rematch
POST /api/v1/sessions/{sessionID}/pause
POST /api/v1/sessions/{sessionID}/resume
DEL  /api/v1/sessions/{sessionID}                  ← manager only

POST /api/v1/dm/{messageID}/read
POST /api/v1/notifications/{notificationID}/read
POST /api/v1/notifications/{notificationID}/accept
POST /api/v1/notifications/{notificationID}/decline

POST /api/v1/queue
DEL  /api/v1/queue
POST /api/v1/queue/accept
POST /api/v1/queue/decline

GET  /api/v1/bots/profiles

GET    /api/v1/admin/allowed-emails
POST   /api/v1/admin/allowed-emails
DELETE /api/v1/admin/allowed-emails/{email}
GET    /api/v1/admin/players
PUT    /api/v1/admin/players/{playerID}/role

WS /ws/rooms/{roomID}?player_id={id}
WS /ws/players/{playerID} ← player channel for queue/DM/notification/filtered-move events
```

---

## `api.ts`, `queryClient.ts`, `store.ts` — namespaces implemented
All namespaces use a shared `request<T>(path, init)` helper (no axios).

| Namespace | Methods |
|---|---|
| `auth` | `me`, `logout`, `loginUrl` |
| `players` | `stats`, `sessions` |
| `sessions` | `get`, `move`, `surrender`, `rematch`, `pause`, `resume`, `forceClose`, `events`, `history` |
| `rooms` | `create`, `list`, `get`, `join`, `leave`, `start`, `updateSetting`, `messages`, `sendMessage` |
| `bots` | `profiles`, `add`, `remove` |
| `playerSettings` | `get`, `update` |
| `dm` | `send`, `history`, `unreadCount`, `markRead`, `report` |
| `mutes` | `mute`, `unmute`, `list` |
| `queue` | `join`, `leave`, `accept`, `decline` |
| `notifications` | `list`, `markRead`, `accept`, `decline` |
| `leaderboard` | `get(gameId, limit)` |
| `gameRegistry` | `list` |
| `admin` | `listEmails`, `addEmail`, `removeEmail`, `listPlayers`, `setRole` |

### Interfaces exported from `api.ts`
`Player`, `RoomViewPlayer`, `AllowedEmail`, `Room`, `RoomView`, `GameSession` (fully typed), `GameResult`, `Move`, `PlayerStats`, `Rating`, `PlayerMute`, `DirectMessage`, `QueuePosition`, `SessionEvent`, `RoomMessage`, `SettingOption`, `LobbySetting`, `GameInfo`, `Notification`, `NotificationType`, `BotProfile`, `PlayerSettingMap`, `PlayerSettings`

### Constants exported from `api.ts`
`DEFAULT_SETTINGS: Required<PlayerSettingMap>` — mirrors `store.DefaultPlayerSettings()`

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

// Settings
settings: ResolvedSettings             // Required<PlayerSettingMap>, always fully populated
hydrateSettings(raw: PlayerSettings)   // merges stored over defaults, call on API response
updateSetting(key, value)              // optimistic update
setSettings(settings)                  // bulk replace, used for localStorage cache restore
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

## UI debt

| Feature | Backend | Frontend | Notes |
|---|---|---|---|
| Love Letter renderer | ✅ | ✅ | Components built, Vitest coverage, E2E pending |
| Player mutes | ✅ | ❌ | Mute/unmute from chat panel; mute list in profile |
| Session pause/resume | ✅ | ✅ | Complete — pause button, vote overlay, suspended screen, return-tomorrow support |
| Direct messages | ✅ | ❌ | Inbox, conversation view, unread badge |
| Notifications full UI | ✅ | ⚠️ | Badge done; inline accept/decline pending |
| Friends system | ❌ | ❌ | `NotifyFriendRequest` implemented but no endpoint calls it |
| Bot creation UI | ✅ | ✅ | Add/remove bot in Room.tsx; profile select |
| Error-as-value migration | — | ⚠️ | `Room.tsx` and `Game.tsx` migrated; `Lobby.tsx`, `Admin.tsx`, other pages pending |
| Audio system | — | ❌ | Volume sliders are stubs — stored but not applied |
| `reduce_motion` CSS | — | ❌ | Setting stored but not applied to CSS animations |
| `show_move_hints` / `confirm_move` in Game | — | ❌ | Settings stored but not wired to `Game.tsx` logic |
| Named bots | ❌ | ❌ | Persistent bots with elo; separate from fill bots |
| Love Letter bot adapter | ❌ | — | Session D — `Determinize` samples hidden hand |

---

## Known technical debt

### Backend
- **`rematch_first_mover_policy` is inert** — `VoteRematch` never reads it
- **`scanSession` in `pg_timer.go`** — must be verified against 003 migration column order
- **Queue confirmation timeout penalisation** — shadow key pattern needed
- **Estimated wait time** — placeholder `position * 10s`
- **Queue: group support** — `QueueEntry` holds one player only
- **Queue: multi-game ranked** — `RankedGameID` hardcoded to `"tictactoe"`
- **Notification hooks** — `NotifyFriendRequest` and `NotifyRoomInvitation` implemented but nothing calls them
- **`miniredis` not extracted to `testutil`**
- **`handleStartGame` hardcodes `SessionModeCasual`**
- **Bot registry is in-memory** — lost on server restart; bots must be re-registered; consider persisting active bot assignments in Redis or a new table

### Frontend
- **Love Letter E2E** — no Playwright tests for gameplay
- **WS payload TODOs** — `player_joined`, `player_left`, `dm_received`, `dm_read`, `session_suspended`, `session_resumed`, `notification_received` typed as `unknown`
- **WS lifecycle coupled to navigation** — direct navigation to `/game/:id` leaves `socket = null`
- **Auth outside React Query** — `auth.me()` in `useEffect`, no caching
- **Polling + WebSocket redundancy** — intentional safety net
- **Error-as-value migration incomplete** — `Lobby.tsx`, `Admin.tsx`, and other pages still use try/catch; migrate when touching
- **`bot.spec.ts` first test may be flaky** — `TestBotPlaysFullGame` assumes P1 wins top row before bot blocks; bot with `easy` profile (100 iterations) may occasionally block; consider asserting game ends rather than P1 wins
- **Audio volume controls are stubs** — `Settings.tsx` renders disabled sliders; no audio system exists yet
- **`reduce_motion` not applied** — stored in settings but no CSS class toggling or `prefers-reduced-motion` override implemented
- **`show_move_hints` / `confirm_move`** — stored in settings but `Game.tsx` does not read them from the store yet
- **Settings cache key `'tf:settings'`** — no versioning or migration strategy if `PlayerSettingMap` shape changes; stale cache could cause type mismatches