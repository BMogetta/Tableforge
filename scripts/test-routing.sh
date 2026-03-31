#!/usr/bin/env bash
# test-routing.sh — verify Traefik routes requests to the correct service.
#
# Logic: a response other than 404/502/503 means Traefik routed to the right
# service. We check specific status codes where predictable.
#
# Requires: make up-app (or make up-test) running.
# Usage: make test-routing
set -euo pipefail

BASE="http://localhost"
PASS=0
FAIL=0
FAKE_UUID="00000000-0000-0000-0000-000000000001"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# check "description" expected_status METHOD /path
check() {
  local description="$1"
  local expected_status="$2"
  local method="$3"
  local path="$4"

  actual=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$path" \
    -H "Content-Type: application/json" 2>/dev/null)

  if [ "$actual" = "$expected_status" ]; then
    echo -e "  ${GREEN}✓${NC} $description ($actual)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $description (expected $expected_status, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

# check_not "description" rejected_status METHOD /path
# Passes if the response is NOT the rejected status (i.e. not a Traefik 404).
check_not() {
  local description="$1"
  local rejected="$2"
  local method="$3"
  local path="$4"

  actual=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$path" \
    -H "Content-Type: application/json" 2>/dev/null)

  if [ "$actual" != "$rejected" ]; then
    echo -e "  ${GREEN}✓${NC} $description ($actual, not $rejected)"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} $description (got $rejected — route not matched)"
    FAIL=$((FAIL + 1))
  fi
}

echo "Waiting for services to be healthy..."
for svc in game-server auth-service user-service chat-service ws-gateway rating-service notification-service match-service; do
  for i in $(seq 1 30); do
    if docker inspect --format='{{.State.Health.Status}}' "$svc" 2>/dev/null | grep -q healthy; then
      break
    fi
    if [ "$i" = "30" ]; then
      echo "Timeout waiting for $svc"
      exit 1
    fi
    sleep 1
  done
done
echo ""

# -- game-server (priority 10 catchall) ---------------------------------------
echo "game-server (catchall /api):"
check "GET /api/v1/games"                                401 GET  "/api/v1/games"
check "GET /api/v1/bots/profiles"                        401 GET  "/api/v1/bots/profiles"
check "GET /api/v1/sessions/{id}"                        401 GET  "/api/v1/sessions/$FAKE_UUID"
echo ""

# -- auth-service (priority 100) ----------------------------------------------
echo "auth-service (/auth/*):"
check_not "GET /auth/github (redirect)"                  404 GET  "/auth/github"
check     "GET /auth/me (no cookie)"                     401 GET  "/auth/me"
echo ""

# -- user-service (priority 50) -----------------------------------------------
echo "user-service (profiles, friends, settings, admin):"
check "GET /api/v1/players/{id}/profile"                 401 GET  "/api/v1/players/$FAKE_UUID/profile"
check "GET /api/v1/players/{id}/friends"                 401 GET  "/api/v1/players/$FAKE_UUID/friends"
check "GET /api/v1/players/{id}/settings"                401 GET  "/api/v1/players/$FAKE_UUID/settings"
check "GET /api/v1/admin/players"                        401 GET  "/api/v1/admin/players"
check "GET /api/v1/admin/allowed-emails"                 401 GET  "/api/v1/admin/allowed-emails"
echo ""

# -- chat-service (priority 50) -----------------------------------------------
echo "chat-service (messages, DMs):"
check_not "GET /api/v1/players/{id}/dm (routed)"         404 GET  "/api/v1/players/$FAKE_UUID/dm"
echo ""

# -- rating-service (priority 50) ---------------------------------------------
echo "rating-service (ratings, leaderboard):"
check_not "GET /api/v1/ratings/tictactoe/leaderboard"    404 GET  "/api/v1/ratings/tictactoe/leaderboard"
echo ""

# -- notification-service (priority 50) ---------------------------------------
echo "notification-service:"
check "GET /api/v1/players/{id}/notifications"           401 GET  "/api/v1/players/$FAKE_UUID/notifications"
echo ""

# -- match-service (priority 50) ----------------------------------------------
echo "match-service (queue):"
check "POST /api/v1/queue/join"                          401 POST "/api/v1/queue/join"
echo ""

# -- ws-gateway (priority 150) ------------------------------------------------
echo "ws-gateway (/ws/*):"
check_not "GET /ws/rooms/{id} (upgrade required)"        404 GET  "/ws/rooms/$FAKE_UUID"
check_not "GET /ws/players/{id} (upgrade required)"      404 GET  "/ws/players/$FAKE_UUID"
echo ""

# -- frontend (priority 1) ----------------------------------------------------
echo "frontend (catchall /):"
check "GET / (serves index.html)"                        200 GET  "/"
echo ""

# -- Summary -------------------------------------------------------------------
TOTAL=$((PASS + FAIL))
echo "Results: $PASS/$TOTAL passed"
if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}$FAIL test(s) failed${NC}"
  exit 1
else
  echo -e "${GREEN}All routing tests passed${NC}"
fi
