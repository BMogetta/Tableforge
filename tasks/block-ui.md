# Block Player UI

## Why
Backend supports blocking players but there's no frontend UI. Users can't
block abusive players from the app.

## What to change

### Frontend
- Add "Block" option to player context menus (profile page, friend list, DM panel)
- Use existing APIs:
  - `POST /api/v1/players/{id}/block/{targetId}` — block
  - `DELETE /api/v1/players/{id}/block/{targetId}` — unblock
- Add "Blocked Players" section in Settings with unblock buttons
- Blocked players should be hidden from:
  - Room list (rooms they created)
  - DM inbox
  - Friend suggestions

### Backend
- Verify block endpoints work correctly (they exist in user-service)
- Check if room list / DM queries already filter blocked players
  - If not, add filtering in the relevant store queries

## Testing
- Frontend component tests for block/unblock flow
- Manual: block a player → verify they disappear from relevant lists
