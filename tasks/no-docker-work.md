# No-Docker Work (2026-04-07)

Tasks that can be done without Docker running. Mark with [x] when done.

## Pending Revisit

- [ ] **E2E player pool refactor** — commit `1ecfd74` (WIP, untested, needs Docker to validate)

## Quick Wins (< 1 hour each)

- [x] Remove TODO debug log — `services/game-server/internal/platform/api/api_lobby.go:272`
- [x] PII masking in auth logs — already done (maskEmail exists)
- [x] Add IdleTimeout to HTTP servers — already set (60s) on all 8 services
- [x] Fix unused import — `frontend/src/features/errors/NotFound.tsx:6` — false alarm, useRouter is used
- [x] Fix clickable divs a11y — `RoomToolbar.tsx:48`, `BansTab.tsx:114`

## Go Tests (miniredis/mocks, no Docker)

### High Priority — Business Logic

- [x] `games/rootaccess/` — already has 43 tests covering all card effects
- [x] `games/registry.go` — 5 tests (Register, Get, All, duplicate panic, DefaultRegistry)
- [x] `shared/middleware/recoverer.go` — 3 tests (no panic, panic 500, WS upgrade)
- [x] `shared/config/env.go` — 3 tests (value, default, empty)
- [x] `shared/errors/apierrors.go` — just constants, nothing to test

### Medium Priority — Handlers & Publishers

- [x] `user-service/internal/api/publisher.go` — 7 tests (all 6 publish functions + no-expiry case)
- [x] `user-service/internal/achievements/` — already has 11 tests (evaluator + registry)
- [x] `chat-service/internal/api/publisher.go` — 3 tests (room event, player event, nil redis)
- [x] `chat-service/internal/api/api_dms.go`, `api_rooms.go` — already has 22 tests
- [x] `auth-service/internal/handler/github.go` — 2 tests for githubGET + email selection logic
- [x] `game-server/internal/domain/runtime/ready.go` — already tested in runtime_ready_test.go
- [ ] `game-server/internal/domain/runtime/asynq_handlers.go` — HandleTurnTimeout, HandleReadyTimeout
- [ ] `game-server/internal/platform/store/pg_pause.go` — VotePause, ClearPauseVotes, VoteResume
- [ ] `ws-gateway/internal/hub/hub.go` — subscription/broadcast logic
- [x] `ws-gateway/internal/presence/presence.go` — already has 11 tests

## Frontend Tests (Vitest, no Docker)

### High Priority

- [ ] Hooks: `useBlockPlayer.ts` (needs React Query + store mocking)
- [x] Hooks: `useBreakpoint.ts` — 5 tests (breakpoints, mobile/tablet/desktop, isAtLeast)
- [x] Utils: `testId.ts` — 4 tests (testId + testAttr)
- [x] Lib: `authGuards.ts` — 5 tests (requireAuth + requireRole with role hierarchy)
- [ ] Utils: `lazyNamed.ts` (React.lazy wrapper, low value)
- [x] Game components — already tested in game-components.test.tsx (37 tests)
- [ ] Root Access game: CardFace, RootAccess, RoundSummary (React components)

### Medium Priority — Features

- [x] `lobby/` — 12 tests (RoomCard 6, ActiveGameBanner 5, RoomCardSkeleton 1)
- [x] `friends/` — 14 tests (FriendItem 9, PendingRequestItem 5)
- [x] `profile/` — 10 tests (AchievementCard 7, AchievementGrid 3)
- [ ] `room/` — 3 components (ChatPopover, RoomSettings, PlayerList)
- [x] `errors/` — 5 tests (ErrorScreen + Splash)

## Technical Debt

### High Priority

- [x] Auth failure logging — already done (slog.Warn on lines 103, 118, 143)
- [x] Admin audit logging — already done (LogAction in admin, bans, role handlers)

### Medium Priority

- [x] Pagination — GetDMHistory now takes limit/offset (default 50, max 200, ASC order)
- [x] Pagination — ListPlayers now takes limit/offset (default 50, max 200)
- [ ] Pagination remaining — ListSessionMoves, ListPendingReports
- [x] Lobby duplicate prevention — `IsPlayerInActiveRoom` check in CreateRoom + JoinRoom
- [ ] Achievement evaluator — `evaluator.go:70` use move count instead of duration heuristic

### Low Priority

- [ ] `as any` casts in `frontend/src/lib/device.ts` (4 occurrences)
