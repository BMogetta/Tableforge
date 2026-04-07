# E2E Test Report — 2026-04-05

## Summary

| Status | Count |
|--------|-------|
| Passed | 43 |
| Failed | 12 |
| Timed out | 1 |
| Skipped (dep failed) | 10 |
| **Total** | **66** |

The 10 skipped tests are all in `session-history.spec.ts` — they depend on a
prior test (`two players can play a full game`) that fails, so they never run.

---

## Results by file

### auth.setup.ts — 11/11 passed

All 11 player authentications pass.

### auth.spec.ts — 0/1 passed

| Test | Status | Error |
|------|--------|-------|
| error boundary catches render errors and shows recovery UI | FAIL | `getByRole('button', { name: 'Try Again' })` not visible |

**Root cause:** The error boundary likely doesn't render a "Try Again" button, or
the button text/role changed. UI mismatch.

### bot.spec.ts — 1/2 passed

| Test | Status | Error |
|------|--------|-------|
| bot plays a full game against a human | PASS | — |
| bot can be removed and replaced before starting the game | TIMEOUT | 60s exceeded |

**Root cause:** The bot remove/replace flow hangs — likely a UI state issue where
the "remove bot" or "add bot" button doesn't appear or the room state doesn't
update after removal.

### chat.spec.ts — 2/5 passed

| Test | Status | Error |
|------|--------|-------|
| chat popover can be toggled open and closed | PASS | — |
| empty message cannot be sent | PASS | — |
| message sent by P1 appears in P2 sidebar via WS | FAIL | `text=hello from p1` not visible in messages container |
| multiple messages from both players appear in correct order | FAIL | `text=msg-alpha` not visible in messages container |
| messages persist after HTTP resync | FAIL | `text=persistent-msg` not visible in messages container |

**Root cause:** All 3 failures share the same pattern: messages are sent but never
appear in the `[class*="messages"]` container. Likely causes:
1. Chat message delivery via WS is broken (ws-gateway or chat-service)
2. The CSS class selector `[class*="messages"]` no longer matches (CSS Modules hash changed)
3. The chat sidebar doesn't render received messages

### game.spec.ts — 5/7 passed

| Test | Status | Error |
|------|--------|-------|
| two players can play a full game | FAIL | `start-game-btn` remains disabled |
| turn timeout ends the game | FAIL | `game-status` testId never becomes visible |
| player can forfeit a game | PASS | — |
| both players can rematch after a game ends | PASS | — |
| rematch full flow | PASS | — |
| back to lobby button after game ends | PASS | — |
| occupied cell is disabled and cannot be clicked | PASS | — |

**Root cause — `two players can play a full game`:** The `start-game-btn` stays
disabled after P2 joins. This is the same bug as in `lobby.spec.ts` (see below).
Since forfeit/rematch tests pass (they use a different join flow), this points to
a race condition in the room join → ready state propagation.

**Root cause — `turn timeout ends the game`:** The `game-status` element never
appears. This test depends on the game starting, which depends on the join bug
above, OR the turn timer Asynq task isn't firing in test mode.

### lobby.spec.ts — 5/6 passed

| Test | Status | Error |
|------|--------|-------|
| two players can join the same room | FAIL | `start-game-btn` remains disabled |
| player leaving room disables start button | PASS | — |
| room disappears from lobby after game ends | PASS | — |
| owner leaving transfers host | PASS | — |
| last player leaving closes the room | PASS | — |
| non-owner leaving does not change host | PASS | — |

**Root cause:** Same as `game.spec.ts` — P2 joins but `start-game-btn` stays disabled.
The button enables when `room.players.length >= 2`. Either P2's join isn't
reflected in the room state WS event, or there's a timing issue between the HTTP
join and the WS player_joined event.

### presence.spec.ts — 4/5 passed

| Test | Status | Error |
|------|--------|-------|
| presence dot shown in room player list | PASS | — |
| presence dot goes offline when player leaves room | PASS | — |
| opponent presence indicator shown during game | PASS | — |
| opponent presence goes offline when opponent disconnects | PASS | — |
| opponent presence text updates correctly | FAIL | Shows "Opponent offline" instead of "Opponent online" |

**Root cause:** The reconnection/presence update isn't propagating in time. The test
expects the opponent to show as "online" but the presence state still reads "offline".
Likely a WS reconnection timing issue — the presence heartbeat hasn't fired yet
when the assertion runs.

### session-history.spec.ts — 0/10 (all skipped)

All 10 tests skipped because they depend on `two players can play a full game`
which fails. These tests are **not independently broken** — they need the join
bug fixed first.

### settings.spec.ts — 7/7 passed

All pass.

### spectator.spec.ts — 6/10 passed

| Test | Status | Error |
|------|--------|-------|
| spectator is rejected when allow_spectators is "no" | PASS | — |
| spectator can join a room with allow_spectators "yes" | PASS | — |
| spectator count updates when a spectator joins/leaves | FAIL | `spectator-count` testId not visible |
| spectator sees game board but cannot make moves | FAIL | `data-cell="0"` not visible |
| spectator sees live move updates via WS | FAIL | URL stays at `/rooms/...`, never navigates to `/game/...` |
| spectator does not see a rematch button after the game ends | FAIL | URL stays at `/rooms/...`, never navigates to `/game/...` |
| private room code is hidden in the lobby list | PASS | — |
| private room has no direct join button in the lobby | PASS | — |
| private room can be joined by entering the code manually | PASS | — |
| private room owner sees the code inside the room view | PASS | — |
| public room shows code and join button in lobby | PASS | — |
| private room setting can be changed back to public | PASS | — |

**Root cause:** The 4 spectator failures share a pattern — the spectator never
transitions from `/rooms/{id}` to `/game/{id}`. This means the spectator isn't
receiving the `game_started` WS event, or the frontend routing for spectators
doesn't trigger navigation on game start.

---

## Failure clusters

The 13 failures group into **5 distinct issues**:

| # | Issue | Tests affected | Priority |
|---|-------|---------------|----------|
| 1 | **P2 join doesn't enable start button** | lobby/join, game/full-game, game/timeout + 10 skipped session-history | HIGH — blocks 13 tests |
| 2 | **Chat messages not rendering** | chat/ws, chat/order, chat/resync | MEDIUM — 3 tests |
| 3 | **Spectator not navigating to /game/** | spectator/board, spectator/moves, spectator/rematch, spectator/count | MEDIUM — 4 tests |
| 4 | **Presence text not updating** | presence/text | LOW — 1 test |
| 5 | **Error boundary UI mismatch** | auth/error-boundary | LOW — 1 test |
| 6 | **Bot remove/replace timeout** | bot/remove-replace | LOW — 1 test |

Fixing issue **#1** alone would unblock 13 tests (from 43 passing to potentially 53).
