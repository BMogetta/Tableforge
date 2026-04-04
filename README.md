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
| `GET /auth/github` | auth-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /auth/me` | auth-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/profile` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/friends` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/settings` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/players` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/admin/allowed-emails` | user-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/dm` | chat-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/ratings/{game}/leaderboard` | rating-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /api/v1/players/{id}/notifications` | notification-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `POST /api/v1/queue` | match-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/rooms/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/players/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET grafana.localhost` | grafana | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /` | frontend | ![pass](https://img.shields.io/badge/-pass-brightgreen) |

_Last updated: 2026-04-03_

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
| game-tests | 11/15 | ![partial](https://img.shields.io/badge/-11_of_15-yellow) |
| presence-tests | 2/5 | ![partial](https://img.shields.io/badge/-2_of_5-yellow) |
| session-history-tests | 0/10 | ![skip](https://img.shields.io/badge/-skipped-lightgrey) |
| settings-tests | 7/7 | ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |
| spectator-tests | 8/12 | ![partial](https://img.shields.io/badge/-8_of_12-yellow) |
| **Total** | **34/55** | ![progress](https://img.shields.io/badge/-34_of_55-yellow) |

_Last updated: 2026-04-03_
<!-- e2e:end -->

<!-- coverage:start -->
## Test Coverage

| Service | Tests | Coverage |
|---------|-------|----------|
| auth-service | 26 passed | ![59.0%](https://img.shields.io/badge/coverage-59.0%25-yellow) |
| chat-service | 30 passed | ![56.5%](https://img.shields.io/badge/coverage-56.5%25-yellow) |
| game-server | 219 passed | ![45.4%](https://img.shields.io/badge/coverage-45.4%25-yellow) |
| match-service | 27 passed | ![42.6%](https://img.shields.io/badge/coverage-42.6%25-yellow) |
| notification-service | 27 passed | ![55.3%](https://img.shields.io/badge/coverage-55.3%25-yellow) |
| rating-service | 37 passed | ![66.8%](https://img.shields.io/badge/coverage-66.8%25-green) |
| user-service | 79 passed | ![64.9%](https://img.shields.io/badge/coverage-64.9%25-green) |
| ws-gateway | 21 passed | ![28.2%](https://img.shields.io/badge/coverage-28.2%25-red) |
| frontend (vitest) | 283 passed | ![73.4%](https://img.shields.io/badge/coverage-73.4%25-green) |

_Last updated: 2026-04-03_

<!-- coverage:end -->
