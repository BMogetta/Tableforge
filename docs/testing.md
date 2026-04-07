# Testing

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

### Test IDs

All interactive elements use `data-testid` via the `testId()` helper:

```tsx
import { testId } from '@/utils/testId'
<button {...testId('submit-btn')}>Submit</button>
<div {...testId(`player-row-${id}`)}>...</div>
```

Test IDs are conditionally rendered -- present in dev/test mode, stripped in production builds. The helper checks `import.meta.env.DEV || import.meta.env.VITE_TEST_MODE`.

There's also `testAttr(name, value)` for data attributes beyond test IDs (e.g., `data-socket-status`).

### Player Fixtures

`make seed-test` outputs a JSON file with 30 player IDs. Tests read this file to get pre-created accounts for multi-player scenarios.

## Scripts

| Script | Command | Purpose |
|--------|---------|---------|
| `scripts/update-coverage.sh` | `make coverage` | Aggregate coverage across all services, update README |
| `scripts/update-e2e-readme.sh` | `make test-e2e-readme` | Run Playwright, update README with results |
| `scripts/test-routing.sh` | `make test-routing` | Verify Traefik routes reach correct services |
| `scripts/check-i18n.mjs` | `make check-i18n` | Compare translation keys across locale files |
| `scripts/gen-schema-zod.mjs` | `make gen-types` | Generate Zod schemas from JSON Schema |
