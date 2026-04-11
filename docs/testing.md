# Testing

Recess has three test layers. Each answers a different question:

| Layer | Question | Speed | Requires Docker? |
|-------|----------|-------|-------------------|
| **Unit tests** | Does this function/component behave correctly in isolation? | Seconds | No |
| **Smoke test** | Are the core API flows working end-to-end? | ~5s | Yes |
| **E2E (Playwright)** | Does the full UI flow work in a real browser? | ~5 min | Yes |

**When to use each:**

- **Unit tests** — run constantly during development. Fastest feedback loop.
  Catch logic bugs in isolated functions, hooks, and handlers.
- **Smoke test** — run after backend changes to validate API contracts before
  waiting for the full Playwright suite. Catches broken endpoints, auth issues,
  request/response shape mismatches, and integration bugs between services.
- **E2E tests** — run before merging or deploying. Catches UI regressions,
  WebSocket race conditions, navigation flows, and visual state transitions
  that the API layer can't verify.

Typical workflow after a backend change:

```
make up-test && make seed-test   # once per session
make smoke-test                  # fast: did I break an endpoint?
make test-one NAME="the flow"    # targeted: does the UI still work?
make test                        # full suite before merge
```

---

## Unit Tests

### Go (backend)

Standard `testing` package. Run from repo root (Go workspace):

```bash
go test ./...                                          # all modules
go test ./services/game-server/internal/domain/runtime/... # specific package
go test -run TestName ./path/to/package/...            # single test
```

Coverage report:

```bash
make coverage    # runs Go + Vitest tests, updates README badges
```

### TypeScript (frontend)

Vitest with React Testing Library:

```bash
cd frontend
npm test          # run all tests
npm run test:ui   # interactive UI
```

---

## Smoke Test

A curl-based script that exercises the core API flows without a browser.
Runs ~20 assertions in ~5 seconds — useful for fast validation after backend changes.

```bash
make smoke-test
```

**What it covers:**
- Auth (test-login, /auth/me)
- Room lifecycle (create, join, settings, start, leave)
- Ready check (empty body regression)
- Game moves (full TicTacToe game, 5 moves)
- Session history (finished state, move count)
- Friends (request, accept, remove)
- Lobby duplicate prevention (409 on second room)

**What it doesn't cover:**
- WebSocket flows (presence, real-time updates, spectator redirect)
- UI rendering, navigation, animations
- Browser-specific behavior (cookies, popups, redirects)

For those, use the full E2E suite.

---

## E2E Tests (Playwright)

### Setup

```bash
make up-test       # starts all services with TEST_MODE=true
make seed-test     # creates 30 test players -> frontend/tests/e2e/.players.json
```

`seed-test` does more than create players:
- Inserts 30 test players with allowed emails
- Cleans up stale rooms/sessions from previous runs
- Resets player state
- Lowers game timeouts from 30s to 5s (faster tests)

### Running

```bash
make test                          # all tests
make test-one NAME="turn timeout"  # grep by name
make test-ui                       # interactive Playwright UI
make clean-test                    # remove auth state + fixtures
```

### Configuration

`frontend/playwright.config.ts`:
- Fully parallel, 4 workers locally, 2 in CI
- Base URL: `http://localhost`
- 60s timeout per test
- Retries: 0 locally, 1 in CI
- Trace on first retry, screenshots only on failure
- Projects: `setup` (auth) then `tests` (Chrome Desktop)

### Player Pool

Tests share a pool of 30 pre-authenticated players. The pool uses file-based
locking to coordinate access across parallel Playwright workers:

1. `auth.setup.ts` authenticates all 30 players and saves browser state
2. Each test fixture calls `acquirePlayers(N, testId)` to lock N players
3. On teardown, `releasePlayers(pool, testId)` unlocks them for reuse

The pool lives in `frontend/tests/e2e/player-pool.ts`. Lock files are in
`.player-pool.lock` / `.player-pool.json`. Run `make clean-test` to reset.

**Fixtures** (`frontend/tests/e2e/fixtures.ts`):
- `players` — 2 players with cleanup and navigation to `/`
- `playersWithSpectator` — 3 players (2 participants + 1 spectator)
- `singlePlayer` — 1 player for solo flows

### Test Suites

| Suite | File | Tests | What it covers |
|-------|------|-------|----------------|
| Auth | `auth.spec.ts` | 1 | Login flow, auth state |
| Chat | `chat.spec.ts` | 5 | Room chat, DMs, message display |
| Settings | `settings.spec.ts` | 7 | User preferences, room settings |
| Game | `game.spec.ts` | 15 | Moves, turn timeout, surrender, rematch |
| Spectator | `spectator.spec.ts` | 12 | Join as spectator, watch game, no interaction |
| Presence | `presence.spec.ts` | 5 | Online/offline dots, disconnect detection |
| Session History | `session-history.spec.ts` | 10 | Replay, event log, stats, navigation |
| Bot | `bot.spec.ts` | — | Playing against bot opponents |
| Lobby | `lobby.spec.ts` | — | Room list, game selection, join by code |

### Test IDs

All interactive elements use `data-testid` via the `testId()` helper:

```tsx
import { testId } from '@/utils/testId'
<button {...testId('submit-btn')}>Submit</button>
<div {...testId(`player-row-${id}`)}>...</div>
```

Test IDs are conditionally rendered -- present in dev/test mode, stripped in production builds. The helper checks `import.meta.env.DEV || import.meta.env.VITE_TEST_MODE`.

There's also `testAttr(name, value)` for data attributes beyond test IDs (e.g., `data-socket-status`).

---

## Scripts

| Script | Command | Purpose |
|--------|---------|---------|
| `scripts/smoke-test.sh` | `make smoke-test` | Fast API-level validation of core flows |
| `scripts/update-coverage.sh` | `make coverage` | Aggregate coverage across all services, update README |
| `scripts/update-e2e-readme.sh` | `make test-e2e-readme` | Run Playwright, update README with results |
| `scripts/test-routing.sh` | `make test-routing` | Verify Traefik routes reach correct services |
| `scripts/check-i18n.mjs` | `make check-i18n` | Compare translation keys across locale files |
| `scripts/gen-schema-zod.mjs` | `make gen-types` | Generate Zod schemas from JSON Schema |
