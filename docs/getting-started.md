# Getting Started

## Prerequisites

- [Make](https://www.gnu.org/software/make/)
- [Docker](https://docs.docker.com/get-docker/) with `docker compose`
- [Go](https://go.dev/dl/) 1.25+
- [Node.js](https://nodejs.org/) 22+
- [jq](https://jqlang.github.io/jq/download/)

For code generation (`make gen-types`, `make gen-proto`):
- [protoc](https://grpc.io/docs/protoc-installation/) + `protoc-gen-go` + `protoc-gen-go-grpc`

## Setup

```bash
git clone <repo-url> && cd recess
make setup                 # verify tools + install frontend deps
cp .env.example .env       # then fill in required values
```

### Environment Variables

The `.env` file controls all secrets and configuration. Required values:

| Variable | How to get it |
|----------|---------------|
| `GITHUB_CLIENT_ID` | [GitHub OAuth App](https://github.com/settings/developers) |
| `GITHUB_CLIENT_SECRET` | Same OAuth app |
| `JWT_SECRET` | `openssl rand -hex 32` |
| `OWNER_EMAIL` | Your GitHub primary email (first-run allowlist) |

All other variables have sensible defaults for local development. See `.env.example` for the full list with comments.

### Database Passwords

Three separate database credentials:

| Variable | Purpose |
|----------|---------|
| `POSTGRES_PASSWORD` | Superuser (`postgres`), admin only |
| `RECESS_DB_PASSWORD` | Application user, owns the `recess` database |
| `UNLEASH_DB_PASSWORD` | Feature flags user, owns the `unleash` database |

Both application users are created automatically on first Postgres startup by `shared/db/migrations/000_init-databases.sh`.

## Running the Project

```bash
make up          # postgres + redis + game-server
make up-app      # + traefik + frontend + all backend services
make up-all      # + full observability stack (Grafana, Tempo, Loki, Prometheus)
make up-prod     # + PgBouncer + pg-backup + cloudflared (production profile)
```

Everything goes through Traefik on port 80. Open `http://localhost` to access the frontend.

## Production Deploy

Production goes online behind a Cloudflare Tunnel — no open ports, no public IP. `make up-prod` starts the `cloudflared` connector alongside the rest of the stack; the connector dials Cloudflare over outbound 443 and routes your configured public hostname to `traefik:80`.

Setup is a one-time Cloudflare dashboard step (tunnel + public hostname pointing at `http://traefik:80`) plus setting `CLOUDFLARE_TUNNEL_TOKEN` in `.env`. The Makefile fails fast if the token is empty. Full walkthrough: [`docs/infrastructure/docker.md`](infrastructure/docker.md#production-profile).

### Useful Commands

```bash
make down        # stop all services
make reset       # hard reset (wipes volumes, images, networks)
make ps          # show running containers
make logs        # tail logs for all services
make lint        # run Go vet + Biome across the monorepo
```

## Running Tests

### Unit Tests

```bash
# All Go tests
go test ./...

# Specific package
go test ./services/game-server/internal/domain/runtime/...

# Frontend (Vitest)
cd frontend && npm test
```

### E2E Tests (Playwright)

```bash
make up-test           # start in test mode (TEST_MODE=true)
make seed-test         # create 30 test players -> frontend/tests/e2e/.players.json
make test              # run all Playwright tests
make test-one NAME="partial test name"
make test-ui           # interactive Playwright UI
make clean-test        # remove auth state + player fixtures
```

See [testing.md](testing.md) for details on test architecture and conventions.

## Code Generation

```bash
make gen-types    # JSON Schema -> Zod schemas + TypeScript types
make gen-proto    # .proto -> Go stubs (gRPC)
```

Run `make gen-types` after modifying `shared/schemas/*.json`.
Run `make gen-proto` after modifying `shared/proto/*/v1/*.proto`.
