.PHONY: up up-app up-all up-test down build seed-test test test-one test-ui logs ps clean clean-test gen-types

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

up-test: networks
	TEST_MODE=true docker compose --profile app up --build -d

# Stop all services and remove containers.
down:
	docker compose --profile app --profile monitoring down

# Rebuild all images without starting.
build:
	docker compose --profile app --profile monitoring build

# Show running containers.
ps:
	docker compose --profile app --profile monitoring ps

# Tail logs for all services.
logs:
	docker compose --profile app --profile monitoring logs -f

# Hard reset — removes all containers, volumes, images and networks for this project.
# WARNING: wipes the database.
reset:
	docker compose --profile app --profile monitoring down -v --rmi local --remove-orphans
	docker network rm app_network data_network monitoring_network 2>/dev/null || true

# ── Database ──────────────────────────────────────────────────────────────────

# Create test players for Playwright and save their IDs.
# Runs seed-test binary inside a temporary container on data_network
# so it can reach postgres without exposing any ports.
# Requires: make up-test first.
# Output: frontend/tests/e2e/.players.json
seed-test:
	@mkdir -p frontend/tests/e2e
	docker build -t tableforge-seed-test tools/seed-test
	docker run --rm \
		--network tableforge_data_network \
		-e DATABASE_URL=postgres://tableforge:tableforge@postgres:5432/tableforge?sslmode=disable \
		tableforge-seed-test \
	> frontend/tests/e2e/.players.json
	@echo "Test players created:"
	@cat frontend/tests/e2e/.players.json

# ── Tests ─────────────────────────────────────────────────────────────────────

# Run all Playwright tests.
# Requires: make up-test && make seed-test first.
test:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && \
		TEST_PLAYER1_ID=$$(cat tests/e2e/.players.json | jq -r .player1_id) \
		TEST_PLAYER2_ID=$$(cat tests/e2e/.players.json | jq -r .player2_id) \
		TEST_PLAYER3_ID=$$(cat tests/e2e/.players.json | jq -r .player3_id) \
		npx playwright test

# Run a single Playwright test by name (partial match).
# Usage: make test-one NAME="turn timeout ends the game"
test-one:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && \
		TEST_PLAYER1_ID=$$(cat tests/e2e/.players.json | jq -r .player1_id) \
		TEST_PLAYER2_ID=$$(cat tests/e2e/.players.json | jq -r .player2_id) \
		TEST_PLAYER3_ID=$$(cat tests/e2e/.players.json | jq -r .player3_id) \
		npx playwright test --grep "$(NAME)"

# Open Playwright UI mode for interactive test development.
test-ui:
	@if [ ! -f frontend/tests/e2e/.players.json ]; then \
		echo "Error: run 'make seed-test' first"; exit 1; \
	fi
	cd frontend && \
		TEST_PLAYER1_ID=$$(cat tests/e2e/.players.json | jq -r .player1_id) \
		TEST_PLAYER2_ID=$$(cat tests/e2e/.players.json | jq -r .player2_id) \
		TEST_PLAYER3_ID=$$(cat tests/e2e/.players.json | jq -r .player3_id) \
		npx playwright test --ui

# ── Cleanup ───────────────────────────────────────────────────────────────────

# Remove test auth state and player fixtures.
clean-test:
	rm -rf frontend/tests/e2e/.auth frontend/tests/e2e/.players.json

# Remove all docker volumes (wipes the database).
clean:
	docker compose --profile app --profile monitoring down -v

# ── API Types ─────────────────────────────────────────────────────────────────

# Generate TypeScript types from the Go API.
# Requires: swag installed (go install github.com/swaggo/swag/cmd/swag@latest)
# Run this after adding or modifying API handlers.
gen-types:
	cd services/game-server && swag init -g cmd/server/main.go -o docs \
		--parseDependency --parseInternal --useStructName

	cd services/game-server/docs && jq -s -f patch-swagger.jq swagger.json swagger.required-patch.json > swagger.patched.json

	cd services/game-server/docs && npx swagger-typescript-api generate \
		--path swagger.patched.json \
		--output ../../frontend/src/lib \
		--name api-generated.ts \
		--no-client

	@echo "✓ TypeScript types generated at frontend/src/lib/api-generated.ts"