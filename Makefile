.PHONY: up up-test down build seed seed-test test test-ui logs ps clean clean-test

# --- Docker ------------------------------------------------------------------

# Start all services in development mode.
up:
	docker compose up --build

# Start all services with test auth bypass enabled.
up-test:
	TEST_MODE=true docker compose up --build

# Stop all services and remove containers.
down:
	docker compose down

# Rebuild all images without starting.
build:
	docker compose build

# Show running containers.
ps:
	docker compose ps

# Tail logs for all services.
logs:
	docker compose logs -f

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
	docker compose exec game-server /bin/seed-test > frontend/tests/e2e/.players.json
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
	docker compose down -v