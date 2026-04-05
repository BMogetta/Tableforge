# Tech Debt: Pause/Resume & Timeout Bugs

Found during intensive unit testing session (2026-04-05).

---

## Bug 1: `nextPlayerAfter` fails with non-contiguous seats

**File:** `services/game-server/internal/domain/runtime/asynq_handlers.go:370-391`

**Problem:** Uses `(currentSeat + 1) % len(players)` assuming seats are 0,1,2...
If there's a gap (e.g. seats 0, 3), it looks for seat 1 which doesn't exist and returns the same player. On a `lose_turn` timeout, the turn never advances → infinite timeout loop.

**Impact:** Low probability (seats are usually contiguous), high severity if triggered.

**Fix:** Sort players by seat, find current index, return `players[(idx+1) % len]`. ~5 min.

**Test that documents it:** `timeout_test.go:TestNextPlayerAfter_NonContiguousSeats`

---

## Bug 2: Runtime vs Store bot detection inconsistency

**File:** `services/game-server/internal/domain/runtime/runtime_pause.go:55-61`

**Problem:** Runtime uses `svc.bots.get()` (in-memory registry) to determine if a player is a bot. The PG store uses `players.is_bot` column. After a server restart, if bots aren't re-registered in the runtime, they're counted as humans.

**Impact:** In a 1v1 vs bot after server restart, pause/resume consensus can never be reached because the runtime expects 2 human votes but the bot never votes.

**Fix:** Have the runtime check `store.GetPlayer().IsBot` as fallback when `bots.get()` returns false. Or ensure bots are always re-registered on startup before any session resumes. ~15-20 min, need to verify startup flow.

**Test that documents it:** `runtime_pause_intensive_test.go:TestPause_BotNotRegistered_InconsistentCount` (comment block)

---

## Bug 3: VoteReady counts bot votes toward human threshold

**File:** `services/game-server/internal/platform/store/pg_ready.go:29-43`

**Problem:** The PG query compares `array_length(ready_players, 1)` (includes bot entries) against `COUNT(*) WHERE is_bot = FALSE` (humans only). When `StartSession` auto-confirms bots, a 1v1 human vs bot triggers immediate consensus (1 bot vote >= 1 human threshold) before the human confirms ready.

**Impact:** Game starts before human loads assets. The human sees the game begin without having clicked ready.

**Fix option A:** Change `StartSession` to not evaluate consensus after bot auto-confirm — let the human's `VoteReady` call trigger it. Simplest, ~10 min.

**Fix option B:** Change PG query to count only human entries in the array (cross-ref with players table). More correct but heavier SQL. ~15 min.

Recommend option A.

---

## What was already fixed

- **FakeStore `checkAllVotedByID`** now excludes bots via `countHumanPlayers()`, matching PG behavior.
- **FakeStore `ResumeSession`** now adjusts `LastMoveAt` to `NOW() - 60% * timeout`, matching PG penalty calculation.
