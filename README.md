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
| auth-service | 26 passed | ![59.0%](https://img.shields.io/badge/coverage-59.0%25-yellow) |
| chat-service | 30 passed | ![56.3%](https://img.shields.io/badge/coverage-56.3%25-yellow) |
| game-server | 208 passed | ![41.5%](https://img.shields.io/badge/coverage-41.5%25-yellow) |
| match-service | 22 passed | ![32.0%](https://img.shields.io/badge/coverage-32.0%25-red) |
| notification-service | 17 passed | ![42.5%](https://img.shields.io/badge/coverage-42.5%25-yellow) |
| rating-service | 19 passed | ![53.1%](https://img.shields.io/badge/coverage-53.1%25-yellow) |
| user-service | 67 passed | ![44.8%](https://img.shields.io/badge/coverage-44.8%25-yellow) |
| ws-gateway | 11 passed | ![23.0%](https://img.shields.io/badge/coverage-23.0%25-red) |
| frontend (vitest) | 243 passed | ![70.9%](https://img.shields.io/badge/coverage-70.9%25-green) |

_Last updated: 2026-04-02_

<!-- coverage:end -->
