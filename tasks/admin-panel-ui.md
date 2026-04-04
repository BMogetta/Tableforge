# Admin Panel — Frontend UI for Existing Backend

## Why
The admin panel only has 2 tabs (emails + players). There are 11 admin
endpoints in user-service and 1 in game-server that have no UI. Admins
currently need to use curl to moderate.

## Prerequisite
Move all admin-related code into `frontend/src/features/admin/`. Currently
the admin feature may reference shared components or have API calls scattered.
All admin API calls, types, components, and tests must live under this feature.

## Structure after refactor
```
frontend/src/features/admin/
├── Admin.tsx              # Main page with tabs
├── Admin.module.css
├── api.ts                 # ALL admin API calls
├── components/
│   ├── AllowedEmailsTab.tsx   # existing, extract from Admin.tsx
│   ├── PlayersTab.tsx         # existing, extract from Admin.tsx
│   ├── ModerationTab.tsx      # NEW — reports + chat moderation
│   ├── BansTab.tsx            # NEW — active bans + history
│   └── RoomsTab.tsx           # NEW — active rooms + sessions
└── __tests__/
    ├── AllowedEmailsTab.test.tsx
    ├── PlayersTab.test.tsx
    ├── ModerationTab.test.tsx
    ├── BansTab.test.tsx
    └── RoomsTab.test.tsx
```

## New tabs to build

### Tab 3: Moderation
Uses existing endpoints:
- `GET /api/v1/admin/reports` — list pending reports
- `GET /api/v1/admin/players/{id}/reports` — reports about a player
- `PUT /api/v1/admin/reports/{id}/review` — resolve report (dismiss, warn, ban)

UI:
- Table of pending reports (reporter, reported player, reason, date)
- Click report → detail view with player info + report history
- Actions: dismiss, warn (close report), ban (opens ban dialog)
- Filter: pending / reviewed / all
- Badge on tab showing pending count

### Tab 4: Bans
Uses existing endpoints:
- `POST /api/v1/admin/players/{id}/ban` — issue ban (reason, expires_at)
- `DELETE /api/v1/admin/bans/{id}` — lift ban
- `GET /api/v1/admin/players/{id}/bans` — ban history for player

UI:
- Table of active bans (player, reason, issued date, expires, issued by)
- "Lift Ban" button per row
- "Ban Player" button → dialog with player ID, reason, duration (permanent/temp)
- Search by player username
- Show expired bans in a collapsed "History" section

### Tab 5: Rooms & Sessions
Uses existing endpoints:
- `GET /api/v1/rooms` — list active rooms
- `GET /api/v1/rooms/{id}` — room detail
- `GET /api/v1/players/{id}/sessions` — player's active sessions
- `DELETE /api/v1/sessions/{id}` — force-end a session (Manager role)

UI:
- Table of active rooms (game, players, status, created date)
- Click room → detail with player list
- If room has active session: "Force End" button
- Filter: waiting / in-game / finished
- Auto-refresh every 30s

## Existing tabs to extract
- Extract `AllowedEmailsTab` and `PlayersTab` from `Admin.tsx` into separate
  component files under `components/`. Admin.tsx should only render the tab
  container and route to the active tab component.

## API layer
Move all admin API calls into `features/admin/api.ts`. Currently some may be
in `frontend/src/lib/api.ts` or scattered. The admin api module should export:

```typescript
export const admin = {
  // Emails
  listEmails: () => ...,
  addEmail: (email, role) => ...,
  removeEmail: (email) => ...,
  // Players
  listPlayers: () => ...,
  setRole: (playerId, role) => ...,
  // Bans
  banPlayer: (playerId, reason?, expiresAt?) => ...,
  liftBan: (banId) => ...,
  listPlayerBans: (playerId) => ...,
  // Reports
  listReports: (status?) => ...,
  listPlayerReports: (playerId) => ...,
  reviewReport: (reportId, resolution) => ...,
  // Rooms
  listRooms: () => ...,
  getRoom: (roomId) => ...,
  forceEndSession: (sessionId) => ...,
}
```

## CSS
Fix `Admin.module.css` — it's currently incomplete (missing styles for tabs,
tables, badges, dialogs). Add styles following existing patterns (`.card`,
`.btn`, semantic tokens).

## Testing
Each component needs a Vitest test file:
- Render with mock data
- Test user interactions (click ban → dialog opens, confirm → API called)
- Test empty states
- Test error states
- Test role-based visibility (Manager vs Owner)

## Files owned
- `frontend/src/features/admin/**`
- `frontend/src/lib/api.ts` (only to remove admin calls if moved)

## Do NOT touch
- Backend code (all endpoints exist)
- Other features
- E2E tests
