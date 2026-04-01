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

## Quick Start

```bash
cp .env.example .env      # fill in GitHub OAuth + JWT secret
make up                    # postgres + redis + game-server
make up-app                # + traefik + frontend
make up-all                # + full observability stack
```

Everything goes through Traefik on port 80:

| Route          | Service          |
|----------------|------------------|
| `/`            | Frontend         |
| `/api/*`       | Game Server      |
| `/grafana/*`   | Grafana          |
| `:9090`        | Traefik dashboard|

See `CLAUDE.md` for the full architecture, service map, and all available commands.

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

<!-- coverage:start -->
## Test Coverage

| Service | Tests | Coverage |
|---------|-------|----------|
| auth-service | 6 passed | ![7.0%](https://img.shields.io/badge/coverage-7.0%25-red) |
| chat-service | 12 passed | ![23.6%](https://img.shields.io/badge/coverage-23.6%25-red) |
| game-server | 162 passed | ![42.1%](https://img.shields.io/badge/coverage-42.1%25-yellow) |
| match-service | 0 passed | ![0.0%](https://img.shields.io/badge/coverage-0.0%25-lightgrey) |
| notification-service | 10 passed | ![22.6%](https://img.shields.io/badge/coverage-22.6%25-red) |
| rating-service | 11 passed | ![40.6%](https://img.shields.io/badge/coverage-40.6%25-yellow) |
| user-service | 19 passed | ![16.7%](https://img.shields.io/badge/coverage-16.7%25-red) |
| ws-gateway | 9 passed | ![21.7%](https://img.shields.io/badge/coverage-21.7%25-red) |
| frontend (vitest) | 243 passed | ![72.1%](https://img.shields.io/badge/coverage-72.1%25-green) |

_Last updated: 2026-04-01_

<!-- coverage:end -->
