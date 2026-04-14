# Recess — Redis Key Map

Complete inventory of all Redis keys, structures, TTLs, and ownership.
Derived from source code audit (queue.go, turn_timer.go, presence.go,
hub.go, hub_player.go, events/events.go, ratelimit/script_runner.go).

---

## Namespace summary

| Namespace         | Structure     | TTL          | Owner domain     |
|-------------------|---------------|--------------|------------------|
| `presence:`       | STRING        | 30s (sliding)| presence         |
| `timer:session:`  | STRING        | = turn limit | runtime          |
| `timer:ready:`    | STRING        | 60s          | runtime          |
| `events:session:` | STREAM        | none (manual)| events           |
| `queue:ranked`    | ZSET          | none         | queue            |
| `queue:meta:`     | HASH          | none         | queue            |
| `queue:pending:`  | HASH          | ~30s         | queue            |
| `queue:declines:` | HASH          | ~30s         | queue            |
| `queue:ban:`      | STRING        | ban duration | queue            |
| `queue:lock`      | STRING        | short (~5s)  | queue            |
| `bot:known`       | SET           | none         | bot backfill     |
| `bot:available`   | SET           | none         | bot backfill     |
| `bot.activate`    | PUB/SUB chan  | ephemeral    | bot backfill     |
| `ws:room:`        | PUB/SUB chan  | ephemeral    | ws hub           |
| `ws:player:`      | PUB/SUB chan  | ephemeral    | ws hub           |
| `rl:`             | ZSET          | sliding      | ratelimit        |
| `rl:auth:`        | ZSET          | sliding      | ratelimit        |
| `rl:move:`        | ZSET          | sliding      | ratelimit        |
| `session:active:` | STRING        | = JWT expiry | auth (NEW)       |

---

## Keys in detail

### `presence:{player_id}`
- **Type:** STRING
- **Value:** `"1"`
- **TTL:** 30s, refreshed on every WebSocket ping/pong cycle
- **Set by:** `presence.Store.Set()` on WS connect and each pong
- **Deleted by:** `presence.Store.Delete()` on WS disconnect
- **Read by:** `presence.Store.IsOnline()`, `presence.Store.FilterOnline()`
- **Purpose:** Lightweight online indicator. Expires naturally if the server
  crashes mid-session without sending a disconnect event.
- **Example:** `presence:8026de9e-1a40-4396-ab00-0a8dca6e130d` → `"1"`

---

### `timer:session:{session_id}`
- **Type:** STRING
- **Value:** `"{session_id}"` (the session UUID as string)
- **TTL:** Remaining turn time in seconds (set per turn, reset on each move)
- **Set by:** `TurnTimer.Schedule()` on each new turn start
- **Deleted by:** `TurnTimer.Cancel()` when a move is received before timeout
- **Read by:** `TurnTimer.ReschedulePending()` on server restart (recovery)
- **Purpose:** Turn timeout trigger via Redis keyspace notifications
  (`__keyevent@0__:expired`). When the key expires, the subscriber fires
  the timeout penalty for the current player.
- **Stateless caveat:** The goroutine that acts on expiry lives in-process.
  Safe for single instance. Multi-instance requires a distributed scheduler
  (e.g. Asynq, or a single timer-worker service).
- **Example:** `timer:session:60bbdc66-f4df-4555-8712-8717ec81a5f8` → `"60bbdc66-..."`

---

### `timer:ready:{session_id}`
- **Type:** STRING
- **Value:** `"{session_id}"`
- **TTL:** 60s (ready handshake timeout)
- **Set by:** `TurnTimer.ScheduleReady()` when session enters ready-wait state
- **Deleted by:** `TurnTimer.CancelReady()` when all players send `player_ready`
- **Purpose:** If not all players confirm asset load within 60s, the game
  starts anyway (bots/slow clients don't block the room indefinitely).
- **Example:** `timer:ready:a93fd47b-7d68-4df7-bbc1-ca2a2e757388`

---

### `events:session:{session_id}`
- **Type:** STREAM
- **TTL:** None — deleted manually after bulk-flush to Postgres
- **Appended by:** `events.Store.Append()` → `XADD events:session:{id} * ...`
- **Read by:** `events.Store.Flush()` → bulk-inserts to `session_events` table
- **Deleted by:** `events.Store.Flush()` after successful Postgres insert
- **Fields per entry:**
  ```
  event_type   string    e.g. "move_made", "turn_timeout", "game_finished"
  player_id    string    UUID or empty string for system events
  payload      string    JSON-encoded event payload
  occurred_at  string    RFC3339 timestamp
  ```
- **Purpose:** Low-latency event log while the session is live. Acts as a
  buffer between the hot write path and Postgres. The `stream_id` (Redis
  entry ID, e.g. `1700000000000-0`) becomes the idempotency key in
  `session_events.stream_id` — flush is safe to retry.
- **Example:** `events:session:05b891e1-b62b-42da-af72-41e833461fa0`

---

### `queue:ranked`
- **Type:** ZSET
- **Value:** member = `{player_id}`, score = MMR float
- **TTL:** None (entries removed explicitly on dequeue or match)
- **Set by:** `queue.Store.Enqueue()` → `ZADD queue:ranked {mmr} {player_id}`
- **Deleted by:** `queue.Store.Dequeue()` and `FindMatches()` → `ZREM`
- **Read by:** `FindMatches()` → `ZRANGEWITHSCORES` to find closest MMR pairs
- **Purpose:** Matchmaking priority queue ordered by MMR. `ZRANGEBYSCORE`
  with a sliding MMR window finds opponents within rating range.
- **Note:** A player's position can be checked with `ZRANK queue:ranked {player_id}`.

---

### `queue:meta:{player_id}`
- **Type:** HASH
- **TTL:** None (deleted when player dequeues or is matched)
- **Fields:**
  ```
  joined_at   string    RFC3339 — used to widen MMR window over time
  mmr         string    float as string — snapshot at enqueue time
  game_id     string    which game they queued for
  ```
- **Set by:** `queue.Store.Enqueue()` → `HSET queue:meta:{id} ...`
- **Deleted by:** `FindMatches()` pipeline after pairing
- **Purpose:** Metadata for the matchmaking algorithm. `joined_at` enables
  wait-time-based MMR expansion (the longer you wait, the wider the range).

---

### `queue:pending:{match_id}`
- **Type:** HASH
- **TTL:** ~30s (accept window — if both players don't accept, match is cancelled)
- **Fields:**
  ```
  player_a    string    UUID
  player_b    string    UUID
  accepted_a  string    "1" if player A accepted, absent if not yet
  accepted_b  string    "1" if player B accepted, absent if not yet
  mmr_a       string    float
  mmr_b       string    float
  ```
- **Set by:** `FindMatches()` after pairing two players
- **Read by:** `AcceptMatch()` to check both players have accepted
- **Deleted by:** TTL expiry (decline path) or after room creation (accept path)
- **Purpose:** Match confirmation handshake. Both players must accept within
  the TTL window or the match is voided and both re-enter the queue.

---

### `queue:declines:{player_id}`
- **Type:** HASH
- **TTL:** ~30s
- **Purpose:** Tracks how many times a player declined a match in the current
  window. Repeated declines trigger a matchmaking ban.

---

### `queue:ban:{player_id}`
- **Type:** STRING
- **Value:** `"1"`
- **TTL:** Ban duration (escalating — longer on repeat offenses)
- **Set by:** `queue.Store.Ban()` after excessive declines
- **Read by:** `queue.Store.IsBanned()` on every enqueue attempt
- **Purpose:** Temporary matchmaking ban for players who repeatedly decline
  matches. Enforced before the player joins the queue.

---

### `queue:lock`
- **Type:** STRING
- **Value:** `"1"`
- **TTL:** ~5s (short — just enough to protect FindMatches execution)
- **Set by:** `FindMatches()` → `SET NX EX queue:lock 1 5`
- **Deleted by:** `defer rdb.Del(ctx, keyMatchmakeLock)` after FindMatches
- **Purpose:** Distributed lock preventing concurrent matchmaking runs.
  Ensures only one instance of `FindMatches` runs at a time across
  multiple server instances.

---

### `bot:known`
- **Type:** SET
- **Value:** member = `{player_id}` (UUID) for every bot-runner instance currently up
- **TTL:** None — bot-runner removes itself via `SREM` on graceful shutdown
- **Set by:** bot-runner at startup (`--mode=backfill`)
- **Read by:** `queue.Service.BackfillScan` — anything in `queue:ranked` that
  is NOT in `bot:known` is treated as a human.
- **Purpose:** Lets match-service classify queued IDs without hitting Postgres
  on every tick. Stale entries (bot-runner killed ungracefully) are benign:
  the detector still works because the bot also won't appear in `bot:available`
  and therefore won't be activated again.
- **Cleanup note:** Crash-left entries are not auto-evicted. A periodic
  `SDIFF bot:known bot:available` + presence check could age them out if the
  set grows unbounded, but at expected bot counts (<100) this is not urgent.

---

### `bot:available`
- **Type:** SET
- **Value:** member = `{player_id}` for bots currently idle and eligible for backfill
- **TTL:** None
- **Set by:**
  - bot-runner at startup (SADD alongside `bot:known`)
  - bot-runner after a match ends (SADD to return to the pool)
- **Deleted by:**
  - `BackfillScan` (SREM when the bot is activated — doubles as an atomic claim)
  - bot-runner on graceful shutdown
- **Read by:** `BackfillScan` — candidate pool for closest-MMR pick.
- **Invariant:** `bot:available ⊆ bot:known`. A bot is never in `available`
  without also being in `known`; the scan relies on this to count active bots
  as `SCARD(known) - SCARD(available)`.

---

### `bot.activate` (Pub/Sub channel)
- **Type:** PUB/SUB channel (no persistence)
- **Payload:** `{"player_id": "<uuid>"}` — JSON
- **Published by:** `queue.Service.BackfillScan` when a lone human has
  waited past `BACKFILL_THRESHOLD_SECS`
- **Subscribed by:** every bot-runner in `--mode=backfill`; each filters by
  its own `player_id` and joins the queue when activated.
- **Purpose:** Fire-and-forget activation signal. Detection + claim
  (SREM on `bot:available`) happen before the publish so at most one bot
  reacts per message.
- **Note:** The dot in `bot.activate` (vs the colon-separated key prefixes
  above) is intentional — dots separate PUB/SUB channel name components in
  the rest of the codebase.

---

### `ws:room:{room_id}` (Pub/Sub channel)
- **Type:** PUB/SUB channel (no persistence, no TTL)
- **Published by:** `hub.Broadcast()`, `hub.BroadcastToRoom()`
- **Subscribed by:** Hub goroutine per room on first WS client connect
- **Purpose:** Fan-out of game events (moves, state changes, presence) to all
  WebSocket clients connected to a room, across multiple server instances.
- **Unsubscribed:** When the last WS client in the room disconnects.

---

### `ws:player:{player_id}` (Pub/Sub channel)
- **Type:** PUB/SUB channel (no persistence, no TTL)
- **Published by:** `hub.SendToPlayer()`, `hub_player.go`
- **Subscribed by:** Hub goroutine per player on WS connect
- **Purpose:** Direct messages to a specific player's WebSocket connection
  (notifications, match found alerts, DM pings) across server instances.

---

### `rl:{ip}` / `rl:auth:{ip}` / `rl:move:{ip}`
- **Type:** ZSET (sliding window log)
- **TTL:** Window duration (`PEXPIRE` set on each request)
- **Script:** Lua atomic sliding window — removes old entries, counts current,
  rejects if over limit, adds new entry.
- **Namespaces:**
  - `rl:{ip}` — global per-IP rate limit
  - `rl:auth:{ip}` — stricter limit on auth endpoints (login, callback)
  - `rl:move:{ip}` — per-IP limit on game move submissions
- **Headers set on response:** `X-RateLimit-Limit`, `X-RateLimit-Remaining`,
  `X-RateLimit-Reset`

---

### `session:active:{player_id}` — NEW, not yet implemented
- **Type:** STRING
- **Value:** `"{session_id}"` (UUID of the active player_sessions row)
- **TTL:** Matches JWT expiry
- **Set by:** Auth service on login — after inserting to `player_sessions`
- **Deleted by:** Auth service on logout
- **Overwritten by:** Next login (automatically revokes previous session)
- **Read by:** Auth middleware on every authenticated request
- **Purpose:** Enforces single active session per player. Auth middleware
  validates that the `session_id` claim in the JWT matches this value.
  Mismatch → 401 (logged in elsewhere).
- **Redis vs DB:** Redis is the hot path check (every request). Postgres
  `player_sessions` is the audit log and the source of truth for the
  "active sessions" UI screen.

---

## Keys to add for new features

These keys don't exist yet but will be needed as you implement the new
schema domains.

### `queue:tournament:{tournament_id}` — future
- **Type:** ZSET or LIST depending on tournament format
- **Purpose:** Tournament bracket queue, separate from ranked queue.

### `leaderboard:{game_id}` — future
- **Type:** ZSET
- **Value:** member = `{player_id}`, score = `display_rating`
- **TTL:** None, updated on each ranked game end
- **Purpose:** `ZREVRANGE leaderboard:tictactoe 0 9` for instant top-10.
  Currently the leaderboard reads from Postgres `ratings` table directly.
  Fine for now, but a Redis mirror is the standard approach at scale.

### `unread:dm:{player_id}` — future (optional)
- **Type:** STRING (counter) or HASH (per-sender counts)
- **Purpose:** Fast unread DM badge count without a Postgres COUNT query
  on every page load. Currently served from DB.

---

## Keyspace notifications requirement

Turn timers rely on Redis keyspace notifications for expiry events.
Ensure this is enabled in your Redis config or Docker Compose:

```yaml
redis:
  image: redis:7-alpine
  command: redis-server --notify-keyspace-events Ex
```

`E` = keyevent events, `x` = expired events.
Without this, `timer:session:*` and `timer:ready:*` keys expire silently
and turn timeouts never fire.

---

## Stateless assessment summary

| Component          | Stateless? | Notes                                        |
|--------------------|------------|----------------------------------------------|
| HTTP handlers      | ✓ Yes      | JWT auth, no in-memory state                 |
| WebSocket hub      | ✓ Yes      | Redis Pub/Sub fan-out, multi-instance safe   |
| Presence           | ✓ Yes      | Redis with sliding TTL                       |
| Rate limiter       | ✓ Yes      | Redis sliding window with Lua                |
| Matchmaking queue  | ✓ Yes      | Distributed lock via SetNX                   |
| Turn timer         | ✗ Partial  | Goroutine lives in-process — single instance |
| Ready timer        | ✗ Partial  | Same as turn timer                           |

**Verdict:** Safe for single-instance deployment (current setup). To scale
horizontally, the timer goroutines need to move to a dedicated worker or
use a distributed job queue (e.g. Asynq with Redis backend).
