# Tableforge

A local multiplayer turn-based board game engine. Add any board game as a plugin and play with a friend over your local network.

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
tableforge/
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
