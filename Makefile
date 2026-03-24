.PHONY: up up-test down build seed seed-test test test-ui logs ps clean clean-test gen-types

# --- Docker ------------------------------------------------------------------

# Start all services in development mode.
up:
	docker compose --profile monitoring up --build

# Start all services with test auth bypass enabled.
up-test:
	TEST_MODE=true docker compose --profile monitoring up --build

# Stop all services and remove containers.
down:
	docker compose --profile monitoring down

# Rebuild all images without starting.
build:
	docker compose --profile monitoring build

# Show running containers.
ps:
	docker compose --profile monitoring ps

# Tail logs for all services.
logs:
	docker compose --profile monitoring logs -f

# --- Database ----------------------------------------------------------------

# Seed the owner email — runs inside the game-server container where DB is reachable.
# Requires: OWNER_EMAIL=... make seed
seed:
	docker compose exec -e OWNER_EMAIL=$(OWNER_EMAIL) game-server /bin/seeder

# Create test players for Playwright and save their IDs.
# Requires: make up-test first.
# Output: frontend/tests/e2e/.players.json
seed-test:
	@mkdir -p frontend/tests/e2e
	docker compose --profile monitoring exec game-server /bin/seed-test > frontend/tests/e2e/.players.json
	@echo "Test players created:"
	@cat frontend/tests/e2e/.players.json

# --- Tests -------------------------------------------------------------------

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

# --- Cleanup -----------------------------------------------------------------

# Remove test auth state and player fixtures.
clean-test:
	rm -rf frontend/tests/e2e/.auth frontend/tests/e2e/.players.json

# Remove all docker volumes (wipes the database).
clean:
	docker compose --profile monitoring down -v

# --- API Types ---------------------------------------------------------------

# Generate TypeScript types from the Go API.
# Requires: swag installed (go install github.com/swaggo/swag/cmd/swag@latest)
# Requires: swagger-typescript-api installed (npx swagger-typescript-api)
# Run this after adding or modifying API handlers.
gen-types:
	cd server && swag init -g cmd/server/main.go -o docs \
		--parseDependency --parseInternal --useStructName

	# Apply required-fields patch on top of the generated swagger.json.
	# Edit server/docs/swagger.required-patch.json to keep this list up to date
	# whenever new response types are added.
	cd server/docs && jq -s -f patch-swagger.jq swagger.json swagger.required-patch.json > swagger.patched.json

	cd server/docs && npx swagger-typescript-api generate \
		--path swagger.patched.json \
		--output ../../frontend/src/lib \
		--name api-generated.ts \
		--no-client

	@echo "✓ TypeScript types generated at frontend/src/lib/api-generated.ts"