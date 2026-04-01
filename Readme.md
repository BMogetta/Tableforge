# Recess

> A self-hosted multiplayer engine for the board games collecting dust on your shelf.

Some games are too good to collect dust. Recess is a self-hosted multiplayer engine
that lets you play the board games you already own — with the friends you already have.
No subscriptions, no strangers, no ads. Just your shelf, a server, and the group chat
that's been saying "we should play something" for three weeks.

## Stack

| Layer       | Technology                          |
|-------------|-------------------------------------|
| Backend     | Go                                  |
| Frontend    | TypeScript + React + Vite           |
| Database    | PostgreSQL                          |
| Traces      | OpenTelemetry → Jaeger              |
| Metrics     | OpenTelemetry → Prometheus + Grafana|
| Infra       | Docker Compose                      |

## Project Structure

```sh
recess/
├── docker-compose.yml
├── server/                     # Go backend
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/
│   │   └── server/
│   │       └── main.go         # Entry point
│   ├── internal/
│   │   ├── engine/             # Core game engine (game loop, turn management)
│   │   ├── lobby/              # Lobby and room management
│   │   ├── ws/                 # WebSocket hub
│   │   ├── api/                # HTTP handlers
│   │   ├── store/              # Database layer
│   │   └── telemetry/          # OTel setup
│   ├── games/                  # Game plugins
│   │   └── registry.go         # Plugin registry
│   └── db/
│       └── migrations/         # SQL migrations
├── frontend/                   # TypeScript + React
│   ├── Dockerfile
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Lobby.tsx
│   │   │   ├── Room.tsx
│   │   │   └── Game.tsx
│   │   ├── components/
│   │   ├── ws/                 # WebSocket client
│   │   └── api/                # HTTP client
└── infra/
    ├── otel/
    │   └── config.yaml
    ├── prometheus/
    │   └── prometheus.yml
    └── grafana/
        └── provisioning/
            └── datasources/
```

## Getting Started

```bash
docker compose up --build
```

| Service     | URL                        |
|-------------|----------------------------|
| Frontend    | <http://localhost:3000>    |
| API         | <http://localhost:8080>    |
| Grafana     | <http://localhost:3001>    |
| Jaeger UI   | <http://localhost:16686>   |
| Prometheus  | <http://localhost:9090>    |
| PostgreSQL  | localhost:5432             |

## Adding a Game

Implement the `Game` interface in `server/games/` and register it in `registry.go`. The engine handles the rest.

```go
type Game interface {
    ID() string
    Init(players []Player) (GameState, error)
    ValidateMove(state GameState, move Move, playerID string) error
    ApplyMove(state GameState, move Move) (GameState, error)
    IsOver(state GameState) (bool, Result)
}
```
