# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Docker (primary development workflow)

```bash
make up          # Start data layer + game-server (postgres, redis, game-server)
make up-app      # Add traefik + frontend
make up-all      # Add full observability stack (OTel, Tempo, Loki, Prometheus, Grafana)
make up-test     # Start with TEST_MODE=true for e2e tests
make down        # Stop all services
make reset       # Hard reset — wipes volumes, images, and networks
```

### Backend (Go)

The monorepo uses a Go workspace (`go.work`), so most `go` commands work from the repo root.

```bash
# Run all Go tests across all modules
go test ./...

# Run tests for a specific package
go test ./services/game-server/internal/domain/runtime/...

# Run a single test
go test -run TestName ./path/to/package/...

# Run the game server locally (outside Docker)
cd services/game-server && go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm run dev       # Vite dev server
npm run build     # TypeScript check + Vite production build
npm test          # Vitest unit tests
npm run test:ui   # Vitest with UI
```

### E2E Tests (Playwright)

```bash
make up-test           # Start in test mode
make seed-test         # Create test players → frontend/tests/e2e/.players.json
make test              # Run all Playwright tests
make test-one NAME="partial test name"  # Run single test
make test-ui           # Interactive Playwright UI
make clean-test        # Remove auth state and player fixtures
```

### Setup

```bash
make setup    # Check required tools + install frontend deps
```

### Code Generation

```bash
# Regenerate frontend/src/lib/schema-generated.zod.ts from JSON Schema definitions
# Generates Zod schemas + inferred TypeScript types
make gen-types

# Regenerate protobuf Go stubs from .proto definitions
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
make gen-proto
```

### Adding dependencies

When adding a new external tool or dependency that is required to build or run
the project, also add it to `make setup` in the Makefile so it is checked
during environment verification.

## Architecture

Recess is a **microservices monorepo**. All services live under `services/` and share code via `shared/` through a Go workspace.

### Services

| Service | Purpose | HTTP | gRPC |
|---------|---------|------|------|
| `services/game-server` | Game engine, lobby, sessions, bots | 8080 | 9080 |
| `services/auth-service` | GitHub OAuth, JWT issuance/validation | 8081 | — |
| `services/user-service` | Profiles, friends, bans, mutes, reports, admin, settings | 8082 | 9082 |
| `services/chat-service` | Room messages, DMs | 8083 | — |
| `services/ws-gateway` | WebSocket hub, real-time fan-out | 8084 | — |
| `services/rating-service` | ELO/MMR calculations, leaderboards | 8085 | 9085 |
| `services/notification-service` | In-app notifications, Redis consumer | 8086 | — |
| `services/match-service` | Ranked matchmaking queue | 8087 | — |

game-server exposes gRPC on `:9080` implementing `lobby.v1.LobbyService` (CreateRankedRoom) and `game.v1.GameService` (StartSession, IsParticipant). match-service is the sole caller.

All traffic from the frontend goes through **Traefik** (reverse proxy). There is no direct service-to-service HTTP — inter-service calls use gRPC (synchronous) or Redis Pub/Sub (async).

### Communication Patterns

- **gRPC + Protobuf** — synchronous calls where the caller needs a response (e.g., game-server → rating-service to fetch MMR before matchmaking). Proto definitions live in `shared/proto/`.
- **Redis Pub/Sub** — asynchronous events. Channel naming: `{domain}.{entity}.{verb}` (e.g., `game.session.finished`). All events carry an `event_id` UUID for idempotency. Event structures defined in `shared/events/`.
- **REST/JSON** — frontend ↔ services only, never service-to-service.

See `shared/contracts.md` for the complete service contract map and versioning rules.

### Shared Module (`shared/`)

- `shared/proto/` — Protobuf definitions + generated Go stubs (rating, user, lobby, game)
- `shared/events/` — Redis Pub/Sub event structs
- `shared/domain/rating/` — ELO/MMR engine (shared between game-server and rating-service)
- `shared/domain/matchmaking/` — matchmaker algorithm (shared between match-service and simulate tool)
- `shared/middleware/` — JWT auth, OpenTelemetry HTTP middleware, JSON Schema request validation
- `shared/schemas/` — JSON Schema definitions (single source of truth for API DTOs)
  - `shared/schemas/defs/` — reusable type definitions (`$ref` targets)
  - Root-level `*.request.json` / `*.response.json` — endpoint-specific schemas
- `shared/redis/` — Redis client wrapper
- `shared/telemetry/` — OTel setup (traces, metrics, logs)
- `shared/config/` — Environment variable loader
- `shared/db/migrations/` — SQL migrations (applied in order: 001, 002, 003, then 999_seed in test mode)
- `shared/errors/` — Shared API error types

### Frontend (`frontend/`)

React 19 + TypeScript app served via Nginx in Docker.

- **Routing:** TanStack Router (file-based, `src/routes/`)
- **Server state:** TanStack Query
- **Client state:** Zustand (`src/stores/`)
- **Linting:** Biome
- **Testing:** Vitest (unit), Playwright (e2e in `tests/e2e/`)

Feature modules in `src/features/`: `auth`, `game`, `lobby`, `room`, `admin`, `devtools`.
Game renderers (pluggable) in `src/games/`: `tictactoe`, `loveletter`.

#### Styling Guidelines

##### Typography & Units

- `--font-scale` is a **unitless multiplier** (default `1`). Users can change it
  at runtime via JS (`document.documentElement.style.setProperty('--font-scale', n)`).
- All `--text-*` tokens are already pre-multiplied: `calc(Nrem * var(--font-scale))`.
  Never multiply by `--font-scale` again at the usage site.
- Typography → always `rem`-based tokens (`--text-sm`, `--text-base`, etc.)
- Component spacing → `--space-N` tokens (px internally, scale with layout not font)
- Borders, radius, shadows → raw `px` values, they do not scale with font
- Never hardcode color values in component rules — always use semantic tokens
  (`--color-interactive-glow`, not `rgba(212, 168, 83, 0.2)`)

##### Tokens

- Always use semantic variables (`--color-bg-surface`, not primitives like `--slate-900`)
- Spacing only via `--space-N` tokens, never hardcoded pixel values
- Z-index only via `--z-*` scale tokens (`--z-dropdown`, `--z-modal`, `--z-toast`, etc.)
- For themes, the accent color is always `--color-interactive`

##### Components

- Buttons: `.btn` + `.btn-{variant}` classes from global.css, never inline styles for buttons
- Cards: `.card` class from global.css, overrides in `.module.css`
- Do not create new utility classes without updating `global.css`

##### Conventions

- CSS Modules for all page and component styles
- Local variables at the top of the module or in `:local` scope
- Never use `!important`

#### Responsive

Desktop-first strategy. Default styles target desktop (1280px+).
Use `max-width` media queries to adapt down:
- `1024px` — small desktop / laptop adjustments
- `768px`  — tablet: collapse sidebars, simplify navigation
- `640px`  — mobile: single column, hide secondary panels

Never use `min-width` unless building a genuinely mobile-first component.
Breakpoint values: 640 / 768 / 1024 / 1280px — no other values.
For global font scaling on small screens, adjust `--font-scale` on `:root`,
never override individual `--text-*` tokens.

#### Accessibility

Target: WCAG 2.1 AA compliance.

- Interactive elements must be semantic: `<button>` for actions, `<a>` for navigation
- Every `<input>` must have an associated `<label>` via `htmlFor` or `aria-label`
- Never use `outline: none` without replacing with `box-shadow` focus ring
  using `var(--color-focus-ring)`
- Dynamic content that updates (game state, chat messages, turn changes)
  must use `aria-live="polite"`
- Game board elements without native HTML equivalent must have `role` + `aria-label`
- Modals/panels must have `role="dialog"`, `aria-modal="true"`, and `aria-labelledby`
  linking to their heading
- Avoid `div`/`span` with `onClick` — always use `button` or anchor

#### JSON Schema & Zod

JSON Schema is the **single source of truth** for all API DTOs shared between
Go and TypeScript. The pipeline:

```
shared/schemas/*.json  →  Go: embedded + validated at runtime (middleware)
                       →  TS: Zod schemas + inferred types (generated)
```

##### Schema file conventions

- Endpoint schemas: `{verb}_{noun}.{request|response}.json` (e.g., `create_room.request.json`)
- Shared types: `defs/{type_name}.json` (e.g., `defs/game_session.json`)
- Title field → PascalCase type name (e.g., `"title": "CreateRoomRequest"`)
- Use `$ref` to reference defs: `{ "$ref": "defs/game_session.json" }`
- No `DTO` suffix — use domain names (`GameSession`, not `GameSessionDTO`)

##### Adding a new endpoint

1. Create `shared/schemas/{verb}_{noun}.request.json` and/or `.response.json`
2. If the response uses new types, create them in `shared/schemas/defs/`
3. Run `make gen-types` to regenerate `frontend/src/lib/schema-generated.zod.ts`
4. In the game-server router, add `validate("{verb}_{noun}.request")` middleware
5. In the frontend API layer, use `validatedRequest(schema, path, init)` for the response

##### Validation rules

- **Backend request validation**: all POST/PUT/DELETE endpoints with a body
  MUST have a JSON Schema and the `validate()` middleware wired in the router.
  The middleware returns 400 with structured error details on failure.
- **Frontend response validation**: use `validatedRequest()` with the Zod schema
  for all endpoints that have a response schema. This validates the server
  response at runtime. `SchemaError` is thrown on mismatch — loud in dev/test
  (console.error + full details), generic message in production.
- **Frontend request validation**: NOT needed. TypeScript types enforce request
  shape at compile time, and the backend validates at runtime. Only validate
  user input with Zod when building request bodies from form data or dynamic
  sources where TypeScript alone can't guarantee correctness (e.g., `minLength`,
  numeric ranges).
- Endpoints without a response schema (void, other services) use `request<T>()`
  without Zod validation.

#### Test IDs

All interactive and assertable elements must have a `data-testid` attribute for
Playwright e2e tests. Test IDs are **conditionally rendered** — they appear in
dev and test mode but are stripped from production builds.

- Use the `testId()` helper from `@/utils/testId`:
  ```tsx
  import { testId } from '@/utils/testId'
  <button {...testId('submit-btn')}>Submit</button>
  ```
- Dynamic IDs use template literals: `{...testId(\`player-row-${id}\`)}`
- Never hardcode `data-testid="..."` directly — always use the helper
- The helper checks `import.meta.env.DEV || import.meta.env.VITE_TEST_MODE`
- Vitest runs with `DEV=true`, Playwright runs with `VITE_TEST_MODE=true`
- When adding new interactive elements, include a `testId` — e2e tests depend on
  stable selectors to avoid fragile CSS/text-based queries

### Database

PostgreSQL with schema separation:
- Default schema — rooms, sessions, moves, events, notifications, chat (owned by game-server)
- `users` schema — profiles, friendships, bans, mutes, reports (owned by user-service)
- `ratings` schema — ratings and rating history (owned by rating-service)

### Redis Key Inventory

See `shared/redis-keymap.md` for the full key map with TTLs and owners.

### Observability

When running `make up-all`:
- **Tempo** — distributed traces (gRPC from all Go services + HTTP from frontend)
- **Loki + Promtail** — container logs
- **Prometheus** — metrics scraped from `/metrics` endpoints
- **Grafana** — dashboards at `http://grafana.localhost`

### Turn Timers & Match Expiry

Turn timers (game-server) and match confirmation expiry (match-service) use **Asynq** — a distributed task queue backed by Redis. Tasks are persisted and guaranteed exactly-once delivery, making the system safe for multi-instance horizontal scaling.
