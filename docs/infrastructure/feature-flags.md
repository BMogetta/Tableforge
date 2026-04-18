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

Services get their SDK client from `shared/featureflags`:

```go
import (
    "github.com/recess/shared/config"
    "github.com/recess/shared/featureflags"
)

flags, err := featureflags.Init(config.LoadUnleash("game-server"))
if err != nil {
    slog.Warn("feature flags init failed, using defaults", "error", err)
}
defer flags.Close()

// Always pass a default — cold start + Unleash outages fall back to it.
if flags.IsEnabled("new-feature", false) {
    // new code path
}
```

### Go middleware

Every service (except auth-service) wraps its router with
`sharedmw.Maintenance(flags)`. This middleware returns `503 {"error":"maintenance"}`
for `POST/PUT/PATCH/DELETE` when `maintenance-mode` is ON; `GET/HEAD/OPTIONS`
always pass, and `/healthz`, `/readyz`, `/metrics` are exempt regardless of
method. Auth-service is not wrapped so users can always log in / refresh.

### React (frontend)

```tsx
import { FlagProvider, useFlag } from "@unleash/proxy-client-react"
import { flagsConfig, Flags } from "@/lib/flags"

<FlagProvider config={flagsConfig}>
    <App />
</FlagProvider>

// In components
const chatEnabled = useFlag(Flags.ChatEnabled)
```

`Flags` is the exported enum in `src/lib/flags.ts` — always import from
there instead of stringly-typed names so typos fail at compile time.

### `useFlag` vs `useCapability`

Two different patterns, pick the right one:

- **`useFlag('foo')`** — reads the frontend SDK's live value. Good for purely
  cosmetic / UX gates (maintenance banner, chat paused state, per-game
  visibility in the lobby). The flag value is fully client-controllable —
  a user can flip a localStorage entry to override it. Never use this for
  security decisions.
- **`useCapability()`** — calls `GET /auth/me/capabilities`, which the server
  computes from JWT role + live Unleash state. Authoritative because the
  server enforces it. Use this whenever the gate's output unlocks privileged
  code (e.g. loading admin devtools). The hook caches for 5 minutes and
  returns a zero-capability object on 401 so callers don't have to handle
  unauth separately.

The admin devtools panel is a real example of the second pattern: the React
Lazy chunk is fetched only when `capabilities.canSeeDevtools === true`, which
the server returns only for the owner role + flag ON combination.

### Cold-start behavior

The frontend SDK returns `false` for every flag during the first ~200ms
before the first fetch resolves. For default-ON flags (chat, achievements,
games), pair `useFlag` with `useFlagsStatus().flagsReady` and treat the
"not ready" state as enabled — otherwise users see a flash of disabled UI
on every page load.

```tsx
const { flagsReady } = useFlagsStatus()
const raw = useFlag(Flags.ChatEnabled)
const chatEnabled = !flagsReady || raw
```

## Maintenance Mode

A `maintenance-mode` flag serves as a kill switch:

- **Frontend** (`MaintenanceBanner.tsx`): full-width banner at the top of
  every route when flag is ON.
- **Backend** (`shared/middleware/maintenance.go`): global middleware wired
  into 7 of 8 services (see above).

For maintenance when the app itself is down (DB crash, deploy failure), use
a Traefik-level redirect to a static Nginx page instead — Unleash won't be
reachable if the stack is down.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `UNLEASH_HOST` | `unleash.localhost` | Traefik routing domain |
| `UNLEASH_DB_PASSWORD` | `unleash` | Unleash database user password |
