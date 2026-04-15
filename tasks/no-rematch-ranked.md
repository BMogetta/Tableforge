# Remove rematch in ranked matches

## Why
Ranked matches currently offer a "Rematch" button on the post-match screen,
which shouldn't be possible — ranked play must route players back through
matchmaking so seeding is fresh and MMR pairs are recomputed. Casual is
unaffected and keeps rematch as-is.

Secondary gap: today there is **no "Back to queue" action** anywhere in the
post-match UI, so a ranked player has to navigate home and re-queue manually.

## What to change

### 1. Frontend — hide rematch when ranked
- `frontend/src/features/game/components/GameOverActions.tsx` — add an
  `isRanked: boolean` prop to `Props` and wrap the rematch button (currently
  around lines 52–65) in a `!isRanked` check.
- `frontend/src/features/game/Game.tsx` — at the `<GameOverActions ...>` call
  (around lines 323–334), pass `isRanked={session.mode === 'ranked'}`.
  `session.mode` is already on the state and typed via `schema-generated.zod.ts`
  (`gameSessionSchema.mode`).

### 2. Backend — reject rematch on ranked (belt-and-suspenders)
- `services/game-server/internal/platform/api/api_session.go` `handleRematch`
  (lines 282–350) — after loading the session, if `session.Mode ==
  store.SessionModeRanked` return `403 forbidden` with a clear error code
  (e.g. `rematch_not_allowed_ranked`). Prevents a crafted client or replayed
  request from bypassing the UI guard.
- While touching this handler, also fix the latent bug at line 308: the
  auto-start path hardcodes `store.SessionModeCasual` instead of reusing
  `session.Mode`. Once ranked is rejected at the top of the handler this
  becomes moot, but leaving the hardcode is a trap for future changes —
  replace with `session.Mode` (which will always be `Casual` here by then).

### 3. Tests
- `frontend/src/features/game/hooks/__tests__/useRematch.test.ts` — add a
  case asserting `voteRematch` is not callable / the hook short-circuits when
  the session is ranked (or, if the hook itself is mode-agnostic, cover the
  UI hiding in a `GameOverActions` component test).
- `frontend/tests/e2e/game.spec.ts` — the casual rematch flow test already
  exists around line 57; add a sibling test using a ranked session that
  asserts the rematch button is absent after game-over.
- `services/game-server/internal/platform/api/api_session_test.go` (or
  equivalent) — cover the 403 path for `POST /sessions/{id}/rematch` when
  `session.Mode == SessionModeRanked`.

## Optional follow-up — "Back to queue" for ranked

If we want ranked to actually close the loop (not just send the player to
the lobby), add a second button:

- New `Back to queue` button in `GameOverActions.tsx`, visible only when
  `isRanked`. On click, re-enqueue the player to the ranked queue by reusing
  whatever the lobby/matchmaking screen already calls. Need a short
  exploration pass in `frontend/src/features/lobby/` and `services/match-service`
  to identify the exact enqueue endpoint/store action before estimating.
- If we don't build this now, the minimum acceptable UX is: ranked players
  see only `Back to Lobby` post-match and re-queue from there. Document this
  decision in the PR.

## Out of scope
- Changes to MMR/rating logic — rematch removal does not affect rating math.
- Spectator flow — spectators never see rematch (`!isSpectator` guard
  already in place).
- Casual rematch behavior — untouched.

## Effort
Low. ~5–8 lines of product code + 2–3 small tests. The optional
"Back to queue" adds a half-day including wiring + e2e.
