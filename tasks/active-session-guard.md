# Backend Enforcement: Active Session Guard

## Why
A player with an active game session can currently create or join another room
via the API. The frontend blocks this with the "active game" banner, but the
backend doesn't enforce it. A direct API call bypasses the check.

## What to change

### `services/game-server/internal/platform/api/api_lobby.go`
In `handleCreateRoom` and `handleJoinRoom`:
1. After extracting `callerID` from JWT context
2. Call `st.ListActiveSessions(ctx, callerID)`
3. If len > 0, return 409 Conflict with `{"error": "active session exists", "session_id": "..."}`

### `services/game-server/internal/platform/store/`
Verify `ListActiveSessions` exists and returns sessions where `finished_at IS NULL`.

### Frontend
- Already handled by the "active game" banner in Lobby.tsx
- No frontend changes needed, but the API error should be handled gracefully
  (show a toast instead of a generic error)

## Testing
- `go test ./services/game-server/...`
- Add a test case: create room → start game → try to create another room → expect 409
