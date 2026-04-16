# Audit: Game Engine

**Date:** 2026-04-16

## Architecture Overview

The engine is split into three layers:

| Layer | Package | Role |
|---|---|---|
| **Interface** | `engine/` | `Game` interface: `Init`, `ValidateMove`, `ApplyMove`, `IsOver` |
| **Runtime** | `runtime/` | Orchestration: calls engine, persists moves, schedules timers, broadcasts |
| **Handlers** | `api/`, `grpc/` | HTTP/gRPC transport — decodes requests, calls runtime, writes responses |

Game plugins (`tictactoe`, `rootaccess`) implement `engine.Game`.

---

## Issues Found

### Issue 1 — RootAccess `ApplyMove` generates randomness internally

**Principle violated:** Determinism and randomness / Pure state machine

**Where:** `games/rootaccess/rootaccess.go:711-717` (`dealRound`)

```go
func dealRound(state engine.GameState, players []string, firstPlayer engine.PlayerID) engine.GameState {
    unshuffled := buildDeck()
    deck, err := randutil.Shuffle(unshuffled)
```

`dealRound` is called from `resolveRound` (line 593), which is called from
`advanceTurn`, which is called from `ApplyMove`. This means `Game.ApplyMove`
is **non-deterministic**: the same state + same move can produce different results.

This also breaks `GetStateAt` (`runtime.go:307-355`), which replays moves via
`eng.Init` + `eng.ApplyMove` — each replay shuffles differently, so the
reconstructed state diverges from what actually happened in the game.

**Fix:** Resolve the shuffle at the runtime layer and pass it into the move
payload, or pre-generate and store the deck order in state at the start of
each round:

```go
// Option A: runtime resolves randomness, injects via payload
payload["shuffled_deck"] = randutil.Shuffle(buildDeck())

// Option B: engine stores permutation seed in state for reproducibility
state.Data["round_seed"] = seed  // set by runtime before calling ApplyMove
```

Option B is simpler — it lets `dealRound` use a seeded PRNG derived from
`round_seed`, making replays deterministic while keeping the shuffle inside
the game plugin.

---

### Issue 2 — `applyLoseTurn` bypasses the engine entirely

**Principle violated:** Pure state machine / Validation before application / Separation of concerns

**Where:** `runtime/asynq_handlers.go:162-205` (`applyLoseTurn`)

```go
func (h *TimerHandlers) applyLoseTurn(...) {
    state.CurrentPlayerID = nextPlayerAfter(timedOutPlayer, players)
    stateJSON, _ := json.Marshal(state)
    h.st.UpdateSessionState(ctx, session.ID, stateJSON)
```

This code path:
1. Mutates `GameState` directly without going through `Game.ApplyMove`
2. Uses `nextPlayerAfter` (runtime's own rotation logic) instead of the game plugin's turn logic
3. Writes state to DB without recording a `Move` row — the move log has a gap
4. Does not call `ValidateMove`
5. Does not call `Game.IsOver` — a game could be in a terminal state and the runtime wouldn't detect it

In contrast, `applyEngineTimeout` (line 210) correctly delegates to `svc.ApplyMove`.

**Fix:** All timeout penalties should go through the engine. Games that need
custom timeout behavior implement `TurnTimeoutHandler` (RootAccess already does).
For games that don't, add a platform-level `TimeoutMove` that the engine applies:

```go
// In onTimeout, replace the switch with:
func (h *TimerHandlers) onTimeout(sessionID uuid.UUID) {
    // ... load session, state, game ...

    var payload map[string]any
    if th, ok := game.(engine.TurnTimeoutHandler); ok {
        payload = th.TimeoutMove()
    } else {
        payload = map[string]any{"timeout_action": cfg.TimeoutPenalty}
    }
    h.applyEngineTimeout(ctx, session, timedOutPlayerID, payload)
}
```

This eliminates the `applyLoseTurn` and `applyLoseGame` code paths entirely.

---

### Issue 3 — Turn enforcement is delegated to game plugins

**Principle violated:** Turn enforcement

**Where:**
- `games/tictactoe/tictactoe.go:63`: `if move.PlayerID != state.CurrentPlayerID`
- `games/rootaccess/rootaccess.go:80`: `if move.PlayerID != state.CurrentPlayerID`

Every game plugin must independently check whose turn it is. The runtime
(`runtime.ApplyMove` at line 175) passes the move straight to
`game.ValidateMove` without verifying turn order first.

If a new game plugin forgets this check, any player can move at any time.

**Fix:** Add a turn check in the runtime before calling the plugin:

```go
// runtime.go ApplyMove, before line 175
if engine.PlayerID(playerID.String()) != state.CurrentPlayerID {
    return MoveResult{}, ErrNotYourTurn
}
```

Game plugins can then remove their duplicate checks. The runtime owns turn
enforcement; the plugins own rule enforcement.

---

### Issue 4 — Move persistence is not atomic

**Principle violated:** Atomicity

**Where:** `runtime/runtime.go:194-209`

```go
svc.store.RecordMove(ctx, ...)     // line 194 — INSERT into moves
svc.store.UpdateSessionState(...)  // line 204 — UPDATE game_sessions
svc.store.TouchLastMoveAt(...)     // line 207 — UPDATE game_sessions
```

These are three separate SQL statements without a transaction. A crash between
`RecordMove` and `UpdateSessionState` would leave the move recorded but the
session state stale. The next `GetStateAt` replay (which skips `RecordMove`
and trusts `session.State`) would diverge from the move log.

The same pattern repeats in the game-over path (lines 222-236):
`FinishSession`, `UpdateRoomStatus`, `CreateGameResult` — all separate writes.

**Fix:** Wrap the critical writes in a single database transaction:

```go
err := svc.store.WithTx(ctx, func(tx store.Store) error {
    if _, err := tx.RecordMove(ctx, params); err != nil {
        return err
    }
    if err := tx.UpdateSessionState(ctx, sessionID, stateJSON); err != nil {
        return err
    }
    if err := tx.TouchLastMoveAt(ctx, sessionID); err != nil {
        return err
    }
    return nil
})
```

The `Store` interface needs a `WithTx` method. The PGStore already uses `pgxpool`,
which supports `pool.BeginTx`.

---

### Issue 5 — `GET /session` returns unfiltered state

**Principle violated:** Projection

**Where:** `api/api_session.go:162-179` (`handleGetSession`)

```go
session, state, result, err := rt.GetSessionAndState(r.Context(), sessionID)
// ...
writeJSON(w, http.StatusOK, SessionResponse{
    Session: sessionToDTO(session),
    State:   state,       // <-- full unfiltered state
    Result:  result,
})
```

The `BroadcastMove` path correctly applies `StateFilter` per-player (runtime.go:107-143),
but the REST endpoint for fetching the current session state returns the raw,
unfiltered `GameState` including all players' hands.

Any authenticated player who polls `GET /sessions/{id}` can see opponents' hidden
cards.

**Fix:** Apply `StateFilter` in `handleGetSession` using the caller's identity:

```go
callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
game, err := registry.Get(session.GameID)
if err == nil {
    if sf, ok := game.(engine.StateFilter); ok {
        state = sf.FilterState(state, engine.PlayerID(callerID.String()))
    }
}
```

The handler needs access to the game registry (currently only receives `*runtime.Service`).

---

### Issue 6 — Timeout handlers broadcast unfiltered state

**Principle violated:** Projection

**Where:**
- `runtime/asynq_handlers.go:149-158` (`applyLoseGame`) — broadcasts raw `state` to the entire room
- `runtime/asynq_handlers.go:194-202` (`applyLoseTurn`) — broadcasts raw `state` via `TimeoutResult`

Both paths use `hub.Broadcast` which sends the same payload to every client.
Games with hidden information (RootAccess) leak all players' hands on timeout.

Note: `applyEngineTimeout` (line 229) correctly uses `svc.BroadcastMove` which
applies `StateFilter`. Only the two direct-broadcast paths are affected.

**Fix:** If Issue 2 is resolved (routing all timeouts through the engine),
these code paths are eliminated. Otherwise, replace `hub.Broadcast` with
`svc.BroadcastMove` which already handles per-player filtering.

---

### Issue 7 — `GetStateAt` replay is broken by non-deterministic Init

**Principle violated:** State integrity

**Where:** `runtime/runtime.go:327-329`

```go
state, err := eng.Init(players)
```

`GetStateAt` reconstructs state by calling `eng.Init` then replaying moves.
For RootAccess, `Init` calls `dealRound` which shuffles the deck randomly
(Issue 1). The reconstructed state will have a different deck order than the
original game.

Additionally, even if `Init` were deterministic, `ApplyMove` calls
`resolveRound` → `dealRound` which shuffles again (Issue 1). So replaying
past a round boundary produces divergent state.

**Fix:** This is resolved by fixing Issue 1 (deterministic shuffles). Once
shuffles are seeded or externalized, `GetStateAt` becomes reliable.

As an additional safety net, consider validating the replayed state against the
stored `state_after` from the move log:

```go
// After applying move m:
if !bytes.Equal(mustMarshal(state), m.StateAfter) {
    return state, fmt.Errorf("replay diverged at move %d", m.MoveNumber)
}
```

---

### Issue 8 — `runtime_bot.go` uses `math/rand` for jitter

**Principle violated:** Determinism and randomness (minor — cosmetic, not gameplay)

**Where:** `runtime/runtime_bot.go:140`

```go
target := botMinMoveDelay + time.Duration(rand.Int63n(int64(botMoveDelayJitter)))
```

Uses the unseeded `math/rand` global source. This does not affect game state
(it's purely a timing delay), but it's inconsistent with the project's pattern
of using `randutil` for all randomness.

**Fix:** Use `randutil.Intn` or `crypto/rand` for consistency:

```go
jitter, _ := randutil.Intn(int(botMoveDelayJitter.Milliseconds()))
target := botMinMoveDelay + time.Duration(jitter)*time.Millisecond
```

---

## What's Clean

| Principle | Status |
|---|---|
| **Separation of concerns** | Engine package has zero knowledge of transport or persistence. Runtime is the only place these meet. Game plugins are pure. |
| **Validation before application** | `runtime.ApplyMove` correctly calls `ValidateMove` before `ApplyMove` (lines 175-179). No path exists where an invalid move reaches `ApplyMove` through the normal flow. |
| **Serialization** | `GameState.Data` is `map[string]any` — fully JSON round-trippable. All models carry JSON tags. No functions or interfaces embedded in state. |
| **Projection mechanism** | `StateFilter` interface exists and is correctly used in `BroadcastMove`. RootAccess implements it with per-player hand filtering, debugger choice hiding, and private reveals. |

---

## Summary Table

| # | Issue | Principle | Severity | File:Line |
|---|---|---|---|---|
| 1 | `dealRound` shuffles inside `ApplyMove` | Determinism | **High** | `rootaccess.go:711` |
| 2 | `applyLoseTurn` bypasses engine | Pure state machine | **High** | `asynq_handlers.go:162` |
| 3 | Turn check in plugins, not runtime | Turn enforcement | **Medium** | `runtime.go:175` |
| 4 | RecordMove + UpdateState not atomic | Atomicity | **High** | `runtime.go:194-209` |
| 5 | GET /session returns unfiltered state | Projection | **High** | `api_session.go:169` |
| 6 | Timeout broadcasts unfiltered state | Projection | **Medium** | `asynq_handlers.go:149,194` |
| 7 | GetStateAt broken by non-determinism | State integrity | **Medium** | `runtime.go:327` |
| 8 | `math/rand` in bot jitter | Determinism | **Low** | `runtime_bot.go:140` |

## Priority Order

1. **Issue 5** — GET /session leaks hidden state (security, exploitable now)
2. **Issue 4** — Non-atomic persistence (data corruption on crash)
3. **Issue 2** — `applyLoseTurn` bypasses engine (ghost state mutations)
4. **Issue 1** — Non-deterministic ApplyMove (blocks replay, breaks GetStateAt)
5. **Issue 3** — Turn enforcement in plugins (defense-in-depth)
6. **Issue 6** — Timeout broadcasts unfiltered (resolved by Issue 2)
7. **Issue 7** — GetStateAt broken (resolved by Issue 1)
8. **Issue 8** — `math/rand` in bot (cosmetic consistency)
