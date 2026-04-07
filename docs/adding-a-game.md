# Adding a Game

Games are pluggable on both backend and frontend. The engine handles rooms, turns, timers, bots, spectators, and replays -- you only implement game logic.

## Backend (Go)

### 1. Implement the Game Interface

Create a new package under `services/game-server/games/`:

```
services/game-server/games/mygame/
  mygame.go
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

- `GameState` -- contains `CurrentPlayerID` and `Data map[string]any` (your game-specific state).
- `Move` -- contains `PlayerID`, `Payload map[string]any`, and `Timestamp`.
- `Result` -- contains `Status` (win/draw) and optional `WinnerID`.

### 2. Optional Interfaces

**StateFilter** -- for games with hidden information (e.g., cards):

```go
type StateFilter interface {
    FilterState(state GameState, playerID PlayerID) GameState
}
```

The engine calls this before sending state to each player, so opponents only see what they should.

**TurnTimeoutHandler** -- custom behavior when a turn times out:

```go
type TurnTimeoutHandler interface {
    TimeoutMove() map[string]any
}
```

Without this, the platform applies its default timeout penalty.

### 3. Register the Game

Use an `init()` function to auto-register when the package is imported:

```go
package mygame

import "github.com/recess/game-server/games"

func init() {
    games.Register(&MyGame{})
}
```

Then import the package in `services/game-server/cmd/server/main.go`:

```go
import _ "github.com/recess/game-server/games/mygame"
```

The registry (`services/game-server/games/registry.go`) panics on duplicate IDs to prevent misregistration.

## Frontend (TypeScript)

### 1. Create the Game Package

```
frontend/src/games/mygame/
  index.ts          # barrel export (public API)
  MyGameBoard.tsx   # main renderer
  MyGameRules.tsx   # rules modal content
  types.ts          # game-specific state types
```

All internal exports **must** have `/** @package */` JSDoc. Only the barrel re-exports are public. Biome's `noPrivateImports` rule enforces this.

### 2. Register in the Frontend

In `frontend/src/games/registry.tsx`:

```tsx
import { MyGameBoard } from '@/games/mygame'
import { MyGameRules } from '@/games/mygame'

// Add to GAME_RENDERERS
mygame: (props) => <MyGameBoard {...props} />,

// Add to GAME_RULES
{ id: 'mygame', label: 'My Game', rules: MyGameRules },
```

The renderer receives `gameData`, `localPlayerId`, `onMove`, and other props from the engine.

### 3. Card-Based Games

If your game uses cards, use the shared Card UI kit (`frontend/src/ui/cards/`):

- `Card` -- single card with flip animation
- `CardHand` -- fan layout with hover lift
- `CardPile` -- stack visualization
- `CardZone` -- layout container (pile/spread/stack)

Each game provides its own face content via render props. See `rootaccess/` for a full example.

## Bot Support

To add AI for your game, implement the `GameAdapter` interface for the IS-MCTS bot engine:

```go
type GameAdapter interface {
    ValidMoves(s GameState) []Move
    ApplyMove(s GameState, m Move) GameState
    IsTerminal(s GameState) bool
    Result(s GameState, player PlayerID) float64   // [0,1] score
    CurrentPlayer(s GameState) PlayerID
    Determinize(s GameState, observer PlayerID) GameState
}
```

Place the adapter in `internal/bot/adapter/mygame/adapter.go`. See `BOT.md` for the full MCTS design and personality profiles.

## Checklist

- [ ] Game package in `services/game-server/games/mygame/`
- [ ] `init()` registration + import in `main.go`
- [ ] Frontend package in `frontend/src/games/mygame/` with barrel export
- [ ] `/** @package */` on all internal exports
- [ ] Registered in `frontend/src/games/registry.tsx`
- [ ] JSON Schema for any new move payloads in `shared/schemas/`
- [ ] Bot adapter (optional)
