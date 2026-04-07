# Architecture

Recess is a **microservices monorepo**. All services live under `services/`, share code via `shared/`, and communicate through gRPC (sync) or Redis Pub/Sub (async). The frontend is a React SPA that talks to services exclusively through Traefik (reverse proxy).

## Service Map

| Service | Purpose | HTTP | gRPC |
|---------|---------|------|------|
| `game-server` | Game engine, lobby, sessions, bots, turn timers | 8080 | 9080 |
| `auth-service` | GitHub OAuth, JWT issuance/validation, token refresh | 8081 | -- |
| `user-service` | Profiles, friends, bans, mutes, reports, settings | 8082 | 9082 |
| `chat-service` | Room messages, direct messages | 8083 | -- |
| `ws-gateway` | WebSocket hub, real-time fan-out (room + player channels) | 8084 | -- |
| `rating-service` | ELO/MMR calculations, leaderboards | 8085 | 9085 |
| `notification-service` | In-app notifications, Redis event consumer | 8086 | -- |
| `match-service` | Ranked matchmaking queue, match confirmation | 8087 | -- |

## Communication Patterns

### REST/JSON (frontend <-> services)

All frontend requests go through Traefik. Services never call each other over HTTP. See [api/routing.md](api/routing.md) for the full route map.

### gRPC + Protobuf (service <-> service)

Synchronous calls where the caller needs a response. Proto definitions live in `shared/proto/`.

| Caller | Callee | Service | Purpose |
|--------|--------|---------|---------|
| match-service | rating-service | `rating.v1.RatingService` | Fetch player MMR for matchmaking |
| match-service | game-server | `lobby.v1.LobbyService` | Create ranked rooms |
| match-service | game-server | `game.v1.GameService` | Start sessions, check participation |
| game-server | user-service | `user.v1.UserService` | Fetch profiles, check bans |
| ws-gateway | user-service | `user.v1.UserService` | Fetch profiles |
| ws-gateway | game-server | `game.v1.GameService` | Check game participation |

### Redis Pub/Sub (async events)

Fire-and-forget events. Channel naming: `{domain}.{entity}.{verb}`. All events carry an `event_id` UUID for idempotency. Event structs defined in `shared/events/`.

| Event | Publishers | Consumers |
|-------|-----------|-----------|
| `game.session.finished` | game-server | rating-service, match-service |
| `match.found` | match-service | ws-gateway |
| `match.ready` | match-service | ws-gateway |
| `player.banned` | user-service | auth-service, match-service, game-server, notification-service |
| `friendship.accepted` | user-service | notification-service |
| `player.session.revoked` | auth-service | ws-gateway |

Full contract map: `shared/contracts.md`.

## Database

PostgreSQL with schema separation. All schemas live in the same `recess` database:

| Schema | Owner Service | Tables |
|--------|---------------|--------|
| `public` | game-server | players, rooms, game_sessions, game_results, game_configs, notifications, rooms_messages |
| `users` | user-service | player_profiles, friendships, player_mutes, bans, player_reports, player_achievements |
| `ratings` | rating-service | ratings, rating_history |
| `admin` | user-service | audit_logs |

Migrations are in `shared/db/migrations/` and run automatically on first Postgres startup via `docker-entrypoint-initdb.d`.

A separate `unleash` database exists for the feature flag service (Unleash), with its own user and credentials.

### Redis

Used for caching, Pub/Sub, rate limiting, presence tracking, matchmaking queues, and turn timers (via Asynq).

Full key inventory with TTLs and owners: `shared/redis-keymap.md`.

## Shared Module

The `shared/` directory contains code used by multiple services:

| Path | Purpose |
|------|---------|
| `shared/proto/` | Protobuf definitions + generated Go stubs |
| `shared/events/` | Redis Pub/Sub event structs |
| `shared/domain/rating/` | ELO/MMR engine |
| `shared/domain/matchmaking/` | Matchmaker algorithm |
| `shared/middleware/` | JWT auth, OpenTelemetry, JSON Schema validation, body limits, panic recovery |
| `shared/schemas/` | JSON Schema definitions (source of truth for API DTOs) |
| `shared/redis/` | Redis client wrapper |
| `shared/telemetry/` | OpenTelemetry setup (traces, metrics, logs) |
| `shared/config/` | Environment variable loader |
| `shared/db/migrations/` | SQL migrations |
| `shared/errors/` | Shared API error types |

The monorepo uses a Go workspace (`go.work`), so `go test ./...` works from the repo root.

## Turn Timers & Match Expiry

Both use **Asynq**, a distributed task queue backed by Redis. Tasks are persisted and guaranteed exactly-once delivery, safe for horizontal scaling.

- **Turn timers** (game-server): scheduled when a turn starts, cancelled on move or pause.
- **Match expiry** (match-service): scheduled when a match is found, cancelled when all players accept.
