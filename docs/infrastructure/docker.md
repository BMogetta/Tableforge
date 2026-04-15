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
| `production` | pgbouncer, pg-backup, cloudflared |

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
- **cloudflared** -- Cloudflare Tunnel connector. Egress-only (outbound 443 to Cloudflare) so the host never needs an open port or public IP. Configure the tunnel in the Cloudflare dashboard to route the public hostname to `http://traefik:80` — the container resolves `traefik` over the shared `app_network`. Set `CLOUDFLARE_TUNNEL_TOKEN` in `.env`; `make up-prod` fails fast if the token is empty.

### Bringing up production

```bash
cp .env.example .env
# fill in at minimum: ENV=production, DOMAIN, ALLOWED_ORIGINS=https://<domain>,
# GITHUB_*, JWT_SECRET, OWNER_EMAIL, all DB + REDIS passwords,
# DB_HOST=pgbouncer, GRAFANA_ADMIN_PASSWORD, CLOUDFLARE_TUNNEL_TOKEN
make up-prod
```

Verify:

```bash
docker compose ps cloudflared                       # running
docker compose logs cloudflared | grep 'Registered' # 4 edge connections
curl -sI https://<domain>/api/v1/health             # 200 via Cloudflare
```

Cloudflare proxies WebSockets by default on the orange-cloud proxy — no tunnel-side flag needed. SSL/TLS mode on the zone should be **Full** (not Flexible, not Full strict): Cloudflare terminates TLS at the edge and speaks HTTP to `traefik:80` over the encrypted tunnel.

GitHub OAuth callback for prod: set Homepage URL to `https://<domain>` and Callback URL to `https://<domain>/auth/github/callback` on the OAuth app at `github.com/settings/developers`.

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
| cloudflared | 128M | 0.25 |
