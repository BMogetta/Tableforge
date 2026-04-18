# Feature Flags (Unleash)

Unleash provides runtime feature flags for both backend and frontend. It runs as part of the `app` profile.

## Access

- **UI**: `http://${UNLEASH_HOST}` (default: `unleash.localhost`)
- **API**: same host, port 4242 internally
- **Auth**: disabled (`AUTH_TYPE=none`). Every request to the admin UI is bound to a synthetic admin user. Safe only because the service is not exposed outside Traefik + cloudflared. For anything public, flip to `AUTH_TYPE=open-source` and seed admin credentials via `INIT_ADMIN_USERNAME` / `INIT_ADMIN_PASSWORD`.

## Architecture

Unleash has its own database (`unleash`) and user (`unleash`) in the shared Postgres instance, completely isolated from the application database.

```
Frontend (React SDK) ----> Traefik ----> Unleash server ----> PostgreSQL (unleash DB)
Backend  (Go SDK)    -----> directly --->
```

## Configuration

Flags live in `infra/unleash/`:

- `flags.json` — one entry per flag: `{name, description, type}`.
- `environments.json` — per-(flag, environment) enabled state.
- `seed.sh` — idempotent script that reads both JSONs and reconciles Unleash state via the admin API.

On `make up-app`, a short-lived `unleash-seeder` service runs `seed.sh` after the Unleash container is healthy, creates any missing flag, and flips the env states to match the JSON. Re-runs are no-ops; adding a new flag = edit the JSON + restart the seeder (`docker compose up -d unleash-seeder`).

### Adding a new flag

1. Append an entry to `infra/unleash/flags.json`.
2. Append the desired state to `infra/unleash/environments.json` (one entry per env).
3. Restart the seeder: `docker compose up -d unleash-seeder`.
4. Use it from code via `useFlag('your-flag')` (frontend) or `flags.IsEnabled("your-flag", default)` (backend).

### Seeder env vars

- `UNLEASH_URL` — base URL (accepts both `http://host:4242` and `http://host:4242/api`; the script normalizes).
- `UNLEASH_TOKEN` — admin API token. Defaults to the compose-seeded `*:*.unleash-insecure-api-token`.

### Manual export/import (alternative)

For ad-hoc migrations between environments:

```bash
unleash export --url http://unleash.localhost --token <token> > state.json
unleash import --url http://prod-unleash --token <token> < state.json
```

## SDK Integration

### Go (backend)

```go
import unleash "github.com/Unleash/unleash-client-go/v4"

unleash.Initialize(
    unleash.WithAppName("game-server"),
    unleash.WithUrl("http://unleash:4242/api"),
    unleash.WithCustomHeaders(http.Header{
        "Authorization": []string{"*:development.unleash-insecure-api-token"},
    }),
)

if unleash.IsEnabled("new-feature") {
    // new code path
}
```

### React (frontend)

```tsx
import { FlagProvider, useFlag } from "@unleash/proxy-client-react"

<FlagProvider config={{
    url: "http://unleash.localhost/api/frontend",
    clientKey: "default:development.unleash-insecure-api-token",
    appName: "frontend",
}}>
    <App />
</FlagProvider>

// In components
const enabled = useFlag("new-feature")
```

## Maintenance Mode

A `maintenance-mode` flag can serve as a kill switch:

- **Frontend**: check the flag early and render a maintenance screen
- **Backend**: global middleware that returns `503 Service Unavailable`

For maintenance when the app itself is down (DB crash, deploy failure), use a Traefik-level redirect to a static Nginx page instead -- Unleash won't be reachable if the stack is down.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `UNLEASH_HOST` | `unleash.localhost` | Traefik routing domain |
| `UNLEASH_DB_PASSWORD` | `unleash` | Unleash database user password |
