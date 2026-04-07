# Docker Infrastructure

Everything runs in Docker Compose. Services are organized into profiles so you can start only what you need.

## Profiles

| Command | What starts |
|---------|-------------|
| `make up` | postgres + redis + game-server |
| `make up-app` | + traefik + frontend + all backend services + unleash |
| `make up-all` | + observability stack (OTel, Tempo, Loki, Prometheus, Grafana) |
| `make up-prod` | + PgBouncer + pg-backup |

| Profile | Services |
|---------|----------|
| (none) | postgres, redis, game-server |
| `app` | traefik, frontend, auth-service, user-service, chat-service, ws-gateway, rating-service, match-service, notification-service, unleash |
| `monitoring` | otel-collector, tempo, loki, prometheus, grafana |
| `production` | pgbouncer, pg-backup |

## Compose Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Core data layer (postgres, redis) + game-server + traefik + frontend |
| `docker-compose.services.yml` | All backend microservices + unleash + production services |
| `docker-compose.monitoring.yml` | Full observability stack + network definitions |
| `docker-compose.override.yml` | Development overrides |

## Networks

Three isolated networks control which services can communicate:

```
data_network         postgres, redis, game-server, all services needing DB/cache
                     Isolated from external traffic.

app_network          traefik, all services, frontend, otel-collector, grafana
                     Traefik routes to any service on this network.

monitoring_network   otel-collector, tempo, loki, prometheus, grafana, traefik
                     Internal observability wiring.
```

Services that need both DB access and Traefik routing (most backend services) are on both `data_network` and `app_network`.

## Volumes

| Volume | Purpose |
|--------|---------|
| `postgres_data` | PostgreSQL data directory |
| `redis_data` | Redis append-only file |
| `traefik_certs` | TLS certificates |
| `pg_backups` | Daily Postgres backups (production only) |
| `tempo-data` | Trace storage |
| `loki-data` | Log storage |
| `prometheus-data` | Metrics storage |
| `grafana-data` | Grafana dashboards and config |

## Database Initialization

On first startup, PostgreSQL runs everything in `shared/db/migrations/` (mounted as `/docker-entrypoint-initdb.d`) in alphabetical order:

1. `000_init-databases.sh` -- creates `recess` and `unleash` users + `unleash` database
2. `001_initial.sql` through `006_*.sql` -- schema migrations
3. `999_seed.sql` -- test seed data (only meaningful in test mode)

This only runs once. To re-initialize: `make reset` (wipes all volumes).

## Production Profile

When running with `--profile production`:

- **PgBouncer** -- connection pooler sitting between services and Postgres. Set `DB_HOST=pgbouncer` in `.env` to route through it (200 max connections, pool size 20).
- **pg-backup** -- daily automated Postgres backups. Retention: 14 days, 4 weeks, 6 months.

## Resource Limits

Every service has CPU and memory limits defined in `deploy.resources.limits`. Default allocations:

| Service | Memory | CPU |
|---------|--------|-----|
| postgres | 512M | 1.0 |
| redis | 256M | 0.5 |
| game-server | 256M | 0.5 |
| frontend | 512M | 0.5 |
| backend services | 256M | 0.5 |
| unleash | 512M | 0.5 |
| pgbouncer | 128M | 0.25 |
