# Friends-Only DMs Setting

## Why
The "Allow DMs from" setting shows "Friends only" as "(coming soon)" in the
Settings UI. Users should be able to restrict who can send them DMs.

## What to change

### Frontend
- `frontend/src/ui/Settings.tsx` — enable the "Friends only" option (remove
  "coming soon" label and disabled state)

### Backend (chat-service)
- `services/chat-service/internal/api/api_dms.go` — in the send DM handler,
  before creating the message:
  1. Fetch the receiver's settings (`allow_dms` field)
  2. If `allow_dms == "friends_only"`, check if sender is in receiver's friend
     list via user-service gRPC or a direct DB query
  3. If not a friend, return 403 `{"error": "player only accepts DMs from friends"}`

### Dependencies
- chat-service needs to query user-service for friendship status
- Check if a gRPC method exists for this, or if a direct DB query is feasible
  (chat-service may not have access to the users schema)

## Testing
- Go test: send DM to player with friends-only → expect 403 if not friends
- Frontend: verify Settings toggle works
