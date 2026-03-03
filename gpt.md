TableForge Frontend – Technical Audit Report
Overview

This document summarizes architectural, security, lifecycle, and scalability issues identified during a code review of the Vite + React frontend.

The goal is not to criticize working code, but to document areas that:

Increase long-term complexity

Introduce security risks

Create architectural inconsistencies

Could cause scaling or maintainability issues

1. Authentication State Outside React Query
Problem

auth.me() is called manually inside App.tsx using useEffect, instead of React Query.

This creates:

No caching

No retry control

No invalidation support

No integration with telemetry tracing

A second state system (Zustand + React Query)

Impact

Two sources of truth:

player in Zustand

All other server state in React Query

This can lead to inconsistent UI behavior and harder debugging.

Recommendation

Move auth.me() into React Query and make it the canonical source of authenticated user state.

2. Security Risk – Client-Supplied player_id
Problem

Multiple API calls send player_id from the frontend:

{
  "player_id": "<value-from-frontend>"
}

Examples:

rooms.create

rooms.join

sessions.move

sessions.surrender

sessions.rematch

Risk

If the backend does not strictly validate identity against the session cookie:

A user could modify player.id in DevTools

Impersonate another user

Submit moves or actions on behalf of others

Severity

🔥 High (if backend trusts the field)

Recommendation

The backend should derive the player identity exclusively from:

Session cookie

JWT

Server-side auth context

The frontend should not send player_id at all.

3. WebSocket Lifecycle Coupled to Navigation Flow
Problem

The WebSocket is created when joining a room and assumed to persist into the Game page.

However:

Navigating directly to /game/:sessionId does not guarantee a socket exists.

Game depends on Room having created the socket.

Impact

Hidden coupling between routes:

Room → Game dependency is implicit, not declarative.

This breaks:

Direct navigation

Page refresh reliability

Clear lifecycle ownership

Recommendation

Refactor WebSocket lifecycle to depend on roomId or sessionId, not on navigation order.

Possible solutions:

useRoomSocket(roomId) hook

Context-based socket provider

Dedicated socket manager

4. Infinite WebSocket Reconnection Loop
Problem

After 3 failed reconnect attempts:

ws_disconnected is emitted

But reconnection continues indefinitely every 2 seconds

There is:

No backoff

No max retry cap

No terminal state

Impact

Backend saturation under outage

Battery drain (mobile)

Infinite retry loops

Recommendation

Implement:

Exponential backoff

Max retry threshold

Final hard disconnect state

5. WebSocket Payload Not Runtime-Validated
Problem

Incoming messages are parsed but not validated:

const event: WsEvent = JSON.parse(e.data)

No runtime verification that:

type is valid

payload matches expected structure

Impact

If backend is compromised or misconfigured:

Malformed data may propagate

Undefined behavior possible

Recommendation

Introduce runtime validation (e.g., Zod schemas or type guards).

6. Duplicate React Query Fetch in Lobby
Problem

gameRegistry.list is queried twice with the same query key.

This causes:

Duplicate fetches

Redundant logic

Unnecessary complexity

Recommendation

Use a single useQuery and derive state from it.

7. Polling + WebSocket Redundancy
Problem

Game page uses:

WebSocket real-time updates

Polling every 3 seconds simultaneously

Polling continues even when WebSocket is healthy.

Impact

Unnecessary network load

Duplicate updates

Increased backend pressure

Recommendation

Disable polling while WebSocket is connected.
Enable polling only as fallback during WS disconnect.

8. Hardcoded API Base and Mixed Routing
Problem

BASE = '/api/v1' is hardcoded.

Some routes use:

/api/v1/...

/auth/...

Inconsistent request construction.

Recommendation

Centralize API routing strategy and allow environment-based configuration.

9. Error Handling Gaps
Problem

React render-phase errors are logged only to console.error, while telemetry already exists.

Impact

ErrorBoundary errors may not be captured in OTLP logs consistently.

Recommendation

Emit structured logs via the OpenTelemetry logger in componentDidCatch.

10. GameRenderer Scalability Limitation
Problem

Game rendering is handled by:

switch (gameId)

This does not scale cleanly as more games are added.

Recommendation

Use a registry pattern:

const gameRenderers: Record<string, React.FC<...>>

Enables pluggable game architecture.

11. Missing CSRF Visibility
Problem

All requests use:

credentials: 'include'

There is no visible CSRF mitigation in frontend.

Risk

If backend does not enforce:

SameSite=strict/lax

CSRF token validation

The system may be vulnerable.

Recommendation

Ensure backend enforces CSRF protection.
Frontend should support token injection if required.

12. Minor Type Safety Degradations

Examples:

(data as any).result

Unchecked unknown casts

These reduce long-term maintainability.

Overall Assessment
Strengths

Strong modular separation (API / WS / state)

Proper React Query usage

Good OpenTelemetry integration

Clean TypeScript modeling

Solid UI structure

Main Risks

🔥 Potential identity spoofing via player_id

⚠ WebSocket lifecycle design fragility

⚠ Network redundancy (polling + WS)

⚠ Infinite reconnect loop
