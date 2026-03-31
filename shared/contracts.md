# Tableforge — Service Contracts

Complete map of inter-service communication: what's gRPC, what's async,
and what stays internal to each service.

---

## Communication model

```
Frontend  ←──── REST/JSON ──────►  API Gateway (Traefik)
                                         │
                          ┌──────────────┼──────────────────┐
                          ▼              ▼                   ▼
                    auth-service   match-service        game-service
                          │              │                   │
                          └──── gRPC ────┘                   │
                                │                            │
                          user-service ◄──── gRPC ───────────┘
                                │
                    ────────────┴──────────────────────────────
                                Redis Pub/Sub (async events)
                    ────────────────────────────────────────────
                          │              │                   │
                    auth-service   match-service   user-service
```

**Three communication layers:**

1. **REST/JSON over HTTP** — frontend ↔ services via Traefik. Never between services.
2. **gRPC + Protobuf** — synchronous calls where the caller needs a response to continue.
3. **Redis Pub/Sub + JSON** — async events where the publisher doesn't need to wait.

---

## gRPC contracts

### `rating.v1.RatingService`
**Owner:** user-service
**Callers:** match-service

```protobuf
rpc GetRating(GetRatingRequest)   returns (GetRatingResponse)
rpc GetRatings(GetRatingsRequest) returns (GetRatingsResponse)
```

**When:** A player joins the ranked queue. match-service needs the current
MMR to assign a score to `queue:ranked` ZSET. Called synchronously because
the enqueue response must include the player's position.

**Proto file:** `shared/proto/rating/v1/rating.proto`

---

### `lobby.v1.LobbyService`
**Owner:** match-service
**Callers:** match-service (internal — queue calls lobby within the same service boundary)

> **Note:** In the current monolith, queue.go imports domain/lobby directly.
> When match-service is extracted, CreateRankedRoom stays internal to it.
> This proto is only needed if lobby is split into its own service later.
> Document it now, implement it later.

```protobuf
rpc CreateRankedRoom(CreateRankedRoomRequest) returns (CreateRankedRoomResponse)
```

**Proto file:** `shared/proto/lobby/v1/lobby.proto`

---

### `game.v1.GameService`
**Owner:** game-service
**Callers:** match-service

```protobuf
rpc StartSession(StartSessionRequest)   returns (StartSessionResponse)
rpc GetMoveLog(GetMoveLogRequest)       returns (GetMoveLogResponse)
```

**StartSession — when:** Both players accept the match. match-service needs
the session_id to include in the `match.ready` event. Synchronous because
the room must exist before the event fires.

**GetMoveLog — when:** replay-service needs the ordered move list to serve
a replay. Synchronous because the HTTP response depends on it.

**Proto file:** `shared/proto/game/v1/game.proto`

---

## Async event contracts

All events flow through Redis Pub/Sub. Channel naming: `{domain}.{entity}.{verb}`.
Every event carries `event_id` (UUID) for idempotency, `occurred_at`, and `version`.

### `game.session.finished`
**Publisher:** game-service
**Consumers:**

| Consumer | Action |
|---|---|
| user-service | Update `ratings` + `rating_history` (ranked only) |
| replay-service | Close replay record, mark session as complete |
| match-service | Unblock room, allow rematch queue re-entry |

**Payload:** session_id, room_id, game_id, mode, ended_by, winner_id,
is_draw, duration_secs, players[]

**Why async:** game-service doesn't need to wait for ratings to update
before the game ends. Eventual consistency is fine here.

---

### `game.move.applied`
**Publisher:** game-service
**Consumers:** replay-service (optional — stream flush already covers this)

**Payload:** session_id, game_id, player_id, move_number, payload

**Note:** This event is optional for now. The `session_events` Redis stream
is already flushed to Postgres on session end. Only add this if a consumer
needs real-time move notifications outside the WS layer.

---

### `match.found`
**Publisher:** match-service
**Consumers:** ws hub → delivers `match_found` WS event to both players

**Payload:** match_id, room_id, room_code, game_id, player_a_id,
player_b_id, mmr_a, mmr_b

**Why async:** match-service doesn't own the WS connections. The hub
subscribes to `ws:player:{id}` channels and forwards to the client.

---

### `match.cancelled`
**Publisher:** match-service
**Consumers:** ws hub → delivers `match_cancelled` WS event

**Payload:** match_id, player_a_id, player_b_id, reason

---

### `match.ready`
**Publisher:** match-service (after both players accept + StartSession succeeds)
**Consumers:** ws hub → delivers `match_ready` WS event with session_id

**Payload:** match_id, session_id, room_id, room_code, player_a_id, player_b_id

---

### `player.banned`
**Publisher:** user-service
**Consumers:**

| Consumer | Action |
|---|---|
| auth-service | Revoke active session, publish `player.session.revoked` |
| match-service | Remove from queue if present |
| game-service | Forfeit any active ranked sessions |

**Payload:** player_id, ban_id, banned_by, reason, expires_at

**Why async:** Banning is not a hot path. Eventual consistency (player might
play one more move before the ban propagates) is acceptable.

---

### `player.unbanned`
**Publisher:** user-service
**Consumers:** none currently (auth-service takes no action on unban)

---

### `friendship.accepted`
**Publisher:** user-service
**Consumers:** notification-service → in-app notification to requester

**Payload:** requester_id, addressee_id

---

### `player.session.revoked`
**Publisher:** auth-service
**Consumers:** ws hub → closes the WebSocket connection for that session

**Payload:** player_id, session_id, reason ("logout" | "superseded" | "banned")

**Why this matters:** Without this event, a player who is banned or logs in
on another device keeps their existing WS connection alive indefinitely.

---

## What stays internal (no contract needed)

These are calls that happen within a single service boundary. They don't
need gRPC or events — they're just function calls or DB queries.

| Call | Service | Why internal |
|---|---|---|
| Create room for casual match | match-service | same service owns lobby |
| Validate JWT on every request | auth-service | middleware, not cross-service |
| Apply a move to game state | game-service | pure in-process logic |
| Calculate ELO delta | user-service | triggered by `game.session.finished` |
| Create notification row | user-service | triggered by `friendship.accepted` |
| Flush session_events to Postgres | game-service | internal cleanup on session end |

---

## Monorepo structure for shared contracts

```
tableforge/
  shared/
    proto/
      rating/v1/rating.proto       ← rating.v1.RatingService
      lobby/v1/lobby.proto         ← lobby.v1.LobbyService
      game/v1/game.proto           ← game.v1.GameService
      gen/                         ← generated Go code (buf generate)
        rating/v1/
        lobby/v1/
        game/v1/
    events/
      events.go                    ← all async event structs
    middleware/
      auth.go                      ← JWT validator (shared by all services)
      tracing.go                   ← OTel middleware
  services/
    auth-service/
    user-service/
    match-service/
    game-service/
    chat-service/
    replay-service/
```

Each service imports `shared` as a local Go module (`replace` directive
in `go.mod` during monorepo phase). When a service is split to its own
repo, the `shared` module is published separately and imported by version.

---

## buf.gen.yaml (code generation)

```yaml
version: v2
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: shared/proto/gen
    opt:
      - paths=source_relative
  - plugin: buf.build/grpc/go
    out: shared/proto/gen
    opt:
      - paths=source_relative
```

Run with: `buf generate shared/proto`

This generates the Go structs and gRPC client/server interfaces from the
`.proto` files. Commit the generated code — don't regenerate in CI unless
you have a lint step that fails on drift.

---

## Event versioning rules

1. **Adding a field** — safe, always backward compatible. Consumers that
   don't know the field ignore it (JSON unknown fields are dropped).

2. **Renaming a field** — breaking change. Add a new channel version:
   `game.session.finished.v2`. Run both channels in parallel until all
   consumers migrate, then deprecate v1.

3. **Removing a field** — breaking change. Same process as rename.

4. **Never reuse a field name with a different type.**

For Protobuf: field numbers are the contract, not names. Never reuse a
field number. Adding fields with new numbers is always safe.
