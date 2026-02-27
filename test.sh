# 1. Create players
ALICE=$(curl -s -X POST http://localhost:8080/api/v1/players \
  -H "Content-Type: application/json" \
  -d '{"username":"alice"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Alice: $ALICE"

BOB=$(curl -s -X POST http://localhost:8080/api/v1/players \
  -H "Content-Type: application/json" \
  -d '{"username":"bob"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Bob: $BOB"

# 2. Alice creates a room
ROOM=$(curl -s -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d "{\"game_id\":\"tictactoe\",\"player_id\":\"$ALICE\"}")
echo "Room: $ROOM"
ROOM_ID=$(echo $ROOM | python3 -c "import sys,json; print(json.load(sys.stdin)['room']['id'])")
ROOM_CODE=$(echo $ROOM | python3 -c "import sys,json; print(json.load(sys.stdin)['room']['code'])")
echo "Room ID: $ROOM_ID, Code: $ROOM_CODE"

# 3. Bob joins
curl -s -X POST http://localhost:8080/api/v1/rooms/join \
  -H "Content-Type: application/json" \
  -d "{\"code\":\"$ROOM_CODE\",\"player_id\":\"$BOB\"}" | python3 -m json.tool

# 4. Alice starts the game
SESSION=$(curl -s -X POST http://localhost:8080/api/v1/rooms/$ROOM_ID/start \
  -H "Content-Type: application/json" \
  -d "{\"player_id\":\"$ALICE\"}")
echo "Session: $SESSION"
SESSION_ID=$(echo $SESSION | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Session ID: $SESSION_ID"

# 5. Check state
curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID | python3 -m json.tool

# 6. Alice plays center (cell 4)
curl -s -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/move \
  -H "Content-Type: application/json" \
  -d "{\"player_id\":\"$ALICE\",\"payload\":{\"cell\":4}}" | python3 -m json.tool

# 7. Bob plays top-left (cell 0)
curl -s -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/move \
  -H "Content-Type: application/json" \
  -d "{\"player_id\":\"$BOB\",\"payload\":{\"cell\":0}}" | python3 -m json.tool

# 8. Check state again
curl -s http://localhost:8080/api/v1/sessions/$SESSION_ID | python3 -m json.tool

# --- WebSocket test ---
# Requires: go install github.com/vi/websocat@latest

echo ""
echo "=== WebSocket test ==="
echo "Connecting to ws://localhost:8080/ws/rooms/$ROOM_ID"
echo "Will receive events for moves made in this room"
echo ""

# Listen for 5 seconds in background, capture output
websocat --no-close ws://localhost:8080/ws/rooms/$ROOM_ID &
WS_PID=$!

sleep 1.5

# Make a move — should appear in websocat output
echo ">>> Alice plays cell 2"
curl -s -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/move \
  -H "Content-Type: application/json" \
  -d "{\"player_id\":\"$ALICE\",\"payload\":{\"cell\":2}}" | python3 -m json.tool

sleep 0.3

echo ">>> Bob plays cell 6"
curl -s -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/move \
  -H "Content-Type: application/json" \
  -d "{\"player_id\":\"$BOB\",\"payload\":{\"cell\":6}}" | python3 -m json.tool

sleep 1
kill $WS_PID 2>/dev/null
wait $WS_PID 2>/dev/null
echo "=== WebSocket test done ==="