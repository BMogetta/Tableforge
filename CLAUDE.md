# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Docker (primary development workflow)

```bash
make up          # Start data layer + game-server (postgres, redis, game-server)
make up-app      # Add traefik + frontend
make up-all      # Add full observability stack (OTel, Tempo, Loki, Prometheus, Grafana)
make up-test     # Start with TEST_MODE=true for e2e tests
make down        # Stop all services
make reset       # Hard reset — wipes volumes, images, and networks
```

### Backend (Go)

The monorepo uses a Go workspace (`go.work`), so most `go` commands work from the repo root.

```bash
# Run all Go tests across all modules
go test ./...

# Run tests for a specific package
go test ./services/game-server/internal/domain/runtime/...

# Run a single test
go test -run TestName ./path/to/package/...

# Run the game server locally (outside Docker)
cd services/game-server && go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm run dev       # Vite dev server
npm run build     # TypeScript check + Vite production build
npm test          # Vitest unit tests
npm run test:ui   # Vitest with UI
```

### E2E Tests (Playwright)

```bash
make up-test           # Start in test mode
make seed-test         # Create test players → frontend/tests/e2e/.players.json
make test              # Run all Playwright tests
make test-one NAME="partial test name"  # Run single test
make test-ui           # Interactive Playwright UI
make clean-test        # Remove auth state and player fixtures
```

### Code Generation

```bash
# Regenerate frontend/src/lib/api-generated.ts from Go API
# Requires: swag (go install github.com/swaggo/swag/cmd/swag@latest)
make gen-types
```

## Architecture

Tableforge is a **microservices monorepo**. All services live under `services/` and share code via `shared/` through a Go workspace.

### Services

| Service | Purpose | HTTP | gRPC |
|---------|---------|------|------|
| `services/game-server` | Game engine, lobby, sessions, bots | 8080 | 9080 |
| `services/auth-service` | GitHub OAuth, JWT issuance/validation | 8081 | — |
| `services/user-service` | Profiles, friends, bans, mutes, reports, admin, settings | 8082 | 9082 |
| `services/chat-service` | Room messages, DMs | 8083 | — |
| `services/ws-gateway` | WebSocket hub, real-time fan-out | 8084 | — |
| `services/rating-service` | ELO/MMR calculations, leaderboards | 8085 | 9085 |
| `services/notification-service` | In-app notifications, Redis consumer | 8086 | — |
| `services/match-service` | Ranked matchmaking queue | 8087 | — |

game-server exposes gRPC on `:9080` implementing `lobby.v1.LobbyService` (CreateRankedRoom) and `game.v1.GameService` (StartSession, IsParticipant). match-service is the sole caller.

All traffic from the frontend goes through **Traefik** (reverse proxy). There is no direct service-to-service HTTP — inter-service calls use gRPC (synchronous) or Redis Pub/Sub (async).

### Communication Patterns

- **gRPC + Protobuf** — synchronous calls where the caller needs a response (e.g., game-server → rating-service to fetch MMR before matchmaking). Proto definitions live in `shared/proto/`.
- **Redis Pub/Sub** — asynchronous events. Channel naming: `{domain}.{entity}.{verb}` (e.g., `game.session.finished`). All events carry an `event_id` UUID for idempotency. Event structures defined in `shared/events/`.
- **REST/JSON** — frontend ↔ services only, never service-to-service.

See `shared/contracts.md` for the complete service contract map and versioning rules.

### Shared Module (`shared/`)

- `shared/proto/` — Protobuf definitions + generated Go stubs (rating, user, lobby, game)
- `shared/events/` — Redis Pub/Sub event structs
- `shared/domain/rating/` — ELO/MMR engine (shared between game-server and rating-service)
- `shared/domain/matchmaking/` — matchmaker algorithm (shared between match-service and simulate tool)
- `shared/middleware/` — JWT auth + OpenTelemetry HTTP middleware
- `shared/redis/` — Redis client wrapper
- `shared/telemetry/` — OTel setup (traces, metrics, logs)
- `shared/config/` — Environment variable loader
- `shared/db/migrations/` — SQL migrations (applied in order: 001, 002, 003, then 999_seed in test mode)
- `shared/errors/` — Shared API error types

### Frontend (`frontend/`)

React 19 + TypeScript app served via Nginx in Docker.

- **Routing:** TanStack Router (file-based, `src/routes/`)
- **Server state:** TanStack Query
- **Client state:** Zustand (`src/stores/`)
- **Linting:** Biome
- **Testing:** Vitest (unit), Playwright (e2e in `tests/e2e/`)

Feature modules in `src/features/`: `auth`, `game`, `lobby`, `room`, `admin`, `devtools`.
Game renderers (pluggable) in `src/games/`: `tictactoe`, `loveletter`.

### Database

PostgreSQL with schema separation:
- Default schema — rooms, sessions, moves, events, notifications, chat (owned by game-server)
- `users` schema — profiles, friendships, bans, mutes, reports (owned by user-service)
- `ratings` schema — ratings and rating history (owned by rating-service)

### Redis Key Inventory

See `shared/redis-keymap.md` for the full key map with TTLs and owners.

### Observability

When running `make up-all`:
- **Tempo** — distributed traces (gRPC from all Go services + HTTP from frontend)
- **Loki + Promtail** — container logs
- **Prometheus** — metrics scraped from `/metrics` endpoints
- **Grafana** — dashboards at `http://localhost:3001`

### Turn Timers & Match Expiry

Turn timers (game-server) and match confirmation expiry (match-service) use **Asynq** — a distributed task queue backed by Redis. Tasks are persisted and guaranteed exactly-once delivery, making the system safe for multi-instance horizontal scaling.
