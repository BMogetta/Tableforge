#!/usr/bin/env bash
# smoke-test.sh — fast API-level validation of core game flows.
#
# Tests the same flows as Playwright e2e but with curl (~5s vs ~5min).
# Useful for validating backend fixes before running full e2e suite.
#
# Requires: make up-test running + at least 2 seeded players in .players.json.
# Usage: make smoke-test
set -euo pipefail

BASE="http://localhost"
PLAYERS_FILE="frontend/tests/e2e/.players.json"
PASS=0
FAIL=0
TOTAL=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
DIM='\033[2m'
NC='\033[0m'

# --- Helpers -----------------------------------------------------------------

die() { echo -e "${RED}FATAL:${NC} $1" >&2; exit 1; }

check_deps() {
  command -v curl >/dev/null || die "curl not found"
  command -v jq >/dev/null || die "jq not found"
  [[ -f "$PLAYERS_FILE" ]] || die "$PLAYERS_FILE not found — run: make seed-test"
}

# Authenticate a player, return the JWT cookie value.
auth_player() {
  local player_id="$1"
  local headers
  headers=$(curl -s -D - -o /dev/null "$BASE/auth/test-login?player_id=$player_id")
  local token
  token=$(echo "$headers" | sed -n 's/.*tf_session=\([^;]*\).*/\1/p' | tr -d '\r')
  echo "$token"
}

# Make an authenticated request. Usage: api TOKEN METHOD /path [body]
api() {
  local token="$1" method="$2" path="$3" body="${4:-}"
  local args=(-s -w '\n%{http_code}' -X "$method" -b "tf_session=$token" -H 'Content-Type: application/json')
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" "${BASE}${path}"
}

# Parse response: last line is status code, rest is body.
parse_response() {
  local response="$1"
  HTTP_CODE=$(echo "$response" | tail -1)
  HTTP_BODY=$(echo "$response" | sed '$d')
}

# Assert HTTP status. Usage: assert "test name" expected_code actual_code
assert_status() {
  local name="$1" expected="$2" actual="$3"
  TOTAL=$((TOTAL + 1))
  if [[ "$actual" == "$expected" ]]; then
    echo -e "  ${GREEN}PASS${NC} $name ${DIM}($actual)${NC}"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC} $name — expected $expected, got $actual"
    [[ -n "${HTTP_BODY:-}" ]] && echo -e "       ${DIM}${HTTP_BODY:0:200}${NC}"
    FAIL=$((FAIL + 1))
  fi
}

# Assert body contains string. Usage: assert_contains "test name" needle body
assert_contains() {
  local name="$1" needle="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  if echo "$body" | grep -q "$needle"; then
    echo -e "  ${GREEN}PASS${NC} $name"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC} $name — body missing '$needle'"
    echo -e "       ${DIM}${body:0:200}${NC}"
    FAIL=$((FAIL + 1))
  fi
}

# --- Setup -------------------------------------------------------------------

check_deps

P1_ID=$(jq -r '.player1_id' "$PLAYERS_FILE")
P2_ID=$(jq -r '.player2_id' "$PLAYERS_FILE")
[[ -z "$P1_ID" || "$P1_ID" == "null" ]] && die "No player1_id in $PLAYERS_FILE"
[[ -z "$P2_ID" || "$P2_ID" == "null" ]] && die "No player2_id in $PLAYERS_FILE"

echo -e "${YELLOW}Authenticating players...${NC}"
T1=$(auth_player "$P1_ID")
T2=$(auth_player "$P2_ID")
[[ -z "$T1" ]] && die "Failed to authenticate P1 ($P1_ID)"
[[ -z "$T2" ]] && die "Failed to authenticate P2 ($P2_ID)"
echo -e "  P1: ${DIM}${P1_ID:0:8}...${NC}"
echo -e "  P2: ${DIM}${P2_ID:0:8}...${NC}"

# --- Cleanup: leave any existing rooms/sessions -----------------------------

echo ""
echo -e "${YELLOW}Cleaning up stale state...${NC}"
for TOKEN in "$T1" "$T2"; do
  PID=$( [[ "$TOKEN" == "$T1" ]] && echo "$P1_ID" || echo "$P2_ID" )
  SESSIONS=$(api "$TOKEN" GET "/api/v1/players/$PID/sessions" | sed '$d')
  for SID in $(echo "$SESSIONS" | jq -r '.[].id' 2>/dev/null); do
    api "$TOKEN" POST "/api/v1/sessions/$SID/surrender" '{}' >/dev/null
  done
  ROOMS=$(api "$TOKEN" GET "/api/v1/rooms" | sed '$d')
  for RID in $(echo "$ROOMS" | jq -r ".items[]? | select(.players[]?.id == \"$PID\") | .room.id" 2>/dev/null); do
    api "$TOKEN" POST "/api/v1/rooms/$RID/leave" '{}' >/dev/null
  done
done
echo -e "  ${DIM}done${NC}"

# --- Test 1: Auth ------------------------------------------------------------

echo ""
echo -e "${YELLOW}1. Auth${NC}"
parse_response "$(api "$T1" GET "/auth/me")"
assert_status "GET /auth/me returns 200" 200 "$HTTP_CODE"
assert_contains "response has player_id" "$P1_ID" "$HTTP_BODY"

# --- Test 2: Room lifecycle --------------------------------------------------

echo ""
echo -e "${YELLOW}2. Room lifecycle${NC}"

# Create room
parse_response "$(api "$T1" POST "/api/v1/rooms" '{"game_id":"tictactoe"}')"
assert_status "POST /rooms creates room" 201 "$HTTP_CODE"
ROOM_ID=$(echo "$HTTP_BODY" | jq -r '.room.id // .id')
ROOM_CODE=$(echo "$HTTP_BODY" | jq -r '.room.code // .code')
[[ -z "$ROOM_ID" || "$ROOM_ID" == "null" ]] && die "Room creation failed: $HTTP_BODY"

# Set first_mover_policy to fixed for deterministic play
parse_response "$(api "$T1" PUT "/api/v1/rooms/$ROOM_ID/settings/first_mover_policy" \
  "{\"player_id\":\"$P1_ID\",\"value\":\"fixed\"}")"
assert_status "PUT settings/first_mover_policy" 204 "$HTTP_CODE"

# Join room (route is /rooms/join, not /rooms/:id/join)
parse_response "$(api "$T2" POST "/api/v1/rooms/join" "{\"code\":\"$ROOM_CODE\"}")"
assert_status "POST /rooms/join" 200 "$HTTP_CODE"

# List rooms (pagination)
parse_response "$(api "$T1" GET "/api/v1/rooms?limit=5")"
assert_status "GET /rooms?limit=5" 200 "$HTTP_CODE"

# Start game
parse_response "$(api "$T1" POST "/api/v1/rooms/$ROOM_ID/start" '{}')"
assert_status "POST /rooms/:id/start" 200 "$HTTP_CODE"
SESSION_ID=$(echo "$HTTP_BODY" | jq -r '.id')

# --- Test 3: Ready check (empty body POST) ----------------------------------

echo ""
echo -e "${YELLOW}3. Ready check (empty body regression)${NC}"

# Both players ready
parse_response "$(api "$T1" POST "/api/v1/sessions/${SESSION_ID}/ready" '{}')"
assert_status "POST /sessions/:id/ready (P1)" 200 "$HTTP_CODE"
parse_response "$(api "$T2" POST "/api/v1/sessions/${SESSION_ID}/ready" '{}')"
assert_status "POST /sessions/:id/ready (P2)" 200 "$HTTP_CODE"

# --- Test 4: Game moves -----------------------------------------------------

echo ""
echo -e "${YELLOW}4. Game moves (TicTacToe)${NC}"

# Small delay for session to be fully started
sleep 1

# P1 move (cell 0)
parse_response "$(api "$T1" POST "/api/v1/sessions/$SESSION_ID/move" '{"payload":{"cell":0}}')"
assert_status "P1 plays cell 0" 200 "$HTTP_CODE"

# P2 move (cell 3)
parse_response "$(api "$T2" POST "/api/v1/sessions/$SESSION_ID/move" '{"payload":{"cell":3}}')"
assert_status "P2 plays cell 3" 200 "$HTTP_CODE"

# P1 move (cell 1)
parse_response "$(api "$T1" POST "/api/v1/sessions/$SESSION_ID/move" '{"payload":{"cell":1}}')"
assert_status "P1 plays cell 1" 200 "$HTTP_CODE"

# P2 move (cell 4)
parse_response "$(api "$T2" POST "/api/v1/sessions/$SESSION_ID/move" '{"payload":{"cell":4}}')"
assert_status "P2 plays cell 4" 200 "$HTTP_CODE"

# P1 move (cell 2) — winning move
parse_response "$(api "$T1" POST "/api/v1/sessions/$SESSION_ID/move" '{"payload":{"cell":2}}')"
assert_status "P1 plays cell 2 (win)" 200 "$HTTP_CODE"

# --- Test 5: Session history -------------------------------------------------

echo ""
echo -e "${YELLOW}5. Session history${NC}"
sleep 1

parse_response "$(api "$T1" GET "/api/v1/sessions/$SESSION_ID")"
assert_status "GET /sessions/:id" 200 "$HTTP_CODE"
assert_contains "session is finished" "finished" "$HTTP_BODY"

parse_response "$(api "$T1" GET "/api/v1/sessions/$SESSION_ID/history")"
assert_status "GET /sessions/:id/history" 200 "$HTTP_CODE"
MOVE_COUNT=$(echo "$HTTP_BODY" | jq 'length' 2>/dev/null || echo 0)
TOTAL=$((TOTAL + 1))
if [[ "$MOVE_COUNT" == "5" ]]; then
  echo -e "  ${GREEN}PASS${NC} move count is 5"
  PASS=$((PASS + 1))
else
  echo -e "  ${RED}FAIL${NC} expected 5 moves, got $MOVE_COUNT"
  FAIL=$((FAIL + 1))
fi

# --- Test 6: Room leave (on a fresh waiting room) --------------------------

echo ""
echo -e "${YELLOW}6. Room leave (empty body regression)${NC}"
# Create a separate room to test leave (the game room is already finished)
parse_response "$(api "$T1" POST "/api/v1/rooms" '{"game_id":"tictactoe"}')"
LEAVE_ROOM_ID=$(echo "$HTTP_BODY" | jq -r '.room.id // .id')
parse_response "$(api "$T1" POST "/api/v1/rooms/$LEAVE_ROOM_ID/leave" '{}')"
assert_status "POST /rooms/:id/leave" 204 "$HTTP_CODE"

# --- Test 7: Friends ---------------------------------------------------------

echo ""
echo -e "${YELLOW}7. Friends${NC}"
# Route is /players/:id/friends/:targetId (target in URL, not body)
parse_response "$(api "$T1" POST "/api/v1/players/$P1_ID/friends/$P2_ID" '{}')"
assert_status "POST friend request" 201 "$HTTP_CODE"

parse_response "$(api "$T2" GET "/api/v1/players/$P2_ID/friends/pending")"
assert_status "GET pending requests" 200 "$HTTP_CODE"

# Accept — route is PUT /players/:id/friends/:requesterId/accept
parse_response "$(api "$T2" PUT "/api/v1/players/$P2_ID/friends/$P1_ID/accept" '{}')"
assert_status "PUT accept friend request" 200 "$HTTP_CODE"

# Remove friend (cleanup)
parse_response "$(api "$T1" DELETE "/api/v1/players/$P1_ID/friends/$P2_ID" '{}')"
assert_status "DELETE remove friend" 204 "$HTTP_CODE"

# --- Test 8: Lobby duplicate prevention --------------------------------------

echo ""
echo -e "${YELLOW}8. Lobby duplicate prevention${NC}"

# Create room
parse_response "$(api "$T1" POST "/api/v1/rooms" '{"game_id":"tictactoe"}')"
assert_status "create room for dup test" 201 "$HTTP_CODE"
DUP_ROOM_ID=$(echo "$HTTP_BODY" | jq -r '.room.id // .id')

# Try creating another room while in the first
parse_response "$(api "$T1" POST "/api/v1/rooms" '{"game_id":"tictactoe"}')"
assert_status "duplicate room rejected (409 or 4xx)" 409 "$HTTP_CODE"

# Cleanup
api "$T1" POST "/api/v1/rooms/$DUP_ROOM_ID/leave" '{}' >/dev/null

# --- Summary -----------------------------------------------------------------

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ $FAIL -eq 0 ]]; then
  echo -e "${GREEN}All $TOTAL tests passed${NC}"
else
  echo -e "${RED}$FAIL/$TOTAL failed${NC}, ${GREEN}$PASS passed${NC}"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

exit $FAIL
