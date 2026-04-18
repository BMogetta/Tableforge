# Feature Flags Implementation Plan

Plan de implementación de Unleash feature flags para Tableforge: seeder de flags, wiring backend + frontend, gating de devtools en prod para admins/owners, y tests.

**Leyenda:** `[ ]` pendiente · `[~]` en progreso · `[x]` hecho y validado · `[!]` bloqueado

**Flags target del seed inicial** (ver decisión ya tomada en conversación):
1. `maintenance-mode` — kill switch global (default OFF)
2. `ranked-matchmaking-disabled` — kill para match-service (default OFF)
3. `chat-enabled` — gate de chat/DMs (default ON)
4. `achievements-enabled` — gate de tracking de logros (default ON)
5. `game-tictactoe-enabled` — per-game toggle (default ON)
6. `game-rootaccess-enabled` — per-game toggle (default ON)
7. `devtools-for-admins` — gate para devtools en prod, solo visibles con role `owner` (default OFF)

**Nota de nomenclatura**: el JWT usa roles `player` / `manager` / `owner`. No existe "admin" en el sistema — el rol más alto es `owner`. El plan usa "admin" solo en los labels de UI/docs por familiaridad; los checks de código comparan contra `sharedmw.RoleOwner`.

---

## Fase 0 — Decisiones y prereqs

- [x] **0.1** Env de Unleash: usar `development` en dev local; en k3s prod se usa `production`. El seeder del compose solo toca `development`.
  - Verificado local: Unleash v6 ya trae ambos envs (`development`, `production`) habilitados por default. No hace falta crearlos. El seeder va a omitir el POST de envs e ir directo a habilitar flags en `development`.
- [x] **0.2** Token convention: `*:*.unleash-insecure-api-token` (ya seedeado por `INIT_ADMIN_API_TOKENS` en compose). Unscoped, sirve tanto para `/api/client` (backend SDK) como `/api/frontend` (React SDK). En k3s prod se rotará a tokens scoped.
  - Verificado local: `GET /api/admin/api-tokens` lista `*:*.unleash-insecure-api-token` con type `admin`, environment `*`.
- [x] **0.3** Config shared: env vars nuevas en `.env.example` y `shared/config`:
  - `UNLEASH_URL` (default `http://unleash:4242/api`)
  - `UNLEASH_API_TOKEN` (default `*:*.unleash-insecure-api-token`)
  - `UNLEASH_ENV` (default `development`)
  - AppName se pasa como arg a `LoadUnleash(appName)` en cada `cmd/server/main.go`.
  - `shared/config/unleash.go` expone `UnleashConfig{URL, Token, AppName, Environment}` y `LoadUnleash`. Tests cubren defaults y overrides.

---

## Fase 1 — Seeder de flags

Init container que corre post-healthy de Unleash y crea las 7 flags de forma idempotente. El mismo patrón portará a k3s en Fase 5 como ArgoCD PostSync hook.

### 1.1 Definición del seed

- [x] **1.1.a** Crear `infra/unleash/flags.json` con los 7 flags.
  - Formato simplificado vs plan original: array de `{name, description, type}`. `project`, `impressionData` y otros campos opcionales los default la API al crear — no los repito en el seed para reducir ruido.
- [x] **1.1.b** Crear `infra/unleash/environments.json` con el estado por env:
  - Array de `{feature, environment, enabled}`. La estrategia `default` la agrega el script (la API no permite enabled=true sin estrategia asociada, pero la maneja el seed.sh, no el JSON).
  - 7 entradas, todas en `development`. Para agregar otro env en el futuro, agregar entradas al mismo array.
- [x] **Validación 1.1**: ambos JSON válidos (jq length == 7); los flags listados en `flags.json` matchean 1:1 con los de `environments.json` (`jq -s` diff).

### 1.2 Script de seeding

- [x] **1.2.a** Crear `infra/unleash/seed.sh`:
  - Espera `/health` (timeout 60s, backoff).
  - Itera `flags.json`: `POST /api/admin/projects/default/features` (409 = ya existe, OK).
  - Itera `environments.json`: `POST /api/admin/projects/default/features/{name}/environments/{env}/on|off` + `POST .../strategies` con `default`.
  - Exit 0 en éxito total, ≠ 0 si algún POST devuelve 5xx.
  - Idempotente: volver a correr no rompe nada.
  - Detalle: strip `/api` trailing si está presente en `UNLEASH_URL`, así la misma env var (convención Go SDK con `/api`) funciona para el seeder.
- [x] **1.2.b** Shebang `#!/bin/sh`, deps: `curl`, `jq` (ambos en `alpine:3.20`).
- [x] **Validación 1.2**: probado local contra Unleash live. Primer run: crea los 7 flags + aplica estados. Segundo run: `= exists` en los 7 (409 → success). Estado final verificado vía `/api/admin/projects/default/features` — matchea la decisión (maintenance/ranked/devtools OFF, resto ON).

### 1.3 Integración al compose

- [x] **1.3.a** Agregar service `unleash-seeder` a `docker-compose.services.yml`:
  - Image: `alpine:3.20@sha256:...` (pineado).
  - Profiles: `app`.
  - Depends on: `unleash (healthy)`.
  - Command: `apk add curl jq && /seed/seed.sh`.
  - Volume: `./infra/unleash:/seed:ro`.
  - Env: `UNLEASH_URL`, `UNLEASH_TOKEN`.
  - `restart: "no"`.
- [x] **1.3.b** Actualizar `docs/infrastructure/feature-flags.md`:
  - Nueva sección "Configuration" con el flujo JSON → seed.sh → reconcile; instrucciones para agregar flags nuevas.
  - Env vars del seeder documentadas.
  - CLI import/export legacy mantenido como alternativa para migrations ad-hoc.
- [x] **Validación 1.3**: `docker compose up -d unleash-seeder` → exit 0, logs muestran los 7 `= exists` + estados aplicados.

### 1.4 Tests

- [x] **1.4.a** Test shell: `infra/unleash/seed_test.sh` corre `seed.sh` dos veces (verifica idempotencia) y luego pide cada flag via API y compara enabled contra `environments.json`. Ejecutable vía `make test-unleash-seed` (nombre elegido para no colisionar con el `seed-test` existente que crea test players).
- [x] **Validación 1.4**: `make test-unleash-seed` pasa con el stack corriendo.

---

## Fase 2 — Shared Go client + middleware

### 2.1 Cliente compartido

- [x] **2.1.a** Agregar dependencia `github.com/Unleash/unleash-client-go/v4@v4.5.0` a `shared/go.mod`.
- [x] **2.1.b** Crear `shared/featureflags/client.go`:
  - `Init(cfg UnleashConfig) (*Client, error)` — inicializa con `WithAppName`, `WithUrl`, `WithEnvironment`, `WithCustomHeaders`, y un `slogListener` para no perder errores/warnings del SDK.
  - `(*Client).IsEnabled(name, defaultValue) bool` — usa `WithFallback(defaultValue)` del SDK; nil-safe (client nil → default).
  - `(*Client).Close()` — cleanup, nil-safe.
  - `Checker` interface exportada para que middleware/handlers dependan de la abstracción, no de `*Client`.
- [x] **2.1.c** Crear `shared/featureflags/client_test.go`:
  - Fake Unleash httptest server con endpoints `/client/{register,features,metrics}`.
  - Tests: flag ON → true; flag OFF → false; flag desconocido → default; nil client → default; `Close` sobre nil no panica.
- [x] **Validación 2.1**: `go test ./shared/featureflags/...` → `ok` en 0.28s, 5 tests.

### 2.2 Middleware de maintenance

- [x] **2.2.a** Crear `shared/middleware/maintenance.go`:
  - `Maintenance(flags featureflags.Checker) func(http.Handler) http.Handler` — toma la interface, no el struct, para facilitar stubs.
  - POST/PUT/PATCH/DELETE con flag ON → 503 `{"error":"maintenance"}`.
  - GET/HEAD/OPTIONS siempre pasan.
  - `MaintenancePaths` exportado (`/healthz`, `/readyz`, `/metrics`) para que los healthchecks no caigan bajo mantenimiento.
  - Nil-safe: si Checker es nil (SDK init falló), nunca bloquea.
- [x] **2.2.b** `shared/middleware/maintenance_test.go`: 5 tests cubriendo flag OFF con todos los verbs, flag ON bloqueando mutaciones, lecturas siempre pasando, allowlist, nil-checker.
- [x] **Validación 2.2**: `go test ./middleware/...` → `ok` en 0.16s.

### 2.3 Capability helper

- [x] **2.3.a** Crear `shared/featureflags/capability.go`:
  - `Compute(flags Checker, role string) Capabilities` devuelve `{CanSeeDevtools: role == "owner" && flags.IsEnabled(FlagDevtoolsForAdmins, false)}`.
  - Constant `FlagDevtoolsForAdmins` exportada para compartirla entre handler y test.
  - Cambio vs plan inicial: función devuelve un struct `Capabilities` en vez de un bool, así agregar futuras capabilities no cambia la firma del handler. El struct tiene JSON tags listos para el endpoint `/auth/me/capabilities`.
  - Anti-drift: `roleOwner = "owner"` se duplica (import cycle con middleware); documentado en comments.
- [x] **Validación 2.3**: 6 tests en `capability_test.go` (owner+ON, owner+OFF, player+ON, manager+ON, nil-checker+owner, empty-role+ON). Todos pasan.

---

## Fase 3 — Wiring backend

### 3.1 Init en los 8 services

- [ ] **3.1.a** En cada `services/<svc>/cmd/server/main.go`, inicializar el cliente de Unleash tras cargar config, antes del router:
  ```go
  flags, err := featureflags.Init(cfg.Unleash.WithAppName("<svc>"))
  if err != nil { /* log.Warn y seguir con defaults */ }
  defer flags.Close()
  ```
- [ ] **3.1.b** Inyectar `flags` al handler/store según lo necesite cada service.
- [ ] **Validación 3.1**: `make up-app` levanta los 8; logs muestran `unleash: ready` en cada uno; ninguno falla si Unleash está down (fallback a defaults).

### 3.2 Maintenance middleware

- [ ] **3.2.a** Wire el `maintenance.Maintenance(flags)` middleware en el router de cada service, después de auth pero antes de los handlers.
- [ ] **3.2.b** Tests por service: POST de prueba con flag OFF → 2xx, con flag ON → 503.
- [ ] **Validación 3.2**: flipear `maintenance-mode` en la UI → en ≤15s todas las mutaciones devuelven 503 en todos los services; GET siguen pasando.

### 3.3 Flag-gated endpoints

- [ ] **3.3.a** **match-service**: en el handler de `/match/queue` (POST para entrar a ranked queue), si `ranked-matchmaking-disabled` está ON devolver 503 con body `{"error":"ranked_disabled"}`.
  - Test unit con flag ON y OFF.
- [ ] **3.3.b** **chat-service**: en los handlers de mensajes (room + DM), si `chat-enabled` está OFF devolver 503 con body `{"error":"chat_disabled"}`.
  - Test unit.
- [ ] **3.3.c** **user-service**: en el processor de eventos que actualizan logros (consumer de Redis pub/sub), skip el update si `achievements-enabled` está OFF. Endpoints de lectura de logros siguen funcionando.
  - Test unit.
- [ ] **3.3.d** **game-server**: en el endpoint que lista games disponibles para crear rooms, filtrar por `game-{id}-enabled`. Si todos los games están disabled, devolver `[]` + 200.
  - Test unit con los 2 games, distintas combinaciones ON/OFF.
- [ ] **Validación 3.3**: cada flag apaga efectivamente el feature correspondiente in ≤15s después de flipear.

### 3.4 Capability endpoint

- [ ] **3.4.a** Agregar endpoint `GET /me/capabilities` en auth-service (junto a `/auth/me`).
  - Response: `{"canSeeDevtools": bool}`
  - Calcula `CanSeeDevtools(flags, role)` con el role del JWT context.
- [ ] **3.4.b** Registrar en JSON schema (`shared/schemas/get_me_capabilities.response.json`) y regenerar Zod types (`make gen-types`).
- [ ] **3.4.c** Test handler:
  - owner + flag ON → `{"canSeeDevtools": true}`
  - player + flag ON → `{"canSeeDevtools": false}`
  - owner + flag OFF → `{"canSeeDevtools": false}`
  - sin JWT → 401
- [ ] **Validación 3.4**: `curl -H "Authorization: Bearer <owner-jwt>" .../auth/me/capabilities` con flag ON → `true`.

---

## Fase 4 — Wiring frontend

### 4.1 SDK + provider

- [ ] **4.1.a** Instalar `@unleash/proxy-client-react` como dep de frontend.
  - **Preguntar al user antes de agregar la dep** (regla del proyecto).
- [ ] **4.1.b** Crear `frontend/src/lib/flags.ts` con config del cliente: URL `http://unleash.localhost/api/frontend`, clientKey `default:development.unleash-insecure-api-token`, appName `frontend`, refreshInterval 15.
- [ ] **4.1.c** Wrap el árbol en `<FlagProvider>` en `main.tsx`.
- [ ] **Validación 4.1**: el provider se inicializa, `useFlag('maintenance-mode')` devuelve `false` en warm state.

### 4.2 Capability hook

- [ ] **4.2.a** Crear `frontend/src/features/auth/useCapability.ts`:
  - Hook que consume `GET /auth/me/capabilities` vía TanStack Query.
  - Query key namespaced: `['capabilities']`.
  - Refetch on focus + cada 5min.
  - Devuelve `{ canSeeDevtools: boolean }` con loading/error states.
- [ ] **4.2.b** Test con MSW o mock de `fetch`: hook devuelve el value correcto para varias respuestas del server.
- [ ] **Validación 4.2**: test vitest verde.

### 4.3 Maintenance banner

- [ ] **4.3.a** Crear `frontend/src/features/maintenance/MaintenanceBanner.tsx`:
  - Usa `useFlag('maintenance-mode')`.
  - Renderiza banner fijo superior con mensaje i18n'd cuando está ON.
  - Integrado en `App.tsx` / root layout.
- [ ] **4.3.b** i18n keys: `maintenance.banner.title`, `maintenance.banner.message`.
- [ ] **4.3.c** Test component: flag ON → banner visible; flag OFF → no renderiza.
- [ ] **Validación 4.3**: `vitest` verde; visual: flipear flag → banner aparece en ≤15s.

### 4.4 Game registry gate

- [ ] **4.4.a** En `frontend/src/games/registry.tsx`, crear un hook `useEnabledGames()` que lee `useFlag('game-tictactoe-enabled')` y `useFlag('game-rootaccess-enabled')` y devuelve el subset filtrado de `GAME_RENDERERS`.
- [ ] **4.4.b** El lobby usa `useEnabledGames()` en vez de `GAME_RENDERERS` directo para la grilla de "crear room".
- [ ] **4.4.c** Test: ambas flags ON → 2 games visibles; una OFF → 1 game; ambas OFF → lista vacía.
- [ ] **Validación 4.4**: vitest + visual.

### 4.5 Chat + achievements gates

- [ ] **4.5.a** En componentes `ChatPopover` y DM inbox: si `chat-enabled` OFF, mostrar estado "Chat temporarily disabled" en vez del input.
- [ ] **4.5.b** En el panel de achievements: si `achievements-enabled` OFF, mostrar "Achievement tracking paused — counters may be behind". Los datos ya registrados siguen visibles.
- [ ] **4.5.c** Tests component.
- [ ] **Validación 4.5**: vitest + visual.

### 4.6 Admin devtools panel (el cambio grande)

- [ ] **4.6.a** Crear `frontend/src/features/devtools/AdminDevtoolsPanel.tsx`:
  - Default export (para que `React.lazy` funcione).
  - Importa `TanStackRouterDevtoolsInProd` (no el dev-only), `ReactQueryDevtoolsPanel`, `WsDevtools`, `ScenarioPicker`.
  - **No incluye** `pacerDevtoolsPlugin` (es no-op en prod builds; ver conversación).
  - Arma el TanStackDevtools wrapper con los 4 plugins.
- [ ] **4.6.b** En `main.tsx`:
  - Mantener el render actual de devtools en dev (`isDev`) como está.
  - Agregar en paralelo un componente nuevo `<AdminDevtoolsGate />` que en prod builds usa `useCapability()`:
    - Si `canSeeDevtools === true` → renderiza `<Suspense><LazyAdminDevtoolsPanel /></Suspense>`
    - Si no → null
  - El lazy import: `const AdminDevtoolsPanel = lazy(() => import('./features/devtools/AdminDevtoolsPanel'))`.
- [ ] **4.6.c** Verificar tree-shaking: `npm run build` + inspect que `AdminDevtoolsPanel` queda en un chunk separado, no en el entry principal. Usar `vite-bundle-analyzer` o similar si hace falta.
- [ ] **4.6.d** Tests:
  - Vitest con el capability mocked a `false` → panel no renderiza.
  - Vitest con capability a `true` → el Suspense renderiza algo (asserción laxa, el contenido del devtools no se puede aserter sin mocks complejos).
- [ ] **Validación 4.6**: build de prod; flag ON + user owner → devtools aparece; flag OFF o user player → no. Chunk separado confirmado en `dist/assets/`.

### 4.7 Limpieza

- [ ] **4.7.a** Update `docs/infrastructure/feature-flags.md` y/o crear `docs/frontend/feature-flags.md`:
  - Cómo agregar un flag nuevo (seed + código).
  - Patrón `useFlag` vs `useCapability` (diferencia: flag es cliente público, capability es server-calculada).
  - Cuándo preferir una u otra.
- [ ] **Validación 4.7**: doc review.

---

## Fase 5 — E2E

### 5.1 Playwright specs

- [ ] **5.1.a** `frontend/tests/e2e/flags-maintenance.spec.ts`:
  - Setup: flipear `maintenance-mode` ON via API.
  - Assert: banner visible en home.
  - Assert: crear room (POST) devuelve 503.
  - Teardown: flipear OFF.
- [ ] **5.1.b** `frontend/tests/e2e/flags-game-toggle.spec.ts`:
  - Setup: disable `game-rootaccess-enabled`.
  - Assert: solo TicTacToe aparece en el grid de "new room".
  - Teardown: enable.
- [ ] **5.1.c** `frontend/tests/e2e/flags-devtools.spec.ts`:
  - Login como owner, enable `devtools-for-admins`.
  - Assert: ícono/trigger de devtools visible.
  - Login como player → no visible aunque flag esté ON.
  - Flag OFF → no visible para nadie.
- [ ] **Validación 5.1**: `make test-one NAME=flags` pasa los 3.

---

## Fase 6 — Migración a k3s (pendiente de Fase 5 del plan CI/CD)

Cuando llegue Fase 5 del plan CI/CD:

- [ ] **6.1** Convertir `unleash-seeder` (compose) a un Kubernetes `Job` con annotation `argocd.argoproj.io/hook: PostSync`. La imagen (alpine + curl + jq), el `seed.sh` y los JSON se reutilizan tal cual.
- [ ] **6.2** ConfigMap para los JSON del seed en vez de volume mount.
- [ ] **6.3** Agregar env `production` a Unleash; el seeder toca `development` Y `production` en k3s con estados posiblemente distintos (ej. devtools-for-admins quizás ON por default en prod para owners).
- [ ] **6.4** Rotar el admin token a tokens scoped (client-only y frontend-only).

Este bloque **no se ejecuta ahora** — está documentado para no perder el camino de migración.

---

## Notas / decisiones

- **2026-04-18** — Nomenclatura: el sistema usa `player`/`manager`/`owner`. El flag `devtools-for-admins` en la UI de Unleash mantiene el nombre "admin" por familiaridad; el check de código compara contra `RoleOwner`. Considerar renombrar la flag a `devtools-for-owners` si causa confusión (trade-off: cambiar el nombre rompe el seed existente).
- **2026-04-18** — `pacerDevtoolsPlugin` NO se incluye en el bundle de prod porque su export principal es un no-op en `NODE_ENV !== 'development'`. Solo router + query + custom tools.
- **2026-04-18** — Refresh interval del SDK: 15s. Eso implica que un flip en la UI tarda hasta 15s en propagarse a backend + frontend. Aceptable para homelab; en prod se puede bajar.
- **2026-04-18** — Default values: `maintenance-mode` default `false` (nunca accidentalmente en mantenimiento si Unleash se cae); gates default `true` (no romper el producto por SDK caído); `devtools-for-admins` default `false` (minimizar blast radius de leaks accidentales).
- **2026-04-18** — Test strategy: cada sub-fase tiene unit tests. Fase 5 agrega E2E. No hay integration tests separados porque los unit tests ya ejercitan el middleware contra un fake Unleash HTTP server.

## Dependencias nuevas (requieren confirmación del user)

Por la regla "Don't add dependencies without asking first" del proyecto:

1. **Go backend**: `github.com/Unleash/unleash-client-go/v4` (shared module).
2. **Frontend**: `@unleash/proxy-client-react` (npm).

Ambas estándar del ecosistema Unleash, pero las menciono explícitamente acá para pedir confirmación antes de la Fase 2.1 / 4.1.
