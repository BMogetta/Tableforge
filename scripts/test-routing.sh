#!/usr/bin/env bash
# test-routing.sh — verify Traefik routes requests to the correct service.
#
# Logic: a response other than 404/502/503 means Traefik routed to the right
# service. We check specific status codes where predictable.
#
# After all checks, updates the <!-- routing:start --> section in README.md.
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

# Collect results for README update: "route|service|pass/fail"
RESULTS=()

# record "route_display" "service" pass|fail
record() { RESULTS+=("$1|$2|$3"); }

# check "description" expected_status METHOD /path "route_display" "service"
check() {
  local description="$1"
  local expected_status="$2"
  local method="$3"
  local path="$4"
  local route_display="$5"
  local service="$6"

  actual=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$path" \
    -H "Content-Type: application/json" 2>/dev/null)

  if [ "$actual" = "$expected_status" ]; then
    echo -e "  ${GREEN}✓${NC} $description ($actual)"
    PASS=$((PASS + 1))
    record "$route_display" "$service" "pass"
  else
    echo -e "  ${RED}✗${NC} $description (expected $expected_status, got $actual)"
    FAIL=$((FAIL + 1))
    record "$route_display" "$service" "fail"
  fi
}

# check_not "description" rejected_status METHOD /path "route_display" "service"
check_not() {
  local description="$1"
  local rejected="$2"
  local method="$3"
  local path="$4"
  local route_display="$5"
  local service="$6"

  actual=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$BASE$path" \
    -H "Content-Type: application/json" 2>/dev/null)

  if [ "$actual" != "$rejected" ]; then
    echo -e "  ${GREEN}✓${NC} $description ($actual, not $rejected)"
    PASS=$((PASS + 1))
    record "$route_display" "$service" "pass"
  else
    echo -e "  ${RED}✗${NC} $description (got $rejected — route not matched)"
    FAIL=$((FAIL + 1))
    record "$route_display" "$service" "fail"
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
check     "GET /api/v1/games"             401 GET  "/api/v1/games"            "GET /api/v1/games"            "game-server"
check     "GET /api/v1/bots/profiles"     401 GET  "/api/v1/bots/profiles"    "GET /api/v1/bots/profiles"    "game-server"
check     "GET /api/v1/sessions/{id}"     401 GET  "/api/v1/sessions/$FAKE_UUID" "GET /api/v1/sessions/{id}" "game-server"
echo ""

# -- auth-service (priority 100) ----------------------------------------------
echo "auth-service (/auth/*):"
check_not "GET /auth/github (redirect)"   404 GET  "/auth/github"             "GET /auth/github"             "auth-service"
check     "GET /auth/me (no cookie)"      401 GET  "/auth/me"                 "GET /auth/me"                 "auth-service"
echo ""

# -- user-service (priority 50) -----------------------------------------------
echo "user-service (profiles, friends, settings, admin):"
check     "GET /api/v1/players/{id}/profile"   401 GET  "/api/v1/players/$FAKE_UUID/profile"   "GET /api/v1/players/{id}/profile"   "user-service"
check     "GET /api/v1/players/{id}/friends"   401 GET  "/api/v1/players/$FAKE_UUID/friends"   "GET /api/v1/players/{id}/friends"   "user-service"
check     "GET /api/v1/players/{id}/settings"  401 GET  "/api/v1/players/$FAKE_UUID/settings"  "GET /api/v1/players/{id}/settings"  "user-service"
check     "GET /api/v1/admin/players"          401 GET  "/api/v1/admin/players"                "GET /api/v1/admin/players"          "user-service"
check     "GET /api/v1/admin/allowed-emails"   401 GET  "/api/v1/admin/allowed-emails"         "GET /api/v1/admin/allowed-emails"   "user-service"
echo ""

# -- chat-service (priority 50) -----------------------------------------------
echo "chat-service (messages, DMs):"
check_not "GET /api/v1/players/{id}/dm"   404 GET  "/api/v1/players/$FAKE_UUID/dm"   "GET /api/v1/players/{id}/dm"   "chat-service"
echo ""

# -- rating-service (priority 50) ---------------------------------------------
echo "rating-service (ratings, leaderboard):"
check     "GET /api/v1/ratings/tictactoe/leaderboard"  401 GET  "/api/v1/ratings/tictactoe/leaderboard"  "GET /api/v1/ratings/{game}/leaderboard"  "rating-service"
echo ""

# -- notification-service (priority 50) ---------------------------------------
echo "notification-service:"
check     "GET /api/v1/players/{id}/notifications"  401 GET  "/api/v1/players/$FAKE_UUID/notifications"  "GET /api/v1/players/{id}/notifications"  "notification-service"
echo ""

# -- match-service (priority 50) ----------------------------------------------
echo "match-service (queue):"
check     "POST /api/v1/queue"            401 POST "/api/v1/queue"            "POST /api/v1/queue"           "match-service"
echo ""

# -- ws-gateway (priority 150) ------------------------------------------------
echo "ws-gateway (/ws/*):"
check_not "GET /ws/rooms/{id}"            404 GET  "/ws/rooms/$FAKE_UUID"     "GET /ws/rooms/{id}"           "ws-gateway"
check_not "GET /ws/players/{id}"          404 GET  "/ws/players/$FAKE_UUID"   "GET /ws/players/{id}"         "ws-gateway"
echo ""

# -- grafana (subdomain) -------------------------------------------------------
echo "grafana (grafana.localhost):"
actual=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: grafana.localhost" http://localhost/ 2>/dev/null)
if [ "$actual" != "404" ] && [ "$actual" != "502" ] && [ "$actual" != "503" ]; then
  echo -e "  ${GREEN}✓${NC} GET grafana.localhost ($actual)"
  PASS=$((PASS + 1))
  record "GET grafana.localhost" "grafana" "pass"
else
  echo -e "  ${RED}✗${NC} GET grafana.localhost (got $actual — not routed)"
  FAIL=$((FAIL + 1))
  record "GET grafana.localhost" "grafana" "fail"
fi
echo ""

# -- frontend (priority 1) ----------------------------------------------------
echo "frontend (catchall /):"
check     "GET / (serves index.html)"     200 GET  "/"                        "GET /"                        "frontend"
echo ""

# -- Summary -------------------------------------------------------------------
TOTAL=$((PASS + FAIL))
echo "Results: $PASS/$TOTAL passed"
if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}$FAIL test(s) failed${NC}"
fi

# -- Update README.md ----------------------------------------------------------
README="README.md"
if [ -f "$README" ]; then
  DATE=$(date +%Y-%m-%d)
  TABLE="## Routing\n\n| Route | Service | Status |\n|-------|---------|--------|\n"
  for entry in "${RESULTS[@]}"; do
    route=$(echo "$entry" | cut -d'|' -f1)
    service=$(echo "$entry" | cut -d'|' -f2)
    status=$(echo "$entry" | cut -d'|' -f3)
    if [ "$status" = "pass" ]; then
      badge="![pass](https://img.shields.io/badge/-pass-brightgreen)"
    else
      badge="![fail](https://img.shields.io/badge/-fail-red)"
    fi
    TABLE+="| \`$route\` | $service | $badge |\n"
  done
  TABLE+="\n_Last updated: ${DATE}_\n"

  # Replace content between routing markers
  TMPFILE=$(mktemp)
  IN_BLOCK=false
  while IFS= read -r line; do
    if [[ "$line" == *"<!-- routing:start -->"* ]]; then
      echo "$line" >> "$TMPFILE"
      echo -e "$TABLE" >> "$TMPFILE"
      IN_BLOCK=true
      continue
    fi
    if [[ "$line" == *"<!-- routing:end -->"* ]]; then
      IN_BLOCK=false
    fi
    if [ "$IN_BLOCK" = false ]; then
      echo "$line" >> "$TMPFILE"
    fi
  done < "$README"
  mv "$TMPFILE" "$README"

  echo ""
  echo "✓ README.md routing table updated"
fi

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
