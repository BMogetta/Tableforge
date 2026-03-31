# Tableforge — Service Contracts

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
                   Redis Pub/Sub (async events)
         ─────────────────────────────────────────────
               │                │                │
       notification-service  ws-gateway    rating-service
```

**Three communication layers:**

1. **REST/JSON over HTTP** — frontend ↔ services via Traefik. Never between services.
2. **gRPC + Protobuf** — synchronous calls where the caller needs a response to continue.
3. **Redis Pub/Sub + JSON** — async events where the publisher doesn't need to wait.

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

All events flow through Redis Pub/Sub. Channel naming: `{domain}.{entity}.{verb}`.
Every event carries `event_id` (UUID) for idempotency, `occurred_at`, and `version`.

### `game.session.finished`
**Publisher:** game-server
**Consumers:**

| Consumer | Action |
|---|---|
| rating-service | Update `ratings` + `rating_history` (ranked only) |
| match-service | Unblock room, allow rematch queue re-entry |

**Payload:** session_id, room_id, game_id, mode, ended_by, winner_id,
is_draw, duration_secs, players[]

---

### `match.found`
**Publisher:** match-service
**Consumers:** ws-gateway → delivers `match_found` WS event to both players

**Payload:** match_id, room_id, room_code, game_id, player_a_id, player_b_id, mmr_a, mmr_b

---

### `match.cancelled`
**Publisher:** match-service
**Consumers:** ws-gateway → delivers `match_cancelled` WS event

**Payload:** match_id, player_a_id, player_b_id, reason

---

### `match.ready`
**Publisher:** match-service (after both players accept + StartSession succeeds)
**Consumers:** ws-gateway → delivers `match_ready` WS event with session_id

**Payload:** match_id, session_id, room_id, room_code, player_a_id, player_b_id

---

### `player.banned`
**Publisher:** user-service
**Consumers:**

| Consumer | Action |
|---|---|
| auth-service | Revoke active session, publish `player.session.revoked` |
| match-service | Remove from queue if present |
| game-server | Forfeit any active ranked sessions |
| notification-service | Create `ban_issued` in-app notification |

**Payload:** player_id, ban_id, banned_by, reason, expires_at

---

### `player.unbanned`
**Publisher:** user-service
**Consumers:** none currently

---

### `friendship.accepted`
**Publisher:** user-service
**Consumers:** notification-service → `friend_request_accepted` in-app notification to requester

**Payload:** requester_id, addressee_id, addressee_username

> `addressee_username` is included in the event so notification-service
> does not need a gRPC round-trip to user-service to compose the message.

---

### `player.session.revoked`
**Publisher:** auth-service
**Consumers:** ws-gateway → closes the WebSocket connection for that session

**Payload:** player_id, session_id, reason ("logout" | "superseded" | "banned")

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
tableforge/
  shared/
    proto/
      rating/v1/   rating.proto + rating.pb.go + rating_grpc.pb.go
      user/v1/     user.proto   + user.pb.go   + user_grpc.pb.go
      lobby/v1/    lobby.proto  + lobby.pb.go  + lobby_grpc.pb.go
      game/v1/     game.proto   + game.pb.go   + game_grpc.pb.go
    events/
      events.go          ← async event structs (Redis Pub/Sub)
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
2. **Renaming a field** — breaking. Add a new channel version (`game.session.finished.v2`),
   run both in parallel until all consumers migrate, then deprecate v1.
3. **Removing a field** — breaking. Same process as rename.
4. **Never reuse a field name with a different type.**

For Protobuf: field numbers are the contract, not names. Never reuse a
field number. Adding fields with new numbers is always safe.
