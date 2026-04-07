# Frontend Overview

React 19 + TypeScript SPA served via Nginx in Docker. All API calls go through Traefik -- the frontend never talks to services directly.

## Stack

| Layer | Technology |
|-------|-----------|
| Framework | React 19 |
| Routing | TanStack Router (file-based, `src/routes/`) |
| Server state | TanStack Query |
| Client state | Zustand (`src/stores/store.ts`) |
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
  stores/             # Zustand store
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

## Store

Single Zustand store (`src/stores/store.ts`) managing:

- **Auth**: current player
- **WebSockets**: room socket + player socket (persistent personal channel)
- **Spectator**: spectator mode flag + count
- **Presence**: online/offline tracking map
- **Queue**: matchmaking status (idle/queued/match found)
- **Settings**: fully resolved player settings with localStorage cache + backend sync

## API Layer

The API client lives in `src/lib/api.ts`:

- `request<T>(path, init?)` -- base HTTP client with auto-refresh on 401 and OpenTelemetry tracing
- `validatedRequest<T>(schema, path, init?)` -- wraps `request()` + Zod validation; throws `SchemaError` on mismatch

Session-specific endpoints (move, surrender, rematch, pause, etc.) are in `src/lib/api/sessions.ts`.

WebSocket URLs are constructed by `wsRoomUrl(roomId)` and `wsPlayerUrl(playerId)`.

## i18n

Two languages supported: English (`en.json`) and Spanish (`es.json`) in `src/locales/`.

Flat namespace with section prefixes: `common.*`, `settings.*`, `lobby.*`, `game.*`, etc. Language changes are persisted in player settings and applied via `i18n.changeLanguage()`.

## Games

See [game-packages.md](game-packages.md) for the plugin architecture and package boundaries.
