# Adding a Game

Games are pluggable on both backend and frontend. The engine handles rooms, turns, timers, bots, spectators, and replays — you only implement game logic. A single game folder on the backend holds the engine rules, the bot adapter, and lobby settings; a matching folder on the frontend holds the renderer and rules; a small leaf package under `shared/achievements/` holds the per-game achievements.

Everything below registers via `init()`. Adding a game is additive — no edits to generic code in `engine/`, `runtime/`, `api/`, `adapter/registry.go`, `features/game/`, etc.

## Backend (Go)

### 1. Implement the Game Interface

Create a new package under `services/game-server/games/`:

```
services/game-server/games/mygame/
  mygame.go       # engine.Game implementation
  adapter.go      # bot.BotAdapter implementation (optional)
  lobby_settings.go  # custom lobby settings (optional)
```

The core interface (`services/game-server/internal/domain/engine/game.go`):

```go
type Game interface {
    ID() string                                              // unique id (e.g., "chess")
    Name() string                                            // display name
    MinPlayers() int
    MaxPlayers() int
    Init(players []Player) (GameState, error)                // set up initial state
    ValidateMove(state GameState, move Move) error           // check legality
    ApplyMove(state GameState, move Move) (GameState, error) // apply and return new state
    IsOver(state GameState) (bool, Result)                   // check termination
}
```

**Key types:**

- `GameState` — contains `CurrentPlayerID` and `Data map[string]any` (your game-specific state).
- `Move` — contains `PlayerID`, `Payload map[string]any`, and `Timestamp`.
- `Result` — contains `Status` (win/draw) and optional `WinnerID`.

**Rules the engine relies on:**

- `ApplyMove` must be pure: same state + same move → same new state, no internal `rand`, no `time.Now()`, no map iteration where order affects output. Deep-copy `state.Data` at entry (use `engine.DeepCopyData` — see `rootaccess.go` and `tictactoe.go`).
- Any randomness must be seeded once in `Init` and stored in state (see RootAccess's `game_seed`).
- Move validation lives in `ValidateMove`; `ApplyMove` is only called after it passes. The runtime enforces the current player's turn before either.

### 2. Optional Interfaces

**StateFilter** — for games with hidden information (e.g., cards):

```go
type StateFilter interface {
    FilterState(state GameState, playerID PlayerID) GameState
}
```

The engine calls this before sending state to each player, so opponents only see what they should. `runtime.BroadcastMove` and `runtime.BroadcastSessionStarted` both respect this — implementing it is all that's needed for per-player projection.

**TurnTimeoutHandler** — custom behavior when a turn times out:

```go
type TurnTimeoutHandler interface {
    TimeoutMove(penalty string) map[string]any
}
```

Without this, the platform applies its default timeout penalty.

**LobbySettingsProvider** — declare custom pre-game settings the room owner can tune.

### 3. Register the Game

Use an `init()` function. Every plugin has one anyway, since the bot adapter (below) registers from the same function or a sibling one:

```go
package mygame

import "github.com/recess/game-server/games"

func init() {
    games.Register(&MyGame{})
}
```

Then blank-import the package in `services/game-server/cmd/server/main.go` (and `cmd/bot-runner/main.go` if your game has a bot):

```go
import _ "github.com/recess/game-server/games/mygame"
```

The registry (`services/game-server/games/registry.go`) panics on duplicate IDs to prevent misregistration.

### 4. Bot Support (optional)

To add AI for your game, put the adapter in the **same package** as the engine (`games/mygame/adapter.go`) and register it from `init()`:

```go
package mygame

import (
    "github.com/recess/game-server/internal/bot"
    botadapter "github.com/recess/game-server/internal/bot/adapter"
)

func init() {
    botadapter.Register("mygame", botadapter.Factory{
        New: func() bot.BotAdapter { return NewAdapter() },
        // Optional — implement if RolloutPolicy should depend on personality:
        NewWithProfile: func(p bot.PersonalityProfile) bot.BotAdapter {
            return NewAdapterWithProfile(p)
        },
    })
}

type Adapter struct { game *MyGame }
func NewAdapter() *Adapter { return &Adapter{game: &MyGame{}} }
// ...implement bot.BotAdapter: ValidMoves, ApplyMove, IsTerminal, Result,
//   CurrentPlayer, Determinize, RolloutPolicy
```

Because the adapter lives in the same package as the engine, the blank import in `main.go` (step 3) wires both at once. Do NOT create a separate adapter package under `internal/bot/adapter/` — that path exists only for the generic registry and should not gain subdirectories.

See `games/rootaccess/adapter.go` and `games/tictactoe/adapter.go` for working examples, plus `BOT.md` for the MCTS design and personality profiles.

### 5. Achievements (optional)

Each game's achievements live in `shared/achievements/games/<gameid>/<gameid>.go` and self-register from `init()`:

```go
package mygame

import (
    "github.com/recess/shared/achievements"
    "github.com/recess/shared/events"
)

func init() {
    achievements.Register(achievements.Definition{
        Key:     "mg_first_win",
        NameKey: achievements.NameKey("mg_first_win"),
        GameID:  "mygame",
        Type:    achievements.TypeFlat,
        Tiers: []achievements.Tier{
            {
                Threshold:      1,
                NameKey:        achievements.TierNameKey("mg_first_win", 1),
                DescriptionKey: achievements.TierDescriptionKey("mg_first_win", 1),
            },
        },
        ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
            if outcome != "win" {
                return cur.Progress, false
            }
            return 1, true
        },
    })
}
```

The `ComputeProgress` closure replaces any switch in the evaluator: it returns the new progress value and whether the event applied. Co-locating the rule with the metadata keeps the evaluator generic.

Blank-import the plugin package in `services/user-service/cmd/server/main.go`:

```go
import _ "github.com/recess/shared/achievements/games/mygame"
```

Register i18n strings in `frontend/src/locales/{en,es}.json` under the `achievements.{key}` keys:

```json
"mg_first_win": {
  "name": "First Win",
  "description": "Win your first game of My Game",
  "tiers": {
    "1": { "name": "Hacker", "description": "Win your first game" }
  }
}
```

See `shared/achievements/games/tictactoe/tictactoe.go` for a working example, and `docs/frontend/overview.md` for the positional-key contract.

## Frontend (TypeScript)

### 1. Create the Game Package

```
frontend/src/games/mygame/
  index.ts          # barrel export (public API)
  MyGameBoard.tsx   # main renderer
  MyGameRules.tsx   # rules modal content
  types.ts          # game-specific state types
```

All internal exports **must** have `/** @package */` JSDoc. Only the barrel re-exports are public. Biome's `noPrivateImports` rule enforces this for relative-path imports (aliased `@/` imports are a known Biome limitation).

### 2. Register in the Frontend

In `frontend/src/games/registry.tsx`:

```tsx
import { MyGameBoard, MyGameRules, type MyGameState } from '@/games/mygame'

const MyGameRenderer: RendererComponent = ({
  gameData,
  localPlayerId,
  onMove,
  disabled,
  players,
}) => (
  <MyGameBoard
    state={gameData.data as MyGameState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={onMove}
    disabled={disabled}
    players={players}
  />
)

// Add to GAME_RENDERERS
mygame: MyGameRenderer,

// Add to GAME_RULES
{ id: 'mygame', label: 'My Game', component: MyGameRules },
```

Both `GAME_RENDERERS` and `GAME_RULES` are consumed dynamically — `features/game/`, `features/game/Replay.tsx`, and `ui/RulesModal.tsx` only know the registry contract, not your specific game.

### 3. Card-Based Games

If your game uses cards, use the shared Card UI kit (`frontend/src/ui/cards/`):

- `Card` — single card with flip animation
- `CardHand` — fan layout with hover lift
- `CardPile` — stack visualization
- `CardZone` — layout container (pile/spread/stack)

Each game provides its own face content via render props. See `rootaccess/` for a full example.

## Checklist

Backend:

- [ ] Engine in `services/game-server/games/mygame/mygame.go` + `init()` calling `games.Register(&MyGame{})`
- [ ] `_ "github.com/recess/game-server/games/mygame"` in `cmd/server/main.go` (and `cmd/bot-runner/main.go` if botted)
- [ ] JSON Schema for any new move payloads under `shared/schemas/` (run `make gen-types` after)
- [ ] Bot adapter in `games/mygame/adapter.go` + `init()` calling `botadapter.Register("mygame", ...)` — same package, no new blank import needed
- [ ] Achievements (if any) in `shared/achievements/games/mygame/mygame.go` + blank import in `services/user-service/cmd/server/main.go`
- [ ] Routing entries in `scripts/test-routing.sh` if you added new HTTP endpoints

Frontend:

- [ ] Game package in `frontend/src/games/mygame/` with barrel export
- [ ] `/** @package */` on all internal exports
- [ ] Registered in `frontend/src/games/registry.tsx` (renderer + rules)
- [ ] i18n entries in `src/locales/{en,es}.json` — game-specific strings under `mygame.*`, achievement strings under `achievements.{key}.*`
