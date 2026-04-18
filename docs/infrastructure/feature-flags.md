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

Unleash is primarily configured through its UI or REST API. For reproducibility across environments, use the CLI for export/import:

```bash
# Export flags
unleash export --url http://unleash.localhost --token <token> > flags.json

# Import to another environment
unleash import --url http://prod-unleash --token <token> < flags.json
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
