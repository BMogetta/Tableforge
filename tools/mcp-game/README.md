# recess-mcp-game

Local MCP server that exposes recess game-server operations as tools for
Claude Code. Lets Claude apply scenario fixtures, load arbitrary game states,
submit moves, and read current state — all against the local dev stack.

## Prereqs

- Dev stack running with `TEST_MODE=true` (e.g. `make up-test`):
  - `auth-service` exposes `/auth/test-login`.
  - `game-server` has `ALLOW_DEBUG_SCENARIOS=true` (automatic under the dev
    override).
- `make seed-test` has been run at least once — the MCP auto-reads
  `frontend/tests/e2e/.players.json` to find a player UUID to authenticate as.
- Node 20+ (the MCP SDK needs it).

## Install

```bash
cd tools/mcp-game
npm install
```

## Configure Claude Code

The MCP config ships committed in `.claude/settings.json` at the repo root,
so any collaborator who clones the repo and runs `npm install` + seed gets
it for free. The authenticated player is resolved automatically from the
seeded JSON — no per-user setup needed.

Restart Claude Code from inside the repo directory. The tools
(`list_scenarios`, `load_scenario`, `get_session`, `apply_move`, etc.)
become available in the next session.

### Overriding the player

By default the MCP logs in as `player1_id` from the seed. To override:

- `RECESS_PLAYER_KEY=player28_id` → pick a different seeded player by key
  (e.g. `player28_id` for the admin slot, `player24_id` for `bot_easy_1`).
- `RECESS_PLAYER_ID=<uuid>` → explicit UUID, bypasses the seed file
  entirely.

Put the override in `.claude/settings.local.json` (gitignored) so it
doesn't leak into shared config:

```json
{
  "mcpServers": {
    "recess-game": {
      "env": {
        "RECESS_PLAYER_KEY": "player28_id"
      }
    }
  }
}
```

## Tools

| Tool | Purpose |
|------|---------|
| `list_games` | All games registered on the server. |
| `list_scenarios` | Debug fixtures available for a game id. |
| `list_player_sessions` | Active sessions for a player (defaults to you). |
| `get_session` | Current state of a session by id. |
| `load_scenario` | Apply a named fixture. |
| `load_state` | Raw state override (full `engine.GameState`). |
| `apply_move` | Submit a move on behalf of the authenticated player. |
| `surrender` | End a session immediately. |

## Typical flow

1. Start a game in the browser (lobby → create room → add bot → start).
2. Ask Claude to drive it: "apply firewall_standoff to my active rootaccess
   session, then try playing ping".
3. Claude resolves `list_player_sessions` → `load_scenario` → `get_session`
   → `apply_move` without you touching anything.

## Security

- Auth uses `/auth/test-login`, which is 404 when `TEST_MODE=false`. Pointing
  this server at production will fail at startup.
- `ALLOW_DEBUG_SCENARIOS` gates the state-override routes on game-server and
  is similarly dev-only. Production images built via `make up-prod` never
  set it.
- The MCP runs locally, speaks to your local stack; no external exposure.

## Troubleshooting

- `test-login failed (404)` — auth-service isn't in TEST_MODE. Check
  `docker compose exec auth-service env | grep TEST_MODE`.
- `404` on `/games/:id/scenarios` — game-server doesn't have
  `ALLOW_DEBUG_SCENARIOS=true`. Bounce it with
  `docker compose up -d --no-deps game-server` after verifying the override
  sets it.
- Empty session list — you need to have started a game in the browser first.
  Future iteration could add a `create_session` tool that automates the
  lobby/start dance.
