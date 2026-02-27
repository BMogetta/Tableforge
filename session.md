# Tableforge — Session Handoff Prompt

## Critical Rules (always apply)
- All code, comments, and documentation **must be in English**
- CSS modules for all component styles
- `credentials: 'include'` on all fetch calls (cookie auth)
- `.env.example` always shown as markdown, never as a file download
- `expose:` not `ports:` for internal services in prod compose
- No bullet points in prose responses

---

## Project Overview

Multiplayer board game platform. Go backend + React/TypeScript frontend.

**Architecture:** plugin-based game engine, PostgreSQL, Redis, WebSocket real-time, OpenTelemetry tracing, Prometheus metrics.

**Deployment:** Raspberry Pi 5 16GB. Docker Compose prod. Cloudflare Tunnel → Caddy → services. GitHub Actions builds arm64 images to ghcr.io. Watchtower auto-updates on new image push.

---

## Repository Structure

```
tableforge/
├── server/
│   ├── cmd/server/main.go
│   ├── cmd/seed/main.go           # seeds OWNER_EMAIL as owner into allowed_emails + promotes player
│   ├── internal/
│   │   ├── api/api.go             # HTTP router + all handlers (including admin)
│   │   ├── auth/                  # GitHub OAuth + JWT middleware (injects playerID, username, role into context)
│   │   ├── engine/                # Game interface + types
│   │   ├── lobby/                 # Room management
│   │   ├── ratelimit/             # Redis sliding window
│   │   ├── runtime/               # Session lifecycle
│   │   ├── store/                 # PostgreSQL store (pgx) — store.go interface + pg_store.go implementation
│   │   ├── telemetry/             # OpenTelemetry setup
│   │   ├── testutil/              # FakeStore for tests
│   │   └── ws/                    # WebSocket hub + Redis pub/sub
│   ├── games/
│   │   ├── registry.go            # games.Register(), games.All()
│   │   └── tictactoe/             # TicTacToe plugin
│   ├── db/migrations/             # SQL migrations (run via initdb.d)
│   │   ├── 001_initial.sql
│   │   └── 002_roles.sql          # Adds player_role enum + role column to players and allowed_emails
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── telemetry.ts           # OTel SDK init, Web Vitals, JS error handler
│   │   ├── api.ts                 # fetch wrapper with OTel trace spans + all types
│   │   ├── ws.ts                  # WebSocket client with auto-reconnect
│   │   ├── store.ts               # Zustand global state (player)
│   │   ├── App.tsx                # Router + auth bootstrap + RequireAuth + RequireRole guards
│   │   ├── pages/
│   │   │   ├── Login.tsx          # GitHub OAuth + AccessDenied screen
│   │   │   ├── Lobby.tsx          # Room list + game selector + leaderboard + Admin link (manager/owner only)
│   │   │   ├── Room.tsx           # Waiting room + WebSocket
│   │   │   ├── Game.tsx           # Game page + GameRenderer router
│   │   │   └── Admin.tsx          # Admin panel: email whitelist, player roles, observability iframes
│   │   └── components/
│   │       ├── TicTacToe.tsx
│   │       └── TicTacToe.module.css
│   └── Dockerfile
├── infra/
│   ├── caddy/Caddyfile            # reverse proxy + security headers + /otlp proxy
│   ├── otel/config.yaml           # OTLP receiver (grpc+http), traces→jaeger, metrics→prometheus, logs pipeline, CORS for browser
│   ├── prometheus/prometheus.yml  # scrapes game-server:8080/metrics and otel-collector:8889
│   └── grafana/provisioning/
│       ├── datasources/datasources.yaml   # Prometheus + Jaeger datasources
│       └── dashboards/
│           ├── dashboards.yaml
│           ├── web-vitals.json     # CLS, LCP, INP, FCP, TTFB gauges + timeseries + rating donut
│           ├── game-server.json    # req rate, error rate, latency p50/p95/p99, goroutines, heap
│           └── js-errors.json      # JS error rate, collector accepted/refused records
├── docker-compose.yml
└── docker-compose.prod.yml
```

---

## Auth & Roles

- GitHub OAuth → JWT in HttpOnly cookie (`tf_session`). JWT TTL: 7 days.
- Email whitelist (`allowed_emails` table). Redirect to `/?error=email_not_allowed` on deny.
- **Player roles:** `player`, `manager`, `owner` — stored in `players.role` (type `player_role` enum).
- Role is loaded from DB on every authenticated request (always fresh, no stale JWT role).
- `requireRole(minimum)` middleware in `api.go` enforces hierarchy: player < manager < owner.
- Owner cannot demote themselves (enforced in both backend handler and `SetPlayerRole`).
- Managers can only invite players (not managers or owners).
- Seeder (`cmd/seed/main.go`) inserts `OWNER_EMAIL` into `allowed_emails` with `owner` role, then promotes the player if they have already logged in. Run it again after first login to complete the promotion.

---

## Store Interface — Key Methods

```go
// Players
CreatePlayer, GetPlayer, GetPlayerByUsername, UpdatePlayerAvatar, SoftDeletePlayer
ListPlayers(ctx) ([]Player, error)
SetPlayerRole(ctx, playerID, role) error

// Rooms
CreateRoom, GetRoom, GetRoomByCode, UpdateRoomStatus
ListWaitingRooms(ctx) ([]Room, error)
SoftDeleteRoom

// Room players
AddPlayerToRoom, RemovePlayerFromRoom, ListRoomPlayers

// Game sessions
CreateGameSession, GetGameSession, GetActiveSessionByRoom
UpdateSessionState, FinishSession, SuspendSession, ResumeSession
ListActiveSessions, SoftDeleteSession

// Moves
RecordMove, ListSessionMoves, GetMoveAt

// OAuth
UpsertOAuthIdentity, GetOAuthIdentity, GetOAuthIdentityByEmail, IsEmailAllowed

// Admin — allowed emails
ListAllowedEmails(ctx) ([]AllowedEmail, error)
AddAllowedEmail(ctx, AddAllowedEmailParams) (AllowedEmail, error)
RemoveAllowedEmail(ctx, email) error

// Results & leaderboard
CreateGameResult, GetPlayerStats, GetLeaderboard, ListPlayerHistory

// Spectators
CreateSpectatorLink, GetSpectatorLink

// Rematch
UpsertRematchVote, ListRematchVotes, DeleteRematchVotes
```

All list methods use `x := []Type{}` (never `var x []Type`) to avoid JSON `null`.

---

## API Routes

```
GET  /health
GET  /metrics                         (Prometheus)
GET  /auth/github
GET  /auth/github/callback
POST /auth/logout
GET  /auth/me

POST /api/v1/players
GET  /api/v1/games
GET  /api/v1/rooms
POST /api/v1/rooms
GET  /api/v1/rooms/{roomID}
POST /api/v1/rooms/join
POST /api/v1/rooms/{roomID}/leave
POST /api/v1/rooms/{roomID}/start
POST /api/v1/sessions/{sessionID}/move
GET  /api/v1/sessions/{sessionID}
GET  /api/v1/sessions/{sessionID}/history
GET  /api/v1/players/{playerID}/sessions
GET  /api/v1/players/{playerID}/stats
GET  /api/v1/leaderboard

GET    /api/v1/admin/allowed-emails         (manager+)
POST   /api/v1/admin/allowed-emails         (manager+)
DELETE /api/v1/admin/allowed-emails/{email} (manager+)
GET    /api/v1/admin/players                (manager+)
PUT    /api/v1/admin/players/{playerID}/role (owner only)

GET /ws/rooms/{roomID}
```

---

## Frontend Telemetry

- `telemetry.ts` initializes OTel SDK before React mounts (imported first in `main.tsx`)
- Traces: every `request()` call in `api.ts` is wrapped in a span with `traceparent` header injected
- Metrics: Web Vitals (CLS, FCP, LCP, TTFB, INP) pushed as `web_vitals` histogram to `/otlp/v1/metrics`
- Logs: `window.onerror` + `unhandledrejection` → OTLP log records to `/otlp/v1/logs`
- Caddy proxies `/otlp/*` → `otel-collector:4318` (strips prefix)
- Collector exports: traces → Jaeger, metrics → Prometheus (port 8889), logs → debug

---

## TicTacToe State Shape

```json
{
  "current_player_id": "<uuid>",
  "data": {
    "board": ["", "X", "", "O", "", "", "", "", ""],
    "marks": { "<playerID>": "X", "<playerID2>": "O" },
    "players": ["<playerID>", "<playerID2>"]
  }
}
```
Move payload: `{ "cell": 0-8 }`

---

## Infra Notes

- Dev: single origin via Caddy on port 80. `FRONTEND_URL=http://localhost`
- otel-collector HTTP endpoint has CORS configured for `http://localhost` and `http://localhost:3000`
- Grafana runs with anonymous admin access (`GF_AUTH_ANONYMOUS_ENABLED=true`)
- Jaeger v2 (2.4.0) — UI on port 16686, gRPC on 16685
- In WSL2, Jaeger and Prometheus ports may not bind to host — use `docker inspect` to verify `PortBindings`

---

## Pending Work (priority order)

### 1. Rate limiting on admin routes
`internal/ratelimit` exists but verify it's wired to routes. Admin endpoints have no rate limiting.

### 2. WebSocket disconnect feedback
No UI indicator when WS drops. User doesn't know they're disconnected. Add a reconnecting/disconnected banner in `Room.tsx`.

### 3. Generic error boundary
No error boundary in React — unhandled render errors show a blank screen. Add a top-level `ErrorBoundary` component in `App.tsx`.

### 4. Turn timeout
`turn_timeout_secs` exists in the `Room` model but is never enforced. Needs a ticker in `runtime/` that auto-forfeits on timeout and broadcasts via WebSocket.

### 5. Rematch flow
Backend has `rematch_votes` table and store methods. Frontend has no UI for it. Needs a post-game screen in `Game.tsx` with a rematch button and vote state.

### 6. Spectator mode
`spectator_links` is in the store but no endpoint or UI exists. Needs: `POST /api/v1/rooms/{roomID}/spectate` to generate a link, and a read-only game view.

### 7. JWT session revocation
No way to invalidate active sessions without rotating `JWT_SECRET`. Consider a `session_revocations` table or Redis set for revoked JTIs.

### 8. Observability iframes in admin panel
Grafana iframe works after adding `GF_SERVER_ROOT_URL` and `GF_SERVER_SERVE_FROM_SUB_PATH=true` + Caddy `/grafana/*` proxy. Jaeger v2 subpath support is limited — may need to open in new tab instead.

### 9. sessionStorage cache for player
`useAppStore` reloads player from `/auth/me` on every page refresh. Could cache in `sessionStorage` to avoid the flash.

### 10. Production hardening (do last)
- PostgreSQL backup (pg_dump via cron or a sidecar container, push to S3-compatible storage)
- Watchtower rollback strategy — pin image tags or add a health check before Watchtower promotes
- Frontend container health check
- Rotate JWT_SECRET procedure documented
- `docker-compose.prod.yml` review: confirm all internal services use `expose:` not `ports:`