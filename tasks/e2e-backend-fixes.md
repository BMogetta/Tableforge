# E2E Backend Fixes (3 bugs blocking tests)

## Bug 1: Frontend sends POST without body, backend rejects with EOF

### Problem
Several endpoints (`/rooms/{id}/start`, `/sessions/{id}/surrender`,
`/sessions/{id}/rematch`, `/rooms/{id}/leave`) decode an empty struct from
the request body. When the frontend sends POST without body, `json.Decode`
fails with EOF.

### Fix (frontend)
In each API call that sends POST without a body, add `body: JSON.stringify({})`:

**Files:**
- `frontend/src/features/room/api.ts` — `rooms.start()`, `rooms.leave()`
- `frontend/src/lib/api/sessions.ts` — `sessions.surrender()`, `sessions.rematch()`

Find all POST/DELETE calls that don't send a body and add:
```typescript
body: JSON.stringify({}),
headers: { 'Content-Type': 'application/json' },
```

### After fix
Remove `patchEmptyBodyRoutes()` workaround from `frontend/tests/e2e/helpers.ts`.

---

## Bug 3: Spectators stuck on GameLoading screen

### Problem
When a spectator navigates to `/game/:sessionId`, the `GameLoading` component
sends `POST /sessions/{id}/ready` and waits for `game_ready` WS event.
Spectators don't participate in the ready-check — they should bypass it.

### Fix
**File:** `frontend/src/features/game/GameLoading.tsx`

At the top of the `useEffect` that handles asset loading + ready call, check
if the user is a spectator. If so, skip the ready flow and call `onReady()`
immediately after assets load:

```typescript
// After assets load and MIN_LOADING_MS passes:
if (isSpectator) {
  setPhase('done')
  onReady()
  return
}
// ... existing ready call for participants
```

The `isSpectator` flag is available from `useAppStore` (check how `Game.tsx`
accesses it).

---

## Bug 4: Presence offline not broadcast on disconnect

### Problem
When a player's WS disconnects, the `presence_update` with `online: false` is
supposed to be published. The code exists at `handler.go:151-158` but uses
the HTTP request `ctx` which is already cancelled by the time `ReadPump()` ends.

### Fix
**File:** `services/ws-gateway/internal/handler/handler.go`

Change line ~151 from:
```go
h.Publish(ctx, roomID, hub.Event{
```
to:
```go
h.Publish(context.Background(), roomID, hub.Event{
```

This is the same pattern used elsewhere in the codebase (e.g., `hub.Broadcast`
uses `context.Background()`).

---

## Testing
```bash
go test ./services/ws-gateway/...
cd frontend && npx vitest run && npx tsc --noEmit
```
Then run e2e: `make up-test && make test`
