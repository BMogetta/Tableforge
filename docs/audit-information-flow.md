# Audit: Client-Server Information Flow

**Date:** 2026-04-16

## Actual Flow (traced end-to-end)

```
 Client                        Server                       All Clients
   |                              |                              |
   |--- POST /sessions/:id/move --->                             |
   |                              |  1. ValidateMove (plugin)    |
   |                              |  2. ApplyMove (plugin)       |
   |                              |  3. RecordMove (DB)          |
   |                              |  4. UpdateSessionState (DB)  |
   |                              |  5. BroadcastMove (WS+filter)|
   |                              |                         ---->| WS: move_applied (filtered)
   |<-- HTTP 200 (full state) ----|                              |
   |                              |                              |
   | Race: HTTP response vs WS event both write to same cache   |
   | Winner determined by move_count deduplication guard         |
```

---

## Issues Found

### Issue 1 — HTTP move response carries full game state, not just an ack

**Principle violated:** Move submission via HTTP / UI state driven by WebSocket events

**Where:**

Server — `api/api_session.go:152-157`:
```go
writeJSON(w, http.StatusOK, MoveResponse{
    Session: sessionToDTO(result.Session),
    State:   result.State,    // full unfiltered engine.GameState
    IsOver:  result.IsOver,
    Result:  result.Result,
})
```

Client — `features/game/Game.tsx:148-178`:
```ts
onSuccess: res => {
  // ...deduplication guard...
  qc.setQueryData(keys.session(sessionId), {
    session: res.session,
    state: res.state,      // writes full state to cache
    result: res.result ? {...} : null,
  })
}
```

The HTTP response is a **full state update**, not an acceptance confirmation.
The client uses it as a primary source of truth — `onSuccess` writes the new
state directly into the React Query cache. The WS event (`useGameSocket.ts:73-78`)
writes to the same cache key. Whichever arrives first wins; the other is
deduplicated by `move_count`.

In practice, the HTTP response usually wins the race (it's on the same TCP
connection that made the request), so **the submitting player's state update
comes from HTTP, not from WS**. The WS event serves as a redundant backup for
the submitter and as the primary update for all other players.

**Impact:**
- The WS-first principle is violated for the submitting player
- The HTTP response carries **unfiltered** state (see Issue 2), while the WS
  event carries properly filtered state via `BroadcastMove` → `StateFilter`

**Fix:** Reduce the HTTP response to an ack:

```go
// Server: return only acceptance signal
type MoveAckResponse struct {
    MoveNumber int  `json:"move_number"`
    IsOver     bool `json:"is_over"`
}
```

```ts
// Client: only clear error on success, let WS drive state
onSuccess: (res) => {
    setMoveError(null)
    // DO NOT write state to cache — wait for WS event
}
```

If latency for the submitter is a concern, apply `StateFilter` to the HTTP
response so at minimum it doesn't leak hidden information (see Issue 2).

---

### Issue 2 — HTTP responses return unfiltered state (security)

**Principle violated:** WebSocket broadcast on move (per-player projection)

**Where:**

POST /move — `api/api_session.go:154`: `State: result.State` (raw, unfiltered)
POST /surrender — `api/api_session.go:272`: `State: result.State`
GET /session — `api/api_session.go:176`: `State: state` (raw, unfiltered)

The `BroadcastMove` function correctly applies `StateFilter` per player
(`runtime.go:137-140`), producing different views for each player. But the HTTP
endpoints bypass filtering entirely — they return the raw `engine.GameState`
including all players' hands, the deck contents, and any hidden information.

For RootAccess, any player can call `GET /sessions/{id}` and see opponents'
cards in the response.

**Fix:** Apply `StateFilter` in every HTTP response that includes game state:

```go
// In handleMove, before writeJSON:
if sf, ok := game.(engine.StateFilter); ok {
    result.State = sf.FilterState(result.State, engine.PlayerID(playerID.String()))
}

// In handleGetSession, same pattern:
callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
if sf, ok := game.(engine.StateFilter); ok {
    state = sf.FilterState(state, engine.PlayerID(callerID.String()))
}
```

---

### Issue 3 — `useGameSession` polls every 3 seconds via HTTP, creating a third state channel

**Principle violated:** WebSocket connection (sole channel for state updates)

**Where:** `features/game/hooks/useGameSession.ts:52-55`:

```ts
refetchInterval: query => {
    const d = query.state.data
    return d?.session?.finished_at ? false : 3000
},
```

While the game is active, the client polls `GET /sessions/{id}` every 3 seconds.
This creates three competing channels for state updates:

1. **WS `move_applied`/`game_over`** events (via `useGameSocket`)
2. **HTTP POST `/move` response** (via `Game.tsx` mutation `onSuccess`)
3. **HTTP GET `/sessions/{id}` polling** (via `useGameSession`)

All three write to the same React Query cache key. The `move_count` guard in
the WS handler and the HTTP mutation prevents regressions, but the polling
response has no such guard — it can overwrite WS-updated cache with stale data
if the poll response is delayed.

Additionally, the polling response is **unfiltered** (Issue 2), so every 3
seconds the client receives raw state including hidden information.

**Fix:** Remove `refetchInterval` and rely on WS events + reconnection fetch:

```ts
const { data, error: loadError } = useQuery({
    queryKey: keys.session(sessionId),
    queryFn: () => sessions.get(sessionId),
    refetchOnWindowFocus: false,
    staleTime: Infinity,  // only refetch on invalidation (reconnect)
    // NO refetchInterval
})
```

The `ws_connected` handler already invalidates the session query
(`useGameSocket.ts:109`), which covers the reconnection case. The only
scenario polling addresses is "WS event lost without a disconnect" — and at
that point the correct fix is WS reliability, not polling.

If polling is kept as a safety net, apply `StateFilter` on the server side
(Issue 2 fix) and add a `move_count` guard in `useGameSession` to prevent
regressions.

---

### Issue 4 — Turn ownership check is inside plugins, not runtime

**Principle violated:** Server-side validation before any state change

**Where:** `runtime/runtime.go:175` calls `game.ValidateMove(state, move)` directly.
Turn check lives in each plugin:
- `games/tictactoe/tictactoe.go:63`
- `games/rootaccess/rootaccess.go:80`

The runtime does not verify turn ownership before delegating to the plugin. A
plugin that forgets the check allows out-of-turn moves.

Also: there is no separate turn check step before move validation. The two are
merged inside `ValidateMove`.

**Fix:** Add a turn check in the runtime before calling the plugin:

```go
// runtime.go, before line 175:
if engine.PlayerID(playerID.String()) != state.CurrentPlayerID {
    return MoveResult{}, ErrNotYourTurn
}
```

---

### Issue 5 — `applyLoseTurn` bypasses engine entirely

**Principle violated:** Server-side validation before any state change

**Where:** `runtime/asynq_handlers.go:162-205`

```go
state.CurrentPlayerID = nextPlayerAfter(timedOutPlayer, players)
stateJSON, _ := json.Marshal(state)
h.st.UpdateSessionState(ctx, session.ID, stateJSON)
```

This path mutates state without calling `ValidateMove` or `ApplyMove`. It
also writes state to DB without recording a move, creating a gap in the
move log.

**Fix:** Route all timeouts through the engine (already detailed in
`docs/audit-game-engine.md`, Issue 2).

---

### Issue 6 — Timeout broadcasts send unfiltered state to all players

**Principle violated:** WebSocket broadcast on move (per-player projection)

**Where:**
- `runtime/asynq_handlers.go:149-158` (`applyLoseGame`): `hub.Broadcast(...)` with raw state
- `runtime/asynq_handlers.go:194-202` (`applyLoseTurn`): `hub.Broadcast(...)` with raw state

Both use `hub.Broadcast` which sends identical payloads to everyone.
`applyEngineTimeout` (line 229) correctly uses `svc.BroadcastMove`.

**Fix:** Use `svc.BroadcastMove` in all timeout paths, or eliminate these
code paths entirely by routing all timeouts through the engine (Issue 5 fix).

---

### Issue 7 — Game-over WS event lacks the reason/ended_by field

**Principle violated:** Game over (event must include the reason)

**Where:**

The `MoveResult` struct broadcast via WS contains:
```go
type MoveResult struct {
    Session store.GameSession
    State   engine.GameState
    IsOver  bool
    Result  *engine.Result  // only Status + WinnerID
}
```

`engine.Result` only carries `Status` (win/draw) and `WinnerID`. It does not
include `ended_by` (normal, timeout, forfeit, ready_timeout, suspended).

The reason IS stored in the database (`store.GameResult.EndedBy`) and in the
event log (`events.TypeGameOver` payload has `ended_by`), but it is **not
included in the WS broadcast** to clients.

Clients cannot distinguish between a normal win, a timeout win, and a forfeit
win from the WS event alone. The only way to get `ended_by` is via
`GET /sessions/{id}` after the game ends.

The timeout handlers partially work around this:
- `applyLoseGame` includes `timed_out_player` in the broadcast payload
- `applyEngineTimeout` does not — it broadcasts a standard `MoveResult`

**Fix:** Add `EndedBy` to `engine.Result` or to the WS broadcast payload:

```go
// Option A: extend engine.Result
type Result struct {
    Status   ResultStatus `json:"status"`
    WinnerID *PlayerID    `json:"winner_id,omitempty"`
    EndedBy  string       `json:"ended_by,omitempty"`  // "normal", "timeout", "forfeit"
}

// Option B: wrap in the broadcast (less invasive)
type GameOverPayload struct {
    MoveResult
    EndedBy string `json:"ended_by"`
}
```

---

### Issue 8 — Surrender response includes full state but only goes to the surrendering player

**Principle violated:** Game over (only communicated to the player who made the final move)

**Where:** `api/api_session.go:243-276`

The surrender handler:
1. Calls `rt.Surrender()` which finishes the session (line 395)
2. Calls `rt.BroadcastMove(...)` with `ws.EventGameOver` (line 268) — broadcasts to all players
3. Returns `MoveResponse` to the HTTP caller (line 270-275)

The WS broadcast IS correct here — all players receive game_over. However:
- The WS broadcast goes through `BroadcastMove` which applies `StateFilter` — this is correct
- But the HTTP response at line 272 returns unfiltered `result.State` — same Issue 2

**Assessment:** The broadcast is correct. The only issue is the unfiltered HTTP
response (already covered by Issue 2).

---

## What's Clean

### WebSocket connection lifecycle (Pass)

- `PlayerSocket` connects on login (`__root.tsx:177`), disconnects on logout (`__root.tsx:94`)
- `RoomSocket` connects on room join (`store.ts:201-210`), disconnects on room leave (`store.ts:212-221`)
- Both have duplicate connection guards
- Both implement exponential backoff reconnection

### Moves submitted via HTTP POST (Pass)

All player actions go through REST:
- `sessions.move()` → `POST /sessions/{id}/move`
- `sessions.surrender()` → `POST /sessions/{id}/surrender`
- `sessions.ready()` → `POST /sessions/{id}/ready`

No game actions are sent through the WebSocket.

### WS broadcast on move (Partial pass)

`BroadcastMove` (`runtime.go:107-143`) correctly:
- Checks if the game implements `StateFilter`
- Produces per-player filtered views via `BroadcastToRoom`
- Sends a spectator view with all hands hidden
- Sends to ALL players in the room (including the move submitter)

Violations are in timeout handlers (Issue 6) and HTTP responses (Issue 2).

### Reconnection (Pass)

- `RoomSocket` and `PlayerSocket` auto-reconnect with exponential backoff
- On `ws_connected`, the game hook invalidates the session query (`useGameSocket.ts:109`)
- This triggers a `GET /sessions/{id}` fetch to resync state
- No full move history replay required — the server stores current state

### Game over broadcast (Partial pass)

Normal game-over (`handleMove` → `BroadcastMove` with `ws.EventGameOver`):
broadcasts to all players with filtered state. Correct.

Timeout game-over (`applyLoseGame`): broadcasts to all players, but unfiltered (Issue 6).

Surrender (`handleSurrender` → `BroadcastMove` with `ws.EventGameOver`):
broadcasts to all players with filtered state. Correct.

---

## Summary Table

| # | Issue | Principle | Severity | Location |
|---|---|---|---|---|
| 1 | HTTP move response carries full state, client uses it as primary update | Move submission / UI state from WS | **High** | `api_session.go:152`, `Game.tsx:148` |
| 2 | HTTP responses return unfiltered state | Per-player projection | **Critical** | `api_session.go:154,176,272` |
| 3 | 3-second polling creates third state channel with unfiltered data | WS as sole channel | **Medium** | `useGameSession.ts:52` |
| 4 | Turn check inside plugins, not runtime | Validation before mutation | **Medium** | `runtime.go:175` |
| 5 | `applyLoseTurn` bypasses engine | Validation before mutation | **High** | `asynq_handlers.go:162` |
| 6 | Timeout broadcasts unfiltered state | Per-player projection | **High** | `asynq_handlers.go:149,194` |
| 7 | Game-over WS event lacks ended_by reason | Game over | **Low** | `engine/game.go:Result` |
| 8 | Surrender HTTP response unfiltered (broadcast is correct) | Per-player projection | **High** | `api_session.go:272` |

## Priority Order

1. **Issue 2** — Unfiltered state in HTTP responses (security, exploitable now via
   GET /session or any move response)
2. **Issue 5** — `applyLoseTurn` bypasses engine (state corruption path)
3. **Issue 6** — Timeout broadcasts unfiltered (fixed by Issue 5 if all timeouts
   go through engine)
4. **Issue 1** — HTTP response as primary state update for submitter (architectural,
   fix after Issue 2 since filtering removes the security concern)
5. **Issue 3** — Polling creates third channel (remove refetchInterval or at
   minimum apply filter on server)
6. **Issue 4** — Turn check in plugins (defense in depth)
7. **Issue 7** — Game-over event lacks reason (UX enhancement)
8. **Issue 8** — Already covered by Issue 2

## Dependency Graph

```
Issue 2 (filter HTTP responses)
  └── Issue 1 becomes less urgent (no info leak even if HTTP response is used)
  └── Issue 3 polling is safe (no unfiltered data)
  └── Issue 8 is resolved

Issue 5 (route timeouts through engine)
  └── Issue 6 is resolved (BroadcastMove applies filtering)
```
