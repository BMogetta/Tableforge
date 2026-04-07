# API Routing

All traffic from the frontend goes through **Traefik** (reverse proxy) on port 80. Services are never called directly -- Traefik routes requests based on path patterns and priorities.

## Route Table

Routes are evaluated by priority (highest wins). When multiple routes match, the highest priority takes precedence.

| Priority | Path Pattern | Service | Port |
|----------|-------------|---------|------|
| 200 | `/otlp/*` | otel-collector | 4318 |
| 150 | `/ws/*` | ws-gateway | 8084 |
| 100 | `/auth/*` | auth-service | 8081 |
| 50 | `/api/v1/players/search`, `/api/v1/players/{id}/{profile,friends,block,mute,mutes,achievements,report,settings}`, `/api/v1/admin/{players,allowed-emails,bans,reports}` | user-service | 8082 |
| 50 | `/api/v1/rooms/{id}/messages`, `/api/v1/players/{id}/dm`, `/api/v1/dm/*` | chat-service | 8083 |
| 50 | `/api/v1/ratings/*`, `/api/v1/players/{id}/ratings/*` | rating-service | 8085 |
| 50 | `/api/v1/players/{id}/notifications`, `/api/v1/notifications/*` | notification-service | 8086 |
| 50 | `/api/v1/queue/*` | match-service | 8087 |
| 50 | `/api/v1/presence/*` | ws-gateway | 8084 |
| 10 | `/api/*` (catchall) | game-server | 8080 |
| 1 | `/` (catchall) | frontend | 3000 |

Host-based routes:

| Host | Service |
|------|---------|
| `${GRAFANA_HOST}` (default: `grafana.localhost`) | Grafana |
| `${UNLEASH_HOST}` (default: `unleash.localhost`) | Unleash |

## Middleware

All HTTP routes go through:

- **security-headers** -- CSP, HSTS, X-Frame-Options, nosniff (defined in `infra/traefik/config/security.yml`)
- **Per-service rate limits** -- configured via Traefik labels in `docker-compose.services.yml`

| Service | Rate Limit (avg/burst) |
|---------|----------------------|
| auth-service | 20 / 10 |
| game-server | 100 / 50 |
| user-service | 60 / 30 |
| chat-service | 60 / 30 |
| rating-service | 60 / 30 |
| notification-service | 60 / 30 |
| match-service | 20 / 10 |
| ws-gateway | 30 / 15 |

## WebSocket Routes

WebSocket connections use sticky sessions for multi-instance deployments:

- `/ws/rooms/{roomId}` -- game events broadcast to a room
- `/ws/players/{playerId}` -- direct player notifications (queue status, DMs, friend requests)

## Verifying Routes

```bash
make up-app
make test-routing    # hits each route and updates README.md with results
```

## Adding a New Route

When adding or modifying Traefik routes:

1. Add labels in the service definition (`docker-compose.services.yml`)
2. Update `scripts/test-routing.sh` with the new route
3. Run `make test-routing` to verify
