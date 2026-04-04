# Grafana + Traefik Dashboard Auth

## Why
- Grafana: `GF_AUTH_ANONYMOUS_ORG_ROLE: Admin` — anyone can access as admin
- Traefik: `api.insecure=true` on `:9090` — shows full service topology

## What to change

### Grafana
- Set `GF_AUTH_ANONYMOUS_ENABLED: "false"`
- Set `GF_AUTH_DISABLE_LOGIN_FORM: "false"`
- Set admin password via `GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD}`

### Traefik
- Remove `--api.insecure=true`
- Add basicAuth middleware on the dashboard route
- Or: disable dashboard entirely in production (`--api.dashboard=false`)

## Files
- `docker-compose.monitoring.yml` (Grafana env)
- `docker-compose.yml` (Traefik command args)
- `.env.example` (add GRAFANA_ADMIN_PASSWORD)
