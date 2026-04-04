# Bot UI in Room

## Why
Backend supports adding/removing bots to rooms, but the Room UI doesn't expose
this. Players can only play against other humans or use the API directly.

## What to change

### Frontend only
- `frontend/src/features/room/Room.tsx` — add a "Add Bot" button in the player
  list area (only visible to room owner when room has open slots)
- Use existing `bots.list()` API to show available bot profiles
- Use existing `bots.add(roomId, botId)` and `bots.remove(roomId, botId)` APIs
- Bot players should show a 🤖 badge in the player list
- Check `frontend/src/features/room/api.ts` — the API functions already exist

### No backend changes needed
The endpoints `POST /api/v1/rooms/{id}/bots`, `DELETE /api/v1/rooms/{id}/bots/{botId}`,
and `GET /api/v1/bots/profiles` are all implemented and tested.

## Testing
- `npm test` — add component test for bot add/remove
- Manual: create room → add bot → start game → bot should play automatically
