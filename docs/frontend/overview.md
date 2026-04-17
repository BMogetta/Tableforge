# Frontend Overview

React 19 + TypeScript SPA served via Nginx in Docker. All API calls go through Traefik -- the frontend never talks to services directly.

## Stack

| Layer | Technology |
|-------|-----------|
| Framework | React 19 |
| Routing | TanStack Router (file-based, `src/routes/`) |
| Server state | TanStack Query |
| Client state | Zustand (`src/stores/`) |
| Animations | Motion (Framer Motion fork) |
| Validation | Zod (generated from JSON Schema) |
| i18n | i18next + react-i18next |
| Linting | Biome |
| Testing | Vitest (unit), Playwright (e2e) |
| Telemetry | OpenTelemetry (browser traces + Web Vitals) |

## Directory Structure

```
frontend/src/
  routes/             # file-based routing (TanStack Router)
  features/           # feature modules
  games/              # pluggable game renderers
  stores/             # Zustand stores (split by domain)
  lib/                # API layer, i18n, schema validation
  ui/                 # shared UI components (cards kit)
  styles/             # global.css (design tokens, themes, utilities)
  utils/              # helpers (testId, etc.)
  locales/            # translation files (en.json, es.json)
```

## Routes

| File | Path | Purpose |
|------|------|---------|
| `__root.tsx` | -- | Root layout, auth check, friends panel, DM inbox |
| `index.tsx` | `/` | Lobby (room list, create game, leaderboard) |
| `login.tsx` | `/login` | GitHub OAuth login |
| `rooms.$roomId.tsx` | `/rooms/:roomId` | Room view (player list, chat, settings) |
| `game.$sessionId.tsx` | `/game/:sessionId` | Active game session |
| `sessions.$sessionId.history.tsx` | `/sessions/:sessionId/history` | Game replay |
| `profile.$playerId.tsx` | `/profile/:playerId` | Player profile |
| `admin.tsx` | `/admin` | Admin dashboard |
| `rate-limited.tsx` | `/rate-limited` | Rate limit error page |

## Feature Modules

Each feature is a directory under `src/features/` with components, hooks, and API calls scoped to that domain:

| Feature | Purpose |
|---------|---------|
| `auth` | GitHub OAuth login flow |
| `lobby` | Room browsing, creation, matchmaking queue UI |
| `room` | Room management, player list, chat, settings |
| `game` | Game board, turn timer, pause/surrender, replay |
| `profile` | Player stats, achievements, match history |
| `friends` | Friends list, friend requests, DM conversations |
| `notifications` | Toast notifications, notification center |
| `admin` | Admin tools |
| `errors` | Error screens, 404, splash |
| `devtools` | React/Router/Query devtools (dev only) |

Features import directly from subfolders -- there are no barrel exports.

## Stores

State is split across five domain-focused Zustand stores under `src/stores/`:

| Store | File | Responsibility |
|-------|------|----------------|
| `useAppStore` | `store.ts` | Current player, DM overlay target |
| `useSocketStore` | `socketStore.ts` | `GatewaySocket` lifecycle (connect/disconnect) |
| `useRoomStore` | `roomStore.ts` | Active room id, view, owner, settings, spectator flag/count, presence map, `joinRoom`/`leaveRoom` |
| `useQueueStore` | `queueStore.ts` | Matchmaking status (`idle` / `queued` / `match_found`) |
| `useSettingsStore` | `settingsStore.ts` | Resolved player settings with localStorage cache + backend sync |

`store.ts` re-exports the domain stores so legacy `import { X } from '@/stores/store'` keeps working; new code should import directly from the specific store.

### WebSocket

A single `GatewaySocket` (`src/lib/ws.ts`) replaces the former dual `RoomSocket` + `PlayerSocket`. It connects once at login to `/ws/players/{playerID}` and dynamically subscribes/unsubscribes from room channels via `subscribe_room` / `unsubscribe_room` control messages. All events — player-scoped (queue, DMs, notifications) and room-scoped (game flow, chat, presence) — arrive over this connection.

`useRoomStore` registers a single listener on the gateway to route room-level events (`presence_update`, `owner_changed`, `setting_updated`, spectator counts, connection status) into state.

## API Layer

The API client lives in `src/lib/api.ts`:

- `request<T>(path, init?)` -- base HTTP client with auto-refresh on 401 and OpenTelemetry tracing
- `validatedRequest<T>(schema, path, init?)` -- wraps `request()` + Zod validation; throws `SchemaError` on mismatch

Session-specific endpoints (move, surrender, rematch, pause, etc.) are in `src/lib/api/sessions.ts`.

The gateway URL is built by `wsPlayerUrl(playerId)`; room subscription is handled in-band by the `GatewaySocket`.

## i18n

Two languages supported: English (`en.json`) and Spanish (`es.json`) in `src/locales/`.

Flat namespace with section prefixes: `common.*`, `settings.*`, `lobby.*`, `game.*`, etc. Language changes are persisted in player settings and applied via `i18n.changeLanguage()`.

**Positional keys for cross-service contracts.** When the backend publishes an identifier that the frontend renders as text — e.g. achievement tier names — the backend stores the i18n key, not the resolved string. Achievements follow the positional scheme `achievements.{key}.name`, `achievements.{key}.description`, `achievements.{key}.tiers.{N}.name`, `achievements.{key}.tiers.{N}.description`, with `{{threshold}}` interpolation for tiered descriptions. The registry infra lives in `shared/achievements/`; definitions auto-register from leaf packages via `init()`: `shared/achievements/global/` for game-agnostic achievements and `shared/achievements/games/{game}/` for per-game ones. Binaries that need the registry populated (currently `user-service`) blank-import the packages in their `main.go`. The frontend fetches the aggregated registry from `GET /api/v1/achievements/definitions` via TanStack Query (`keys.achievementDefinitions()`) and resolves the keys through i18next; translations live in `src/locales/{en,es}.json`. Downstream consumers (notification-service, WS events) carry the key verbatim and resolve at render time.

## Games

See [game-packages.md](game-packages.md) for the plugin architecture and package boundaries.
