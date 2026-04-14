# tf-mcp-game

Local MCP server that exposes tableforge game-server operations as tools for
Claude Code. Lets Claude apply scenario fixtures, load arbitrary game states,
submit moves, and read current state — all against the local dev stack.

## Prereqs

- The stack is running (`make up-all`) with dev mode enabled:
  - `auth-service` runs with `TEST_MODE=true` (automatic under the override).
  - `game-server` runs with `ALLOW_DEBUG_SCENARIOS=true` (automatic under the
    override).
- Node 20+ (the MCP SDK needs it).
- A seeded test player UUID — e.g. one of the `bot_easy_1` / `bot_hard_1`
  rows created by `make seed-test`, or your own GitHub-logged player.

## Install

```bash
cd tools/mcp-game
npm install
```

## Configure Claude Code

Add to your Claude Code settings (`~/.claude/settings.json` or
`~/.config/claude/claude_desktop_config.json`, depending on platform):

```json
{
  "mcpServers": {
    "tf-game": {
      "command": "npx",
      "args": ["--yes", "tsx", "/absolute/path/to/tableforge/tools/mcp-game/src/index.ts"],
      "env": {
        "TF_BASE_URL": "http://localhost",
        "TF_PLAYER_ID": "<player-uuid>"
      }
    }
  }
}
```

Restart Claude Code. The tools (`list_scenarios`, `load_scenario`,
`get_session`, `apply_move`, etc.) become available in the next session.

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
