# Pagination on List Endpoints

## Why
List endpoints return all records in a single response. The `/rooms` endpoint
already returns 125KB+ during test runs. Without pagination, responses grow
unbounded and slow down the lobby.

## Endpoints to paginate

| Endpoint | Service | Current response |
|----------|---------|-----------------|
| `GET /api/v1/rooms` | game-server | All rooms (125KB+) |
| `GET /api/v1/players/{id}/notifications` | notification-service | All notifications |
| `GET /api/v1/players/{id}/matches` | game-server | Already paginated ✓ |
| `GET /api/v1/players/{id}/sessions` | game-server | All active sessions (small) |
| `GET /api/v1/rooms/{id}/messages` | chat-service | All messages |

## Pattern
Use `?limit=N&offset=M` query params. Return response with:
```json
{
  "items": [...],
  "total": 142,
  "limit": 20,
  "offset": 0
}
```

### Backend (per endpoint)
1. Add `limit` and `offset` params to handler (default limit=20, max=100)
2. Add `COUNT(*)` query for total
3. Add `LIMIT $N OFFSET $M` to the SQL query
4. Update JSON Schema response if one exists

### Frontend
- Add infinite scroll or "Load more" button to room list
- Add pagination to notification panel
- Add pagination to chat message history (load older messages on scroll up)

## Files per service
- game-server: `api_lobby.go` (rooms), store query
- notification-service: handler + store
- chat-service: `api_rooms.go` (messages), store query
- Frontend: `Lobby.tsx`, `NotificationsPanel.tsx`, `ChatPopover.tsx`
- JSON Schemas: update response schemas if they exist

## Testing
- Go tests for each modified endpoint
- Frontend: verify infinite scroll / load more works
