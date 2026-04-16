# Audit: Reusable Backend Game Components

**Date:** 2026-04-16

## Inventory

The codebase uses a plugin architecture rather than a library of extracted components.
There are no standalone `Deck`, `Hand`, or `Board` structs — the reusable pieces are:

| Component | Location | Purpose |
|---|---|---|
| `engine` types | `services/game-server/internal/domain/engine/game.go` | Game interface, GameState, Player, Move, Result |
| `engine` lobby settings | `engine/lobby_settings.go` | LobbySetting types + platform defaults |
| `randutil` | `shared/platform/randutil/randutil.go` | Cryptographic Shuffle, Pick, Intn |

Game-specific code in `games/rootaccess/` and `games/tictactoe/` contains inline
deck, hand, and board operations that are not extracted into reusable components.

---

## Design Principles Evaluated

1. **Serialization** — State must be fully JSON round-trippable, no functions/interfaces in state
2. **Immutability** — Operations return new state, never mutate in place
3. **No domain logic** — Components are game-agnostic
4. **Determinism** — No internal randomness; resolved externally
5. **Configuration at construction** — Parameters fixed at init, not per-operation

---

## 1. `engine.GameState` + `Game` interface

**What it does:** Defines the contract every game plugin implements (`Init`, `ValidateMove`,
`ApplyMove`, `IsOver`) plus optional extensions (`StateFilter`, `TurnTimeoutHandler`,
`LobbySettingsProvider`). `GameState` wraps current player + an opaque `map[string]any` data bag.

**Result: all clear.**

- **Serialization:** All fields carry JSON tags. `Data` is `map[string]any` — fully round-trippable.
- **Immutability:** `ApplyMove` signature returns `(GameState, error)`, correct contract.
- **No domain logic:** Pure infrastructure, zero game-specific code.
- **Determinism:** No randomness in the package.
- **Config at construction:** Game metadata fixed at registration; runtime config via `LobbySettingsProvider`.

---

## 2. `randutil` (Shuffle, Pick, Intn)

**What it does:** Centralized cryptographic randomness — Fisher-Yates shuffle, random pick, random int.

| Principle | Status |
|---|---|
| Serialization | N/A (stateless utility) |
| Immutability | **Pass** — `Shuffle` returns a new slice |
| No domain logic | **Pass** — pure math |
| Determinism | **FAIL** — calls `crypto/rand` internally |
| Config at construction | N/A |

### Determinism violation

`Shuffle` and `Pick` generate randomness internally. Any caller becomes non-deterministic
and non-reproducible. You cannot replay a game from its move log because deck shuffles
cannot be reconstructed.

### Fix

Add seed-accepting or permutation-accepting variants:

```go
// Option A: accept a pre-computed permutation
func ShuffleWithPermutation[T any](s []T, perm []int) []T { ... }

// Option B: accept an RNG source
func ShuffleWith[T any](s []T, rng func(n int) int) []T { ... }
```

The existing `Shuffle` can remain as a convenience wrapper for production,
but game components should call the deterministic variant with externally-resolved randomness.

---

## 3. RootAccess — inline deck/hand/discard operations

**What it does:** `rootaccess.go` contains ~30 helper functions that manage deck (draw, shuffle,
set-aside), hands (add, remove, get), discard piles (append, order), player lifecycle (eliminate,
advance turn), and round resolution. `rootaccess_cards.go` defines card types, values, and deck
composition.

| Principle | Status |
|---|---|
| Serialization | **Pass** |
| Immutability | **FAIL** — mutates `state.Data` map in place |
| No domain logic | **FAIL** — card helpers entangled with game rules |
| Determinism | **FAIL** — `dealRound` calls `randutil.Shuffle` directly |
| Config at construction | **FAIL** (minor) — `tokensToWin` resolved per-call |

### 3a. Immutability violation (critical)

Every helper (`drawCard`, `removeFromHand`, `appendDiscard`, `eliminatePlayer`, etc.) receives
`state engine.GameState` by value — but `state.Data` is a `map[string]any` (reference type).
Writing `state.Data["hands"] = ...` mutates the caller's map. There are 47 `state.Data[...] = ...`
assignments across the file, all in-place mutations.

This is masked because the engine only uses the returned state, but it's fragile — if any caller
retained a reference to the pre-call state (undo, logging, bot MCTS tree cloning), they'd see
corrupted data.

**Fix:** Deep-copy `state.Data` once at the `ApplyMove` entry point:

```go
func (g *RootAccess) ApplyMove(state engine.GameState, move engine.Move) (engine.GameState, error) {
    state.Data = deepCopyData(state.Data)
    // rest of logic — now safe to mutate state.Data
}
```

Single copy per move instead of one per helper call.

### 3b. Domain logic violation

`rootaccess_cards.go` mixes potentially reusable deck operations with game-specific rules:

- `tokensToWin(playerCount)` — win condition (game rule)
- `isValidGuess(c)` — Ping card validation (game rule)
- `cardValues` map — game-specific card ranking
- `deckComposition` — game-specific deck contents

If another card game is added, none of this is reusable. Generic operations (build deck, shuffle,
draw, move between zones) are interleaved with RootAccess semantics.

**Fix (only when a second card game is added):** Extract a generic `deck` component:

```go
type Deck[C comparable] struct {
    Cards []C `json:"cards"`
}

func NewDeck[C comparable](composition []C) Deck[C] { ... }
func (d Deck[C]) Draw() (C, Deck[C], error) { ... }
func (d Deck[C]) Shuffle(perm []int) Deck[C] { ... }
```

Leave `tokensToWin`, `isValidGuess`, `cardValues`, `deckComposition` inside `rootaccess` —
those are game rules that belong there. Extraction is premature until a second card game exists.

### 3c. Determinism violation

`dealRound` at `rootaccess.go:711-712`:

```go
deck, err := randutil.Shuffle(unshuffled)
```

The shuffle is not reproducible. Same state + move sequence cannot replay to the same outcome.

**Fix:** Resolve the shuffle outside and pass it in, or store the permutation in state:

```go
func dealRound(state engine.GameState, players []string, firstPlayer engine.PlayerID, shuffledDeck []CardName) engine.GameState {
    // use shuffledDeck instead of calling Shuffle internally
}
```

### 3d. Configuration-at-construction violation (minor)

`tokensToWin(playerCount)` is called during round resolution (`rootaccess.go:574`), recomputing
the win threshold each time. Player count doesn't change mid-game, so this is safe but violates
the principle.

**Fix:** Store in state during `Init`:

```go
state.Data["tokens_to_win"] = tokensToWin(len(players))
```

Then read from state instead of recomputing.

---

## 4. TicTacToe — inline board operations

**What it does:** Minimal implementation with `[9]string` board, mark assignment, win-line checking.

**Result: all clear.**

- **Serialization:** `[9]string`, `map[string]string`, `[]string` — all JSON-safe.
- **Immutability:** `ApplyMove` extracts board as `[9]string` value (copy), mutates copy, builds fresh `GameState`.
- **Determinism:** No randomness.
- **Config at construction:** Board is always 3x3, inherent to the game.

---

## Summary

| Component | Serialization | Immutability | Domain Logic | Determinism | Config at Ctor |
|---|---|---|---|---|---|
| `engine` types | Pass | Pass | Pass | Pass | Pass |
| `randutil` | N/A | Pass | Pass | **Fail** | N/A |
| RootAccess helpers | Pass | **Fail** | **Fail** | **Fail** | **Fail** (minor) |
| TicTacToe | Pass | Pass | N/A | Pass | Pass |

## Priority

1. **RootAccess immutability** — deep-copy `state.Data` at `ApplyMove` entry. Most dangerous: bot MCTS tree-search clones state and would silently corrupt nodes.
2. **Determinism** — if game replay or deterministic testing matters, externalize the shuffle in `dealRound`. Otherwise accept as known trade-off.
3. **Component extraction** — only when a second card game is added. Until then, inline approach avoids premature abstraction.
