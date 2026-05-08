# Recess — Service Contracts

Complete map of inter-service communication: what's gRPC, what's async,
and what stays internal to each service.

---

## Communication model

```
Frontend  ←──── REST/JSON ──────►  API Gateway (Traefik)
                                         │
          ┌──────────────────────────────┼──────────────────────────────┐
          ▼              ▼               ▼              ▼               ▼
    auth-service   game-server     user-service   rating-service  notification-service
                    gRPC :9080      gRPC :9082     gRPC :9085
                    ┌────┴─────┐
                    ▼          ▼
              match-service  ws-gateway
                    │
              rating-service (gRPC :9085)
         ─────────────────────────────────────────────
                       Async transports
         ─────────────────────────────────────────────
   Streams (fan-out)   Asynq (point-to-point)   Pub/Sub (replica fan-out + per-connection)
        │                       │                            │
   rating, user           bot-runner, notification     ws-gateway (service + per-conn)
```

**Communication layers:**

1. **REST/JSON over HTTP** — frontend ↔ services via Traefik. Never between services.
2. **gRPC + Protobuf** — synchronous calls where the caller needs a response to continue.
3. **Async events** — three transports, picked per use case (see "Async event contracts" below):
   - **Redis Streams** — fan-out events (one publisher → multiple consuming services), each service uses its own consumer group, each event consumed once across the group's replicas.
   - **Asynq** — point-to-point tasks (one publisher → one consuming service), with built-in retry, scheduling, DLQ.
   - **Redis Pub/Sub** — fan-out where every consumer **replica** needs the message (per-connection WS hubs, ws-gateway service consumer); also legacy / unwired channels.

---

## Traefik routing

| Priority | Path pattern | Service |
|----------|-------------|---------|
| 150 | `/ws/*` | ws-gateway |
| 100 | `/auth/*`, `/api/v1/me` | auth-service |
| 50 | `/api/v1/players/{id}/(profile\|friends\|block\|mute\|mutes\|achievements\|report\|settings)` | user-service |
| 50 | `/api/v1/admin/(players\|allowed-emails\|bans\|reports)` | user-service |
| 50 | `/api/v1/rooms/{id}/messages`, `/api/v1/players/{id}/dm`, `/api/v1/dm` | chat-service |
| 50 | `/api/v1/ratings/*`, `/api/v1/players/{id}/ratings/*` | rating-service |
| 50 | `/api/v1/players/{id}/notifications`, `/api/v1/notifications/*` | notification-service |
| 50 | `/api/v1/queue/*` | match-service |
| 10 | `/api/*` (catchall) | game-server |
| 1 | `/` (catchall) | frontend |

---

## gRPC contracts

### `rating.v1.RatingService`
**Owner:** rating-service
**Callers:** match-service (queue, before enqueue to fetch MMR)

```protobuf
rpc GetRating(GetRatingRequest)   returns (GetRatingResponse)
rpc GetRatings(GetRatingsRequest) returns (GetRatingsResponse)
```

**When:** A player joins the ranked queue in match-service. The queue needs
current MMR to assign a score in `queue:ranked` ZSET. Synchronous because
the enqueue response must include the player's position.

**Proto file:** `shared/proto/rating/v1/rating.proto`

---

### `user.v1.UserService`
**Owner:** user-service
**Callers:** game-server, ws-gateway

```protobuf
rpc GetProfile(GetProfileRequest)   returns (GetProfileResponse)
rpc CheckBan(CheckBanRequest)       returns (CheckBanResponse)
```

**GetProfile — when:** game-server needs display data (username, avatar)
for room views without going through the HTTP layer.

**CheckBan — when:** ws-gateway checks ban status before upgrading a
WebSocket connection. Synchronous because an upgrade cannot proceed for
a banned player.

**Proto file:** `shared/proto/user/v1/user.proto`

---

### `lobby.v1.LobbyService`
**Owner:** game-server
**Callers:** match-service

```protobuf
rpc CreateRankedRoom(CreateRankedRoomRequest) returns (CreateRankedRoomResponse)
```

**When:** match-service finds two compatible players and both accept.
Synchronous because the room_id and join code must be in the `match_ready`
WS event payload.

**Proto file:** `shared/proto/lobby/v1/lobby.proto`

---

### `game.v1.GameService`
**Owner:** game-server
**Callers:** match-service, ws-gateway

```protobuf
rpc StartSession(StartSessionRequest)         returns (StartSessionResponse)
rpc GetMoveLog(GetMoveLogRequest)             returns (GetMoveLogResponse)
rpc IsParticipant(IsParticipantRequest)       returns (IsParticipantResponse)
```

**StartSession — when:** Both players accept a match. match-service calls
this after `CreateRankedRoom` to get the session_id for the `match_ready`
WS event. Synchronous.

**GetMoveLog — when:** replay-service fetches the ordered move list to
serve a replay. Synchronous.

**IsParticipant — when:** ws-gateway upgrades a room WebSocket connection.
Determines participant vs spectator status without querying Postgres
directly. Synchronous because the upgrade decision depends on it.

**Proto file:** `shared/proto/game/v1/game.proto`

---

## Async event contracts

All event payloads carry `event_id` (UUID) for idempotency, `occurred_at`, and `version`.
Transport is chosen per event based on the consumer topology — see the legend
above and the per-event tables below.

### Redis Streams (fan-out events)

Stream name = event name. Each consuming service uses a consumer group whose
name equals the service name (e.g., `rating-service`). Each event is delivered
once across the group's replicas; each group sees every event independently.
Producer + Worker helpers live in `shared/streams/`.

#### `game.session.finished`
**Publisher:** game-server
**Stream:** `game.session.finished`
**Consumer groups:**

| Group | Action |
|---|---|
| `rating-service` | Update `ratings` + `rating_history` (ranked only) |
| `user-service` | Evaluate achievement progress, persist + publish `achievement.unlocked` |

**Payload:** session_id, room_id, game_id, mode, ended_by, winner_id,
is_draw, duration_secs, move_count, players[]

#### `player.banned`
**Publisher:** user-service
**Stream:** `player.banned`
**Consumer groups:**

| Group | Action |
|---|---|
| `auth-service` | Revoke active session, then publish `player.session.revoked` (Pub/Sub) |
| `match-service` | Remove the player from the matchmaking queue if present |
| `notification-service` | Create `ban_issued` in-app notification |

**Payload:** player_id, ban_id, banned_by, reason, expires_at

---

### Asynq (point-to-point tasks)

Task type identifies the work; queue scopes the worker pool. Asynq provides
retry with exponential backoff, scheduling, retention, and a DLQ — Asynqmon
exposes operational visibility.

#### `bot:activate:<botID>`
**Publisher:** match-service (backfill detector, when a lone human has
waited past `BACKFILL_THRESHOLD_SECS`).
**Queue:** `bot-activate`. **Task type:** `bot:activate:<bot-uuid>` (per-bot).
**Consumer:** bot-runner in `--mode=backfill` — each bot's runner registers
a handler under its own task type, so only the selected bot fires.
**Retention:** 2 minutes. **Retry:** 0 (one-shot).
**Payload:** `{"player_id": "<bot-uuid>"}`

#### `notification:friendship-requested`
**Publisher:** user-service (on friend request created).
**Queue:** `notification`. **Consumer:** notification-service.
**Action:** create `friend_request` notification + WS push to addressee.
**Payload:** requester_id, requester_username, addressee_id

#### `notification:friendship-accepted`
**Publisher:** user-service (on friend request accepted).
**Queue:** `notification`. **Consumer:** notification-service.
**Action:** create `friend_request_accepted` notification + WS push to requester.
**Payload:** requester_id, addressee_id, addressee_username

> `addressee_username` is in the payload so notification-service doesn't
> need a gRPC round-trip to user-service to compose the message.

#### `notification:achievement-unlocked`
**Publisher:** user-service (after evaluating achievement progress).
**Queue:** `notification`. **Consumer:** notification-service.
**Action:** create `achievement_unlocked` notification + WS push to player.
**Payload:** player_id, achievement_key, tier, tier_name

#### Other Asynq tasks
- `match:expiry` — match-service internal: expires unconfirmed matches
- `runtime:turn-timeout`, `runtime:ready-timeout` — game-server internal: turn timers

---

### Redis Pub/Sub (fan-out across consumer replicas, or per-connection)

Pub/Sub is at-most-once and broadcasts to every subscriber. Used in two cases:

1. **ws-gateway service consumer** — each replica owns local WebSocket connections,
   so every replica must see each event (only the right one acts).
2. **Per-connection delivery** (`room.{roomID}`, `player.{playerID}`) — each pod
   subscribes per active connection; messages reach the pod hosting that
   connection.

Pub/Sub consumers in this repo MUST be naturally idempotent or fan-out-safe.
A global SETNX dedupe across replicas would break the fan-out semantic.

#### `player.session.revoked`
**Publisher:** auth-service (on session revoke — logout, superseded login,
or downstream of `player.banned`).
**Consumer:** ws-gateway service consumer → close the WebSocket connection
for that session. Every ws-gateway replica subscribes; only the pod hosting
the player's connection has anything to disconnect.
**Payload:** player_id, session_id, reason ("logout" | "superseded" | "banned")

#### `admin.broadcast.sent`
**Publisher:** user-service (admin sends a broadcast).
**Consumer:** ws-gateway service consumer → broadcast to all clients connected
to that replica. Every ws-gateway replica must broadcast independently.
**Payload:** message, broadcast_type ("info" | "warning"), sent_by

#### `player.unbanned`
**Publisher:** user-service. **Consumer:** none currently. Reserved for future.

#### Per-connection channels (`room.{roomID}`, `player.{playerID}`)
**Publishers:** game-server hub, ws-gateway hub, match-service queue
(match_found / match_ready / match_cancelled), chat-service publisher
(room messages, DMs), notification-service publisher (notification pushes).
**Consumer:** ws-gateway hub — per-active-connection subscriptions only.

---

## What stays internal (no contract needed)

| Call | Service | Why internal |
|---|---|---|
| Apply a move to game state | game-server | pure in-process logic |
| Flush session_events to Postgres | game-server | internal cleanup on session end |
| Validate JWT on every request | all services | shared middleware, not cross-service |
| Calculate ELO delta | rating-service | triggered by `game.session.finished` |

---

## Monorepo structure

```
recess/
  shared/
    proto/
      rating/v1/   rating.proto + rating.pb.go + rating_grpc.pb.go
      user/v1/     user.proto   + user.pb.go   + user_grpc.pb.go
      lobby/v1/    lobby.proto  + lobby.pb.go  + lobby_grpc.pb.go
      game/v1/     game.proto   + game.pb.go   + game_grpc.pb.go
    events/
      events.go          ← async event structs (used as Streams payloads,
                            Asynq task payloads, and Pub/Sub messages)
    streams/
      *.go               ← Redis Streams Producer + Worker (fan-out events)
    ws/
      ws.go              ← client-facing WS event types
    middleware/
      auth.go            ← JWT validator (shared by all services)
    errors/
      apierrors.go       ← standard API error string constants
    domain/
      rating/            ← ELO engine
      matchmaking/       ← matchmaker algorithm
  services/
    game-server/         :8080  gRPC :9080
    auth-service/        :8081
    user-service/        :8082  gRPC :9082
    chat-service/        :8083
    ws-gateway/          :8084
    rating-service/      :8085  gRPC :9085
    notification-service/:8086
    match-service/       :8087
```

Stubs are committed alongside `.proto` files. Regenerate with:

```bash
protoc \
  --go_out=shared/proto/{service}/v1 --go_opt=paths=source_relative \
  --go-grpc_out=shared/proto/{service}/v1 --go-grpc_opt=paths=source_relative \
  -I shared/proto/{service}/v1 \
  shared/proto/{service}/v1/{service}.proto
```

---

## Event versioning rules

1. **Adding a field** — safe. Consumers that don't know the field ignore it.
2. **Renaming a field** — breaking. Add a new stream/queue version
   (`game.session.finished.v2` for Streams; new task type for Asynq), run
   both in parallel until all consumers migrate, then retire v1.
3. **Removing a field** — breaking. Same process as rename.
4. **Never reuse a field name with a different type.**
5. **Changing the transport** (Pub/Sub → Streams → Asynq) — coordinated
   migration, lockstep across publisher and all consumers in a single PR
   per channel. See P3.6 in `ROADMAP.md` for the playbook.

For Protobuf: field numbers are the contract, not names. Never reuse a
field number. Adding fields with new numbers is always safe.
