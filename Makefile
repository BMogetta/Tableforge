.PHONY: up up-app up-all up-prod up-test down build seed-test test test-one test-ui test-e2e-readme test-routing smoke-test coverage check-i18n logs ps clean clean-test gen-types gen-proto setup lint version

# ── Build metadata ────────────────────────────────────────────────────────────

# Resolved from the nearest git tag + commits ahead + short SHA, with -dirty
# suffix when the working tree has uncommitted changes. Injected into the
# frontend build as APP_VERSION and shown in the VersionBadge component.
APP_VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo unknown)
export APP_VERSION

version:
	@echo $(APP_VERSION)

# ── Docker ────────────────────────────────────────────────────────────────────

networks:
	docker network create app_network 2>/dev/null || true
	docker network create data_network 2>/dev/null || true
	docker network create monitoring_network 2>/dev/null || true

up: networks
	docker compose up --build -d

up-app: networks
	docker compose --profile app up --build -d

up-all: networks
	docker compose --profile app --profile monitoring up --build -d

up-prod: networks
	docker compose -f docker-compose.yml -f docker-compose.services.yml -f docker-compose.monitoring.yml --profile production --profile app --profile monitoring up -d --build

up-test: networks
	TEST_MODE=true RATE_LIMIT_AVG=9999 RATE_LIMIT_BURST=9999 MATCHMAKER_TICK_INTERVAL=1s MATCHMAKER_SPREAD_PER_SEC=20 docker compose -f docker-compose.yml --profile app up --build -d

# Stop all services and remove containers.
down:
	docker compose --profile app --profile monitoring --profile production down

# Rebuild all images without starting.
build:
	docker compose --profile app --profile monitoring --profile production build

# Show running containers.
ps:
	docker compose --profile app --profile monitoring --profile production ps

# Tail logs for all services.
logs:
	docker compose --profile app --profile monitoring --profile production logs -f

# Hard reset — removes all containers, volumes, images and networks for this project.
# WARNING: wipes the database.
reset:
	docker compose --profile app --profile monitoring --profile production down -v --rmi local --remove-orphans
	docker network rm app_network data_network monitoring_network 2>/dev/null || true

# ── Database ──────────────────────────────────────────────────────────────────

# Create test players for Playwright and save their IDs.
# Runs seed-test binary inside a temporary container on data_network
# so it can reach postgres without exposing any ports.
# Requires: make up-test first.
# Output: frontend/tests/e2e/.players.json
seed-test:
	@mkdir -p frontend/tests/e2e
	docker build -t recess-seed-test tools/seed-test
	docker run --rm \
		--network data_network \
		-e DATABASE_URL=postgres://recess:$${RECESS_DB_PASSWORD:-recess}@postgres:5432/recess?sslmode=disable \
		recess-seed-test \
	> frontend/tests/e2e/.players.json
	@echo "Test players created:"
	@cat frontend/tests/e2e/.players.json

# ── Tests ─────────────────────────────────────────────────────────────────────

# Run all Go + Vitest tests, collect coverage, and update README.md.
# Does NOT require Docker — only runs unit/integration tests.
coverage:
	@bash scripts/update-coverage.sh

# Verify Traefik routes reach the correct services.
# Requires: make up-app first.
test-routing:
	@bash scripts/test-routing.sh

# Run Playwright tests and update the e2e section in README.md.
# Requires: make up-test && make seed-test first.
test-e2e-readme:
	@bash scripts/update-e2e-readme.sh

# Fast API-level smoke test with curl (~5s). Validates core flows without Playwright.
# Requires: make up-test && make seed-test first.
smoke-test:
	@bash scripts/smoke-test.sh

# Run all Playwright tests.
# Requires: make up-test && make seed-test first.
test:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && npx playwright test

# Run a single Playwright test by name (partial match).
# Usage: make test-one NAME="turn timeout ends the game"
test-one:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && npx playwright test --grep "$(NAME)"

# Open Playwright UI mode for interactive test development.
test-ui:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && npx playwright test --ui

# ── Cleanup ───────────────────────────────────────────────────────────────────

# Remove test auth state and player fixtures.
clean-test:
	rm -rf frontend/tests/e2e/.auth frontend/tests/e2e/.players.json

# Remove all docker volumes (wipes the database).
clean:
	docker compose --profile app --profile monitoring --profile production down -v

# ── i18n ──────────────────────────────────────────────────────────────────

# Check that all locale files have matching keys.
check-i18n:
	@node scripts/check-i18n.mjs

# ── API Types ─────────────────────────────────────────────────────────────────

# Generate TypeScript types from JSON Schema definitions.
# Source of truth for request/response DTOs shared between Go and TS.
# Run this after adding or modifying shared/schemas/*.json.
gen-types:
	@node scripts/gen-schema-zod.mjs

# Regenerate all protobuf Go stubs from .proto definitions.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
gen-proto:
	@for proto in shared/proto/*/v1/*.proto; do \
		echo "  $$proto"; \
		protoc --go_out=. --go_opt=paths=source_relative \
		       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		       -I. "$$proto"; \
	done
	@echo "✓ Protobuf stubs regenerated"

# ── Setup ─────────────────────────────────────────────────────────────────────

# Verify that all required tools are installed.
setup:
	@echo "Checking required tools..."
	@fail=0; \
	check() { \
		if command -v $$1 >/dev/null 2>&1; then \
			printf "  %-20s ✓ %s\n" "$$1" "$$($$1 $$2 2>&1 | head -1)"; \
		else \
			printf "  %-20s ✗ missing\n" "$$1"; \
			fail=1; \
		fi; \
	}; \
	check go version; \
	check node --version; \
	check npm --version; \
	check docker --version; \
	check docker compose version 2>/dev/null || check docker-compose --version; \
	check jq --version; \
	echo ""; \
	echo "Code generation (required for make gen-types / gen-proto):"; \
	check protoc --version; \
	check protoc-gen-go --version; \
	check protoc-gen-go-grpc --version; \
	echo ""; \
	echo "Optional:"; \
	check golangci-lint --version; \
	check npx --version; \
	echo ""; \
	if [ "$$fail" = "1" ]; then \
		echo "Some required tools are missing. Install them and re-run make setup."; \
		exit 1; \
	else \
		echo "All required tools installed."; \
	fi
	@echo ""
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo ""
	@echo "Installing MCP game-server tool dependencies..."
	@cd tools/mcp-game && npm install

# ── Lint ──────────────────────────────────────────────────────────────────────

# Run linters across the entire monorepo.
# Go: uses go vet (always matches Go version). Run golangci-lint separately if installed.
lint:
	@echo "=== Go (go vet) ==="
	@fail=0; \
	for svc in services/*/; do \
		[ -f "$$svc/go.mod" ] || continue; \
		echo "  $$svc"; \
		cd $(CURDIR)/$$svc && go vet ./... 2>&1 | sed 's/^/    /' || fail=1; \
	done; \
	if [ "$$fail" = "1" ]; then exit 1; fi
	@echo ""
	@echo "=== Frontend (Biome) ==="
	cd frontend && npx @biomejs/biome lint ./src || true