# Test Coverage Gaps (audited 2026-04-05)

Audit of untested or undertested code paths across backend and frontend.
Priority = risk if it breaks in prod without tests catching it.

---

## Backend — Critical (security / data integrity)

### 1. `shared/middleware/auth.go` — 0% tested

JWT verification, role extraction, token expiration.
`VerifyToken()` handles 3 error branches: `ErrTokenExpired`, invalid token,
invalid claims. None are tested.

**What to test:**
- Valid token passes and populates context with player ID + role
- Expired token returns 401
- Malformed/tampered token returns 401
- Missing Authorization header returns 401
- Token with wrong signing method rejected

**How:** Unit tests with a test JWT signer. No external deps needed.

### 2. `shared/redis/redis.go` — 0% tested

Redis client initialization with OTel instrumentation.

**What to test:**
- Connection with valid addr succeeds
- Invalid addr returns error (not panic)
- Client is instrumented (has tracing hook)

**How:** miniredis for happy path, bad addr string for error path.

### 3. `shared/config/env.go` — 0% tested

Environment variable parsing for all services.

**What to test:**
- All required vars present -> struct populated correctly
- Missing required var -> clear error message naming the var
- Invalid format (e.g. non-numeric port) -> clear error

**How:** `t.Setenv()` in unit tests.

### 4. `shared/errors/apierrors.go` — 0% tested

Maps internal errors to HTTP status + JSON response.

**What to test:**
- Known error types map to correct status codes
- Unknown errors map to 500 with generic message (no internal leak)
- Error response JSON shape matches frontend expectations

---

## Backend — High (business logic)

### 5. `game-server/internal/platform/events/` — 0% tested

Event publishing for `game.session.finished`, `game.move.applied`, etc.
Events drive inter-service communication (rating updates, notifications).

**What to test:**
- Event published with correct channel name + payload shape
- `event_id` UUID present for idempotency
- Publish error doesn't panic (graceful degradation)
- `Persist()` flushes buffered events

**How:** miniredis subscriber to capture published messages.

### 6. `game-server/internal/platform/grpc/` — happy path only

`game_handler.go` (StartSession, IsParticipant) and `lobby_handler.go`
(CreateRankedRoom). Match-service is the sole caller.

**What to test:**
- Invalid request (missing fields) returns proper gRPC status
- Session not found -> NOT_FOUND
- Player not participant -> PERMISSION_DENIED
- StartSession with already-started session -> error (not double-start)

**How:** In-process gRPC test with bufconn + FakeStore.

### 7. `user-service/internal/consumer/` — no dedup tests

Handles `player.banned` / `player.unbanned` Redis events.

**What to test:**
- Duplicate ban event is idempotent (no double-ban)
- Unban of non-existent ban is no-op
- Malformed event payload -> logged, not panic

### 8. `notification-service/internal/publisher/` — errors ignored

Publishes to player Redis channels. If publish fails, notification is silently lost.

**What to test:**
- Publish error is logged (or returned)
- Marshal error on malformed payload is handled

### 9. `ws-gateway/internal/handler/handler.go` — limited to presence

WebSocket lifecycle: connect, message routing, disconnect, room join/leave.
Only presence tests exist.

**What to test:**
- Client connects and joins room -> receives room state
- Client disconnects -> presence offline broadcast (fixed, but no regression test)
- Malformed message -> connection not killed, error logged
- Auth token expired mid-session -> graceful disconnect
- Rate limiting on message flood

### 10. `game-server/internal/platform/store/pg_pause.go`, `pg_ready.go`, `pg_timer.go`

SQL operations for pause votes, ready votes, timer state. No direct unit tests.
Covered indirectly through runtime tests with FakeStore, but FakeStore behavior
may diverge from real PG queries (this already caused Bug 3 — VoteReady).

**What to test:**
- Integration tests against real PG (testcontainers) for the tricky SQL:
  - `pg_pause.go` vote counting excludes bots
  - `pg_ready.go` consensus check excludes bots
  - `pg_timer.go` rescheduling with correct deadlines

---

## Backend — Medium

### 11. `shared/middleware/validate.go`

JSON Schema request validation middleware. Likely tested indirectly via API tests,
but no unit tests for edge cases:
- Request body exceeds size limit
- Schema not found for endpoint (misconfiguration)
- Partial validation errors (multiple fields invalid)

### 12. `match-service/internal/queue/`

Matchmaking queue. `expiry_test.go` exists but check:
- Queue with 0 players
- Player joins then disconnects before match
- Two players with very different MMR (no match within timeout)

### 13. `chat-service/internal/api/publisher.go`, `user_checker.go`

Event publishing for chat messages. `user_checker.go` validates user existence
before allowing DMs.

---

## Frontend — High

### 14. `features/game/hooks/useGameSocket` — 0% tested

WebSocket event deduplication, move ordering, state sync.
Most fragile path in the frontend — lots of mutable state.

**What to test:**
- Duplicate move event is ignored (dedup by event_id or move index)
- Out-of-order moves are applied in correct sequence
- Disconnect + reconnect resyncs state
- game_over event transitions to result screen

**How:** Vitest + mock WebSocket. Test the hook with renderHook().

### 15. `features/game/hooks/useGameSession`, `usePauseResume`, `useRematch` — 0% tested

State machines for session lifecycle, pause/resume voting, rematch flow.

**What to test:**
- useGameSession: polling cycle, game-over detection, cleanup on unmount
- usePauseResume: vote counting, state transitions (active->paused->active)
- useRematch: vote, accept, decline, timeout

### 16. Feature API clients (6 modules) — 0% tested

`features/{game,room,lobby,friends,notifications,admin}/api.ts`
All depend 100% on e2e tests. If e2e doesn't run, zero coverage.

**What to test (per module):**
- Successful response parsed correctly
- 4xx error mapped to user-friendly message
- Network failure doesn't crash

**How:** Vitest + msw (mock service worker) or vi.fn() on fetch.

### 17. `lib/ws.ts` — WebSocket reconnection — 0% tested

Exponential backoff, max retries, auth failure recovery.

**What to test:**
- Disconnect triggers reconnect with backoff
- Max retries exceeded -> gives up, surfaces error
- Auth failure on reconnect -> redirects to login (not infinite retry)

### 18. `features/auth/Login.tsx` — OAuth popup flow — 0% tested

Token exchange, popup handling, error recovery.

---

## Frontend — Medium

### 19. Game renderers (`src/games/tictactoe/`, `src/games/rootaccess/`)

Component rendering + interaction. Partially covered by e2e but no unit tests
for game-specific logic (valid move highlighting, animation triggers).

### 20. Card UI kit (`src/ui/cards/`)

Animation components (Card, CardHand, CardPile). No tests. Low risk since
purely visual, but layout regressions possible.

---

## Completed (2026-04-05)

1. ✅ `shared/middleware/auth.go` — 17 tests (VerifyToken, Require, RequireRole, context helpers)
2. ✅ `game-server/platform/events/` — 11 tests (Append, Persist, Read, helpers)
3. ✅ `useGameSocket` hook — 18 tests (dedup, delegation, cleanup, null sockets)
4. ~~`shared/config/env.go` + `shared/errors/apierrors.go`~~ — skipped, trivial (constants + os.Exit)
5. `ws-gateway/handler/` — existing tests already cover presence + origin; WS handlers need e2e
6. ✅ `game-server/platform/grpc/` — 4 tests added (StartSession validation, GetRoomPlayers)
7. ✅ `notification-service/publisher/` — 2 tests (publish, Redis down)
8. ✅ `usePauseResume` hook — 10 tests (state, WS setters, mutations)
9. ✅ `useRematch` hook — 5 tests (state, WS events, navigation, mutation)
10. ✅ `useGameSession` hook — 6 tests (fetch, game-over detection, error)
11. ✅ `sessions` API client — 10 tests (all endpoints, methods, bodies)
12. ✅ `rooms` + `bots` API client — 12 tests (CRUD, pagination, bots)
13. ✅ `ws.ts` RoomSocket + PlayerSocket — 15 tests (backoff, max retries, auth refresh/redirect, cleanup)
14. ✅ `lobby` API client — 6 tests (leaderboard, gameRegistry, queue CRUD)
15. ✅ `friends` API client — 11 tests (search, CRUD, DM conversations, blocks, presence)
16. ✅ `notifications` API client — 4 tests (list, markRead, accept, decline)

## Remaining

- `user-service/consumer` (needs fake store for achievement evaluation)
- `admin` API client (lower priority — internal tool)
- `auth/Login.tsx` OAuth popup flow
- Game renderers + Card UI kit (visual, low risk)
