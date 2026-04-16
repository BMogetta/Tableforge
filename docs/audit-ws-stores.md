# Audit: WebSocket Implementation & Zustand Stores

**Date:** 2026-04-16

## Architecture Overview

The frontend uses **two** WebSocket classes (`lib/ws.ts`) and a **single** Zustand
store (`stores/store.ts`) for lifecycle management:

| Socket | Scope | Lifetime | Events |
|---|---|---|---|
| `PlayerSocket` | Player-global | Login to logout | queue, DMs, notifications |
| `RoomSocket` | Per-room | Join to leave | game flow, chat, presence, room changes |

Both are instantiated exclusively inside the Zustand store (`connectPlayerSocket`,
`joinRoom`). Components subscribe to events via the `socket.on()` callback pattern.

---

## What's Clean

### Single WebSocket construction (Pass)

All `new RoomSocket()` and `new PlayerSocket()` calls live in `stores/store.ts`
(lines 186 and 208). No component, hook, or other store opens its own connection.
The `new WebSocket()` calls in `lib/ws.ts` are internal to the socket classes
(reconnection). No rogue `new WebSocket()` anywhere.

### Duplicate connection guards (Pass)

Both `connectPlayerSocket` and `joinRoom` guard against duplicates:

```ts
// store.ts:183 — PlayerSocket
if (existing && existing.url === url) return

// store.ts:203 — RoomSocket
if (get().activeRoomId === roomId && get().socket) return
```

### Reconnection (Pass)

Both `RoomSocket` and `PlayerSocket` implement exponential backoff reconnection:
- Delay: `500 * 2^(attempt-1)`, capped at 30s
- Max attempts: 10
- Auth failure (4001/1008): tries token refresh before reconnecting
- `onclose` sets `this.ws = null` before scheduling `connect()`, so no two
  connections can coexist

### PlayerSocket lifecycle (Pass)

- **Created** on login: `__root.tsx:177` — `connectPlayerSocket(wsPlayerUrl(p.id))`
- **Destroyed** on logout: `__root.tsx:94` — `disconnectPlayerSocket()` when `player` becomes null
- Never closed on game end or room leave

### No `socket.send()` in components (Pass)

No component calls `socket.send()` or writes to the WebSocket directly.
All outbound communication goes through REST API calls (`sessions.move()`,
`rooms.sendMessage()`, etc.).

---

## Issues Found

### Issue 1 — Event handling is distributed across 8 components/hooks, not centralized

**Principle violated:** Event routing / No WebSocket logic in components

**Where:** Every `socket.on()` subscription outside the store:

| File | Line | Socket | Events handled |
|---|---|---|---|
| `features/room/Room.tsx` | 176 | room | ws_connected, ws_reconnecting, ws_disconnected, player_joined, player_left, game_started, owner_changed, room_closed, setting_updated, spectator_joined/left, presence_update |
| `features/game/Game.tsx` | 109 | room | ws_connected, ws_disconnected |
| `features/game/hooks/useGameSocket.ts` | 107 | room | ws_connected, move_applied, game_over, rematch_vote, rematch_ready, game_started, pause/resume events, presence_update |
| `features/game/hooks/useGameSocket.ts` | 152 | player | move_applied, game_over |
| `features/game/GameLoading.tsx` | 89 | room | player_ready, game_ready, game_over |
| `features/room/components/ChatPopover.tsx` | 83 | room | chat_message, chat_message_hidden |
| `features/lobby/components/NewGamePanel.tsx` | 62 | player | match_found, match_cancelled, match_ready, queue_left, queue_joined |
| `features/friends/components/DMConversation.tsx` | 50 | player | dm_received |
| `routes/__root.tsx` | 80 | player | notification_received, dm_received |
| `features/devtools/useWsDevtools.ts` | 22 | both | all (dev-only capture) |

There is **no central router**. Each component independently subscribes to the
raw socket and picks the events it cares about. This means:

1. Event handling logic lives inside React components, not in stores
2. Multiple components handle the same event (e.g., `presence_update` is handled
   in both `Room.tsx:201` and `useGameSocket.ts:112`)
3. Adding a new event type requires finding all `socket.on()` sites
4. Unsubscription depends on component lifecycle — if a component unmounts at
   the wrong time, events are lost

**Fix:** Create a central event router in the Zustand store that dispatches
events to registered handlers. Components consume derived state from stores
instead of subscribing to the socket directly:

```ts
// In store.ts — central onMessage handler registered once per socket
function routeRoomEvent(event: WsEvent) {
  switch (event.type) {
    case 'presence_update':
      useAppStore.getState().setPlayerPresence(event.payload.player_id, event.payload.online)
      break
    case 'chat_message':
      useChatStore.getState().handleChatMessage(event.payload)
      break
    // ... etc
  }
}
```

**Note:** This is a structural pattern preference, not a correctness bug. The
current approach works — each `socket.on()` returns an unsubscribe function and
components call it in their cleanup. The tradeoff is discoverability vs.
co-location. Centralizing improves the former; the current approach optimizes
for the latter.

**Severity: Low** — no runtime bugs, but increases maintenance cost as event
types grow.

---

### Issue 2 — `presence_update` is handled in two places with identical logic

**Principle violated:** Per-store event handlers (single entry point)

**Where:**
- `features/room/Room.tsx:201-203`
- `features/game/hooks/useGameSocket.ts:111-113`

Both do:
```ts
if (event.type === 'presence_update') {
  setPlayerPresence(event.payload.player_id, event.payload.online)
}
```

This duplication is benign (idempotent writes to the same store) but breaks the
principle that each store should have exactly one entry point for each event.

**Fix:** Handle `presence_update` in one place only — either always in the
room-level handler (Room.tsx) and remove it from useGameSocket, or centralize
per Issue 1.

**Severity: Low** — idempotent, no runtime impact.

---

### Issue 3 — `Game.tsx` subscribes to socket for connection status tracking

**Principle violated:** No WebSocket logic in components

**Where:** `features/game/Game.tsx:107-114`

```ts
useEffect(() => {
  if (!socket) return
  const off = socket.on(event => {
    if (event.type === 'ws_connected') setSocketStatus('connected')
    if (event.type === 'ws_disconnected') setSocketStatus('disconnected')
  })
  return () => off()
}, [socket])
```

`Game.tsx` subscribes to the socket to maintain a local `socketStatus` state.
This same pattern also exists in `Room.tsx:177-179`.

**Fix:** Track connection status in the Zustand store as a derived field.
Both `RoomSocket` and `PlayerSocket` already emit `ws_connected`/`ws_disconnected`
— the store can listen once and expose `connectionStatus`:

```ts
// In joinRoom:
socket.on(event => {
  if (event.type === 'ws_connected') set({ roomSocketStatus: 'connected' })
  if (event.type === 'ws_reconnecting') set({ roomSocketStatus: 'reconnecting' })
  if (event.type === 'ws_disconnected') set({ roomSocketStatus: 'disconnected' })
})
```

Components read `useAppStore(s => s.roomSocketStatus)` instead of subscribing.

**Severity: Low** — no bugs, but the same connection-tracking logic is
duplicated in Room.tsx and Game.tsx.

---

### Issue 4 — `NewGamePanel.tsx` calls `connectRoomSocket` directly from event handler

**Principle violated:** No WebSocket logic in components

**Where:** `features/lobby/components/NewGamePanel.tsx:76`

```ts
if (event.type === 'match_ready') {
  const payload = event.payload
  clearQueue()
  connectRoomSocket(payload.room_id, wsRoomUrl(payload.room_id, player.id))
  navigate(...)
}
```

A React component is opening a new WebSocket connection (via `joinRoom`/`connectRoomSocket`)
in response to a socket event it handles locally. This mixes transport orchestration
with UI component logic.

**Fix:** Move queue event handling to the store or a dedicated hook. The `match_ready`
handler should live alongside the other queue state (`setQueued`, `setMatchFound`,
`clearQueue`):

```ts
// In store.ts, when registering playerSocket events:
if (event.type === 'match_ready') {
  get().clearQueue()
  get().joinRoom(payload.room_id, wsRoomUrl(payload.room_id, playerId))
  // navigation is the only part that belongs in the component
}
```

**Severity: Medium** — the component is making architectural decisions (when to
open a socket) that should live in the store.

---

### Issue 5 — `RoomSocket` is closed on game end via `leaveRoom()`

**Principle violated:** Lifecycle correctness

**Where:**
- `features/game/Game.tsx:187` — surrender callback: `leaveRoom()` + navigate to `/`
- `features/game/Game.tsx:200` — back-to-queue: `leaveRoom()` + navigate to `/`
- `features/game/Game.tsx:211` — back-to-lobby: `leaveRoom()` + navigate to `/`
- `features/game/Game.tsx:216` — view-replay: `leaveRoom()` + navigate to history

All four post-game actions call `leaveRoom()` which closes the `RoomSocket`.
This is correct for navigation away from the room, but it means:

1. If a rematch starts (via `useRematch`), the socket is still alive (good)
2. Once the user clicks any "done" action, the socket closes (expected)

**Assessment:** This is actually **correct behavior**. The socket lives for the
duration of the room-based interaction (Room + Game screens). Closing it when
the user navigates away is the intended lifecycle. The principle "must not close
when game ends" would only be violated if the socket closed automatically at
game-over without user action.

**No fix needed.**

---

### Issue 6 — `ChatPopover` handles chat events locally instead of through a store

**Principle violated:** Per-store event handlers / Event routing

**Where:** `features/room/components/ChatPopover.tsx:83-106`

The `ChatPopover` component subscribes to the room socket and directly mutates
the React Query cache when `chat_message` or `chat_message_hidden` arrives.
There is no chat Zustand store — the component owns the event handling.

This means chat events are only processed while the `ChatPopover` component is
mounted. If the popover is closed when a message arrives, the message won't
appear in the cache until the next fetch/refetch.

**Fix:** If chat message loss matters, create a `useChatSocket` hook (or store)
that mounts at the Room level and always processes chat events. The `ChatPopover`
then reads from the React Query cache without subscribing to the socket.

**Severity: Low** — the 30-second refetch interval (`refetchInterval: 30_000`)
acts as a safety net. Messages are never truly lost, just delayed.

---

## Summary Table

| # | Issue | Principle | Severity | File:Line |
|---|---|---|---|---|
| 1 | Event handling distributed across 8+ files | Event routing | Low | Multiple (see table) |
| 2 | `presence_update` handled in two places | Per-store handlers | Low | Room.tsx:201, useGameSocket.ts:112 |
| 3 | Game.tsx subscribes to socket for status | No WS logic in components | Low | Game.tsx:107 |
| 4 | NewGamePanel opens socket from event handler | No WS logic in components | Medium | NewGamePanel.tsx:76 |
| 5 | Socket closed on game end (user-initiated) | Lifecycle correctness | N/A | Game.tsx:187,200,211,216 |
| 6 | ChatPopover owns chat event handling | Per-store handlers | Low | ChatPopover.tsx:83 |

## Overall Assessment

The WebSocket implementation is **solid**:
- Singleton construction in the store with duplicate guards
- Robust reconnection with exponential backoff and auth refresh
- Clean separation between PlayerSocket (login-scoped) and RoomSocket (room-scoped)
- No direct `socket.send()` in components
- Correct lifecycle (PlayerSocket: login/logout, RoomSocket: room entry/exit)

The main structural gap is the lack of a **central event router** — events are
handled at the point of consumption (in components and hooks) rather than
dispatched from a single place. This is a deliberate co-location tradeoff, not
a bug. The only actionable issue is **Issue 4** (NewGamePanel opening a socket
from within a component-level event handler), which should be moved to the store.

## Priority

1. **Issue 4** — Move `match_ready` socket-opening logic out of NewGamePanel
2. **Issue 3** — Track connection status in the store (removes duplicate logic)
3. **Issue 1** — Central router (architectural improvement, tackle when event count grows)
4. **Issue 2/6** — Minor dedup (do alongside Issue 1 if pursued)
