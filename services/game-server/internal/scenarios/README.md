# Scenario fixtures

Debug-only game-state fixtures loaded via the ScenarioPicker devtools panel
(or the raw `POST /api/v1/sessions/{id}/debug/load-scenario` endpoint). Used
to reproduce specific states for QA/regression without playing games to
completion.

## How it works

1. Fixtures live in `data/{gameID}/{name}.json` and are embedded into the
   game-server binary via `go:embed` at build time (see `scenarios.go`).
2. The routes only register when `ALLOW_DEBUG_SCENARIOS=true` (auto-true in
   `docker-compose.override.yml`, never loaded by `make up-prod`).
3. On apply, the fixture's JSON is read, `__PLAYER_N__` placeholders are
   replaced with the session's seat-ordered player IDs, and the result is
   persisted via `runtime.LoadState` — which cancels active timers,
   broadcasts `EventMoveApplied`, and fires `MaybeFireBot` if the new
   current player is a bot.

## Fixture format

```json
{
  "name": "Human-readable title shown in the dropdown",
  "description": "One or two sentences explaining what this exercises.",
  "state": {
    "current_player_id": "__PLAYER_1__",
    "data": { /* per-game engine state — rootaccess uses defs/rootaccess_state.json */ }
  }
}
```

### Placeholders

- `__PLAYER_1__` → first seat's player ID
- `__PLAYER_2__` → second seat's player ID
- …up to the number of players in the session. Pad more if you ever need
  3+ players (the Go resolver iterates the array, no hard cap).

The placeholder MUST appear anywhere the player ID would normally appear:
`players`, `current_player_id`, keys of `hands`, `tokens`, `discard_piles`,
and element values of `protected`, `eliminated`, `discard_order`, etc.

### Validation

State bytes are parsed as a generic `engine.GameState` — enough to catch
syntax errors. Per-game schema validation (`shared/schemas/defs/...`) is not
wired yet; if a fixture has the wrong shape, the next move will surface it.
Keep fixtures honest.

## Adding a new fixture

1. Drop a JSON file under `data/{gameID}/{descriptive_name}.json`.
2. Rebuild the game-server (`docker compose up -d --build --no-deps game-server`).
   `go:embed` needs a rebuild; the picker won't see it until the binary
   re-compiles.
3. The fixture shows up automatically in the ScenarioPicker — no code change.

## Current fixtures

### rootaccess

- `firewall_standoff` — Player 1's turn vs. a firewalled player 2. Player 1
  holds Ping + Sniffer (both target-requiring with no legal target). Use to
  verify the no-op fallback (both client + server treat the move as legal
  and resolve it without a target).
- `duplicate_firewalls` — Player 1 holds two identical firewalls. Exercises
  HandDisplay's fly-out when the hand has duplicates (stale Motion-One
  inline styles on reused DOM slots).
- `near_round_win` — Round 2, player 1 is one token from winning the round.
  Fast path to RoundSummary / token-track transition UI.
