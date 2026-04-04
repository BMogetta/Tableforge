# Deploy to Raspberry Pi

## Why
The app needs to run on a Raspberry Pi 5 (ARM64). CI/CD already builds ARM64
images to ghcr.io. What's missing is the deployment mechanism on the Pi itself.

## What to create

### 1. `docker-compose.prod.yml` — production override
Override all services to use pre-built images from ghcr.io instead of
local builds. Each service pins to `${DEPLOY_SHA:-latest}`:

```yaml
services:
  game-server:
    image: ghcr.io/${GITHUB_OWNER}/tableforge-game-server:${DEPLOY_SHA:-latest}
    build: !override  # disable local build
  # ... same for all 8 services + frontend
```

### 2. `scripts/deploy.sh` — deploy script for the Pi
```bash
#!/bin/bash
# Usage: ./scripts/deploy.sh [SHA]
# Pulls latest images (or pinned SHA), backs up DB, restarts services
# one by one with health check verification.
```

Features:
- Accepts optional SHA argument (defaults to `latest`)
- Runs `pg_dump` backup before deploy
- Restarts services one at a time (zero-downtime rolling)
- Health check after each service restart
- Aborts and reports if any service fails health check
- Logs deploy to a file (`/var/log/tableforge-deploy.log`)

### 3. `scripts/rollback.sh` — quick rollback
```bash
#!/bin/bash
# Usage: ./scripts/rollback.sh
# Deploys the previous git SHA's images.
```

### 4. Maintenance mode in Traefik
Add a maintenance middleware that returns a static 503 page.
Toggle via `touch /opt/tableforge/maintenance.flag`.

### 5. Update `Makefile`
```makefile
deploy:
    @bash scripts/deploy.sh $(SHA)
rollback:
    @bash scripts/rollback.sh
maintenance-on:
    @touch maintenance.flag && docker compose exec traefik kill -HUP 1
maintenance-off:
    @rm -f maintenance.flag && docker compose exec traefik kill -HUP 1
```

### 6. Update `.env.example`
```
# Deploy
GITHUB_OWNER=bmogetta
DEPLOY_SHA=latest
```

## Files to create/modify
- `docker-compose.prod.yml` (new)
- `scripts/deploy.sh` (new)
- `scripts/rollback.sh` (new)
- `infra/traefik/config/maintenance.yml` (new) — maintenance page middleware
- `infra/maintenance.html` (new) — static 503 page
- `Makefile` — add deploy/rollback/maintenance targets
- `.env.example` — add GITHUB_OWNER, DEPLOY_SHA

## Dependencies
- CI/CD workflows must be pushing images to ghcr.io first
- Pi must have `docker login ghcr.io` configured
- Pi must have the repo cloned at `/opt/tableforge`

## Testing
- `docker compose -f docker-compose.yml -f docker-compose.services.yml -f docker-compose.prod.yml config --quiet`
- Dry run deploy on local machine (without --profile production)
