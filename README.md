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
git clone https://github.com/BMogetta/recess.git && cd recess
make setup                 # verify tools + install frontend deps
cp .env.example .env       # fill in GitHub OAuth + JWT secret
make up-app                # start everything
```

Open `http://localhost`. See [docs/getting-started.md](docs/getting-started.md) for prerequisites and environment variables.

## Documentation

| Topic | Link |
|-------|------|
| Setup, prerequisites, commands | [Getting Started](docs/getting-started.md) |
| Service map, communication patterns, database | [Architecture](docs/architecture.md) |
| Implement a new game (Go + React) | [Adding a Game](docs/adding-a-game.md) |
| Traefik routes, priorities, rate limits | [API Routing](docs/api/routing.md) |
| JSON Schema, gRPC, Pub/Sub contracts | [Service Contracts](docs/api/contracts.md) |
| Frontend stack, routes, store, features | [Frontend Overview](docs/frontend/overview.md) |
| Design tokens, themes, responsive, a11y | [Styling Guide](docs/frontend/styling.md) |
| Game package boundaries, card UI kit | [Game Packages](docs/frontend/game-packages.md) |
| Docker profiles, networks, volumes, init | [Docker Infrastructure](docs/infrastructure/docker.md) |
| OTel, Tempo, Loki, Prometheus, Grafana | [Observability](docs/infrastructure/observability.md) |
| Unleash setup, SDK integration | [Feature Flags](docs/infrastructure/feature-flags.md) |
| Auth, ownership checks, injection prevention | [Security](docs/security.md) |
| Unit tests, Playwright e2e, scripts | [Testing](docs/testing.md) |

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
| `DELETE /api/v1/queue/players/{id}/state` | match-service | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/rooms/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /ws/players/{id}` | ws-gateway | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET grafana.localhost` | grafana | ![pass](https://img.shields.io/badge/-pass-brightgreen) |
| `GET /` | frontend | ![pass](https://img.shields.io/badge/-pass-brightgreen) |

_Last updated: 2026-04-11_

<!-- routing:end -->

<!-- e2e:start -->
## E2E Tests

| Project | Passed | Status |
|---------|--------|--------|
| tests | 48/58 | ![partial](https://img.shields.io/badge/-48_of_58-yellow) |
| **Total** | **48/58** | ![progress](https://img.shields.io/badge/-48_of_58-yellow) |

_Last updated: 2026-04-11_
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
| auth-service | 45 passed | ![58.3%](https://img.shields.io/badge/coverage-58.3%25-yellow) |
| chat-service | 36 passed | ![51.6%](https://img.shields.io/badge/coverage-51.6%25-yellow) |
| game-server | 430 passed | ![67.0%](https://img.shields.io/badge/coverage-67.0%25-green) |
| match-service | 78 passed | ![64.7%](https://img.shields.io/badge/coverage-64.7%25-green) |
| notification-service | 45 passed | ![66.8%](https://img.shields.io/badge/coverage-66.8%25-green) |
| rating-service | 37 passed | ![66.5%](https://img.shields.io/badge/coverage-66.5%25-green) |
| user-service | 102 passed | ![58.8%](https://img.shields.io/badge/coverage-58.8%25-yellow) |
| ws-gateway | 78 passed | ![53.5%](https://img.shields.io/badge/coverage-53.5%25-yellow) |
| frontend (vitest) | 671 passed | ![74.1%](https://img.shields.io/badge/coverage-74.1%25-green) |

_Last updated: 2026-04-11_

<!-- coverage:end -->
