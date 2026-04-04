# Bot System — Technical Handoff

> **Purpose of this document:** This file describes the full design and requirements for implementing an IS-MCTS (Information Set Monte Carlo Tree Search) bot system, embedded as a Go library inside the existing game platform. Paste this file alongside the project's `handsoff` file so the AI assistant can cross-reference what already exists and what needs to be built.

---

## 1. Overview

We want to add one or more AI bots that can join any game room on the platform and play as a regular player. The bot system must be:

- **Embedded** as a Go library inside the existing server (not a separate microservice).
- **Game-agnostic**: the same MCTS engine must work for any game (currently TicTacToe and Root Access, more in the future).
- **Configurable**: difficulty, time limits, style of play, and number of concurrent bots must all be tunable.
- **Integrated with the existing room/player system**: a bot joins a room the same way a human player does, receives game state updates via the same WebSocket flow, and responds with moves via the same HTTP REST endpoints.

---

## 2. Algorithm: IS-MCTS (Information Set MCTS)

Standard MCTS does not handle hidden information (e.g. opponent's hand in Root Access). IS-MCTS solves this by **determinization**: before running the search tree, the bot samples a plausible "complete" world consistent with its observations, runs MCTS on that world, and repeats this N times. The final move is the one with the best average score across all sampled worlds.

### The four MCTS steps (per determinization)

```
1. SELECT      — walk down the tree choosing nodes by UCB1 score
2. EXPAND      — try one untried move from the selected node
3. SIMULATE    — play randomly to a terminal state (rollout)
4. BACKPROPAGATE — update win/visit counts up the tree
```

### UCB1 formula

```
UCB1 = (wins / visits) + C * sqrt(ln(parent.visits) / visits)
```

Where `C` is the exploration constant (classic value: `√2 ≈ 1.41`). This is a configurable parameter.

### Parallelism

Each determinization runs in its own goroutine. Results (move → total score, move → visit count) are merged with a mutex after all goroutines finish. This makes the bot naturally multi-core.

---

## 3. Required Go Interfaces

This is the **critical section**. The bot engine only knows about these interfaces. Every game must implement them.

### 3.1 `GameState`

```go
// GameState is the snapshot of a game at a point in time.
// MUST support deep copy — MCTS branches the state thousands of times.
type GameState interface {
    Clone() GameState
}
```

> **Ask the assistant:** Show me the current structs that represent game state for TicTacToe and Root Access. We need to verify that Clone() can be implemented correctly (no shared pointers or maps that would cause aliasing bugs).

### 3.2 `GameAdapter`

```go
// GameAdapter is the bridge between the MCTS engine and a specific game.
// Each game implements this interface.
type GameAdapter interface {
    // ValidMoves returns all legal moves for the current player in state s.
    ValidMoves(s GameState) []Move

    // ApplyMove applies move m to state s and returns the resulting NEW state.
    // Must NOT mutate s — return a new value.
    ApplyMove(s GameState, m Move) GameState

    // IsTerminal returns true if the game is over (win, loss, draw, no moves).
    IsTerminal(s GameState) bool

    // Result returns a score in [0, 1] for the given player at a terminal state.
    // Typical values: 1.0 = win, 0.0 = loss, 0.5 = draw.
    Result(s GameState, player PlayerID) float64

    // CurrentPlayer returns the ID of the player whose turn it is.
    CurrentPlayer(s GameState) PlayerID

    // Determinize fills in hidden information (e.g. opponent's hand) with a
    // random sample consistent with what `observer` can legally know.
    // For games with no hidden info (TicTacToe), this is an identity function.
    Determinize(s GameState, observer PlayerID) GameState
}
```

### 3.3 `Move`

```go
// Move is an opaque identifier for a player action.
// The engine doesn't need to know what it means — the adapter does.
// Must be comparable (usable as a map key).
type Move interface {
    comparable
}
```

In practice this will likely be a `string` (e.g. `"place:1:2"`) or a small struct. To be decided per game.

### 3.4 `PlayerID`

```go
type PlayerID string  // or int — to match the existing player model
```

> **Ask the assistant:** How are players identified in the current system? Is there a Player struct or just an ID? Show me the relevant files.

---

## 4. Bot Configuration

Bots are configured at creation time. Configuration comes from two sources:

| Parameter | Source | Description |
|---|---|---|
| `Iterations` | Env var / arg | MCTS iterations per determinization. Higher = stronger but slower. Default: `1000` |
| `Determinizations` | Env var / arg | How many parallel world samples to run. Default: `20` |
| `MaxThinkTime` | Env var / arg | Hard deadline for a move decision (e.g. `2s`). The bot must respond before the game's turn timer fires. |
| `ExplorationC` | Env var / arg | UCB1 exploration constant. Default: `1.41` |
| `Personality` | Room creation / API | Named profile that presets the above parameters (see Section 5). |

### Environment variable names (proposed)

```
BOT_ITERATIONS=1000
BOT_DETERMINIZATIONS=20
BOT_MAX_THINK_TIME=2s
BOT_EXPLORATION_C=1.41
```

### Struct

```go
type BotConfig struct {
    Iterations       int
    Determinizations int
    MaxThinkTime     time.Duration
    ExplorationC     float64
    Personality      PersonalityProfile
}
```

---

## 5. Personality Profiles

Personalities are **named presets** that set the numerical parameters above plus optional heuristic weights used in the rollout/evaluation phase.

```go
type PersonalityProfile struct {
    Name        string
    // Preset MCTS parameters
    Iterations       int
    Determinizations int
    ExplorationC     float64
    // Heuristic weights (game-agnostic values, interpreted per game adapter)
    Aggressiveness float64 // 0.0 (passive) to 1.0 (aggressive)
    RiskAversion   float64 // 0.0 (risk-loving) to 1.0 (risk-averse)
}

// Built-in profiles
var Profiles = map[string]PersonalityProfile{
    "easy":       {Name: "easy",       Iterations: 100,  Determinizations: 5,  ExplorationC: 2.0,  Aggressiveness: 0.3, RiskAversion: 0.7},
    "medium":     {Name: "medium",     Iterations: 500,  Determinizations: 10, ExplorationC: 1.41, Aggressiveness: 0.5, RiskAversion: 0.5},
    "hard":       {Name: "hard",       Iterations: 2000, Determinizations: 30, ExplorationC: 1.0,  Aggressiveness: 0.7, RiskAversion: 0.3},
    "aggressive": {Name: "aggressive", Iterations: 1000, Determinizations: 20, ExplorationC: 0.8,  Aggressiveness: 1.0, RiskAversion: 0.1},
}
```

The frontend can show these profiles as selectable options when adding a bot to a room.

---

## 6. Bot Lifecycle & Integration

### 6.1 How a bot joins a room

The flow should mirror how a human player joins:

1. Frontend sends a request: `POST /rooms/{roomID}/bots` with `{ "personality": "medium" }`.
2. Server creates a `BotPlayer` entity with a generated ID (e.g. `"bot-uuid"`).
3. `BotPlayer` is added to the room just like a human player.
4. When the game starts and it's the bot's turn, the server calls `bot.RequestMove(state)` instead of waiting for a WebSocket message.

> **Ask the assistant:** How does the current room/lobby system work? Show me the Room struct and how players are added. Is there already a Player interface or is player always a concrete struct?

### 6.2 Turn handling

```
Game engine detects: current player = bot
    ↓
Calls bot.RequestMove(state, deadline)
    ↓
Bot runs IS-MCTS within deadline
    ↓
Returns Move
    ↓
Game engine applies move (same path as human move)
```

The `deadline` is derived from the game's per-turn timer. The bot must return a move before it expires. If MCTS hasn't finished, it returns the best move found so far (context cancellation via `context.WithDeadline`).

### 6.3 Multiple concurrent bots

Each `BotPlayer` instance is independent. Since goroutines are cheap, multiple bots in different rooms run concurrently with no shared state. A `BotManager` struct can track active bots:

```go
type BotManager struct {
    bots map[RoomID]*BotPlayer
    mu   sync.RWMutex
}
```

---

## 7. Proposed Package Structure

```
internal/
  bot/
    mcts/
      node.go       — Node struct, UCB1
      mcts.go       — Core loop: select, expand, simulate, backpropagate
      engine.go     — IS-MCTS orchestration (determinizations, goroutines, merge)
    config.go       — BotConfig, PersonalityProfile, env var loading
    player.go       — BotPlayer (implements the existing Player interface)
    manager.go      — BotManager (create, remove, list bots)
  games/
    adapter.go      — GameAdapter, GameState, Move interfaces
    tictactoe/
      adapter.go    — TicTacToe implementation of GameAdapter
    rootaccess/
      adapter.go    — RootAccess implementation of GameAdapter
```

---

## 8. What to Ask the Assistant (Session Checklist)

Use these prompts in future sessions, each with this file + the relevant handsoff sections:

### Session A — Interfaces & State
> "Here is bot.md and the current game state structs [paste files]. Verify that the GameState and GameAdapter interfaces are compatible with the existing code. Identify what's missing and implement Clone() for TicTacToe and RootAccess."

### Session B — Core MCTS Engine
> "Here is bot.md and the current project structure [paste files]. Implement the packages `internal/bot/mcts/node.go`, `mcts.go`, and `engine.go` as described. Do not implement any game adapter yet — use a mock GameAdapter for tests."

### Session C — TicTacToe Adapter
> "Here is bot.md and the TicTacToe game logic files [paste files]. Implement `internal/games/tictactoe/adapter.go`. Include a Determinize that is an identity function since TicTacToe has no hidden info. Write a test that runs the bot against itself."

### Session D — Root Access Adapter
> "Here is bot.md and the Root Access game logic files [paste files]. Implement `internal/games/rootaccess/adapter.go`. The Determinize function must sample the opponent's hand from the cards not visible to the observer. Write a test."

### Session E — BotPlayer & Integration
> "Here is bot.md and the room/player system files [paste files]. Implement BotPlayer so it satisfies the existing Player interface. Implement the POST /rooms/{id}/bots endpoint. Wire up the bot's turn into the game loop."

### Session F — Config, Personalities & API
> "Here is bot.md and the server config files [paste files]. Implement BotConfig loading from environment variables and the PersonalityProfile presets. Expose profiles via GET /bots/profiles so the frontend can populate the bot creation UI."

---

## 9. Open Questions (to resolve with handsoff)

These need to be answered by reading the existing codebase:

- [ ] How is `PlayerID` typed today? (`string`, `uuid.UUID`, `int`?)
- [ ] Is there a `Player` interface or a concrete `Player` struct?
- [ ] How does the game engine signal "it's your turn"? (method call, channel, event?)
- [ ] Where is turn timer logic? Can we hook into it to set the bot's deadline?
- [ ] Are game states stored in Postgres as JSON blobs? If so, can we deserialize them into the structs needed by the adapter?
- [ ] Does Root Access already have a concept of "visible" vs "hidden" card state server-side?

---

## 10. Key Implementation Rules

These are non-negotiable for correctness:

1. **`Clone()` must be a true deep copy.** Any shared pointer between the original and the clone will cause silent, hard-to-debug race conditions during parallel MCTS.
2. **`ApplyMove()` must never mutate its input.** Always return a new state.
3. **`Determinize()` must produce a state consistent with the observer's knowledge.** Never reveal cards the observer cannot know.
4. **The bot must always respect `MaxThinkTime`.** Use `context.WithDeadline` and check `ctx.Done()` in the MCTS loop. A bot that times out a turn is worse than a weak bot.
5. **Bot IDs must be distinguishable from human player IDs** to avoid routing game events to the wrong handler.