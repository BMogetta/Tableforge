# Recess

> A self-hosted multiplayer engine for the board games collecting dust on your shelf.

Some games are too good to collect dust. Recess is a self-hosted multiplayer engine
that lets you play the board games you already own — with the friends you already have.
No subscriptions, no strangers, no ads. Just your shelf, a server, and the group chat
that's been saying "we should play something" for three weeks.

## Stack

| Layer        | Technology                                |
|--------------|-------------------------------------------|
| Backend      | Go (microservices, gRPC + REST)           |
| Frontend     | TypeScript, React 19, Vite                |
| Real-time    | WebSocket (per-room + per-player)         |
| Database     | PostgreSQL                                |
| Cache / PubSub | Redis                                   |
| Observability| OpenTelemetry → Tempo, Loki, Prometheus, Grafana |
| Infra        | Docker Compose, Traefik                   |

## Prerequisites

- [Make](https://www.gnu.org/software/make/)
- [Docker](https://docs.docker.com/get-docker/) with `docker compose`
- [Go](https://go.dev/dl/) 1.25+
- [Node.js](https://nodejs.org/) 22+
- [jq](https://jqlang.github.io/jq/download/)

For code generation (`make gen-types`, `make gen-proto`):
- [protoc](https://grpc.io/docs/protoc-installation/) + `protoc-gen-go` + `protoc-gen-go-grpc`

## Setup

```bash
git clone https://github.com/BMogetta/recess.git && cd recess
make setup                 # verify tools + install frontend deps
cp .env.example .env       # fill in GitHub OAuth + JWT secret
```

## Quick Start

```bash
make up                    # postgres + redis + game-server
make up-app                # + traefik + frontend
make up-all                # + full observability stack
```

Everything goes through Traefik on port 80. Run `make test-routing` to verify.

See `CLAUDE.md` for the full architecture, service map, and all available commands.

<!-- routing:start -->
## Routing

| Route | Service | Status |
|-------|---------|--------|
| `GET /api/v1/games` | game-server | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/bots/profiles` | game-server | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/sessions/{id}` | game-server | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/stats` | game-server | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /auth/github` | auth-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /auth/me` | auth-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/profile` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/friends` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/settings` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/players` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/allowed-emails` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/audit-logs` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `POST /api/v1/admin/broadcast` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/dm` | chat-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/ratings/{game}/leaderboard` | rating-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/notifications` | notification-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `POST /api/v1/queue` | match-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/rooms/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/players/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET grafana.localhost` | grafana | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /` | frontend | ![pass](https://img.shields.io/badge/-pass-brightgreen) |

_Last updated: 2026-04-05_

<!-- routing:end -->

## Adding a Game

Implement the `Game` interface in `services/game-server/games/` and register it
in `registry.go`. The engine handles rooms, turns, timers, bots, and spectators.

```go
type Game interface {
    ID() string
    Name() string
    MinPlayers() int
    MaxPlayers() int
    Init(players []Player) (GameState, error)
    ValidateMove(state GameState, move Move) error
    ApplyMove(state GameState, move Move) (GameState, error)
    IsOver(state GameState) (bool, Result)
}
```

Optional interfaces: `StateFilter` (hidden information), `TurnTimeoutHandler` (custom timeout moves).

<!-- e2e:start -->
## E2E Tests

| Project | Passed | Status |
|---------|--------|--------|
| chat-tests | 5/5 | ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |
| chromium-parallel | 1/1 | ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |
| game-tests | 10/15 | ![partial](https://img.shields.io/badge/-10_of_15-yellow) |
| presence-tests | 2/5 | ![partial](https://img.shields.io/badge/-2_of_5-yellow) |
| session-history-tests | 0/10 | ![skip](https://img.shields.io/badge/-skipped-lightgrey) |
| settings-tests | 7/7 | ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |
| spectator-tests | 8/12 | ![partial](https://img.shields.io/badge/-8_of_12-yellow) |
| **Total** | **33/55** | ![progress](https://img.shields.io/badge/-33_of_55-yellow) |

_Last updated: 2026-04-04_
<!-- e2e:end -->

<!-- i18n:start -->
## Translations

| Language | Keys | Coverage |
|----------|------|----------|
| English | 299 | ![100%](https://img.shields.io/badge/translated-100%25-brightgreen) |
| Español | 299 | ![100%](https://img.shields.io/badge/translated-100%25-brightgreen) |

_Last updated: 2026-04-04_
<!-- i18n:end -->

<!-- coverage:start -->
## Test Coverage

| Service | Tests | Coverage |
|---------|-------|----------|
| auth-service | 41 passed | ![57.4%](https://img.shields.io/badge/coverage-57.4%25-yellow) |
| chat-service | 33 passed | ![50.8%](https://img.shields.io/badge/coverage-50.8%25-yellow) |
| game-server | 318 passed | ![54.0%](https://img.shields.io/badge/coverage-54.0%25-yellow) |
| match-service | 73 passed | ![64.7%](https://img.shields.io/badge/coverage-64.7%25-green) |
| notification-service | 43 passed | ![57.3%](https://img.shields.io/badge/coverage-57.3%25-yellow) |
| rating-service | 37 passed | ![66.8%](https://img.shields.io/badge/coverage-66.8%25-green) |
| user-service | 95 passed | ![58.7%](https://img.shields.io/badge/coverage-58.7%25-yellow) |
| ws-gateway | 61 passed | ![31.9%](https://img.shields.io/badge/coverage-31.9%25-red) |
| frontend (vitest) | 472 passed | ![70.8%](https://img.shields.io/badge/coverage-70.8%25-green) |

_Last updated: 2026-04-05_

<!-- coverage:end -->
