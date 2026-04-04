# Root Access — Implementation Plan

## Game overview

Root Access (formerly Love Letter) is a card game of deduction and risk for 2–5 players. Each round,
players try to have the highest-value card at the end, or be the last player
standing. The full game is won by collecting a number of Access Tokens (affection
tokens) that depends on player count.

This plan targets the **updated edition** (20-card deck) which adds the BACKDOOR (0)
and DEBUGGER (6) and shifts King, Countess and Princess up by one value.

---

## Cards

The updated deck has 20 cards across 9 types:

| Card | Value | Count | Effect when played |
|---|---|---|---|
| BACKDOOR | 0 | 2 | No immediate effect. At the end of the round, if you are the only player who played or discarded a Spy this round, you gain 1 Access Token (even if eliminated) |
| PING | 1 | 6 | Name a non-Guard card. If the target player holds it, they are eliminated |
| SNIFFER | 2 | 2 | Look at another player's hand (private — only the caster sees it) |
| BUFFER_OVERFLOW | 3 | 2 | Compare hands with another player in secret; lower value is eliminated (tie = nothing happens) |
| FIREWALL | 4 | 2 | Protected from all card effects until your next turn |
| REBOOT | 5 | 2 | Choose a player (including yourself) to discard their hand and draw a new card. If the Princess is discarded this way, that player is eliminated |
| DEBUGGER | 6 | 2 | Draw 2 cards from the deck. Keep 1, place the other 2 at the bottom of the deck in any order. If fewer than 2 cards remain in the deck, draw as many as available |
| SWAP | 7 | 1 | Trade hands with another player |
| ENCRYPTED_KEY | 8 | 1 | Must be played if you also hold a King or Prince in hand |
| ROOT | 9 | 1 | If you play or discard this card for any reason, you are eliminated |

---

## Round flow

1. Shuffle the deck. Set aside 1 card face-down (unknown). In a 2-player game,
   also set aside 3 additional cards face-up (visible but not in play).
2. Deal 1 card to each player.
3. On your turn:
   a. Draw the top card from the deck (you now hold 2 cards).
   b. Play one card face-up to your discard pile and resolve its effect.
   c. **Chancellor special step:** if you played the Chancellor, before ending
      your turn you must choose which of the 3 cards in hand to keep, and place
      the remaining 2 at the bottom of the deck in any order. This requires an
      additional move payload (see below).
4. The round ends when:
   - The deck is empty after a draw (players compare hands; highest value wins), or
   - Only one player remains (all others eliminated).
5. Spy token check: if exactly one player played or discarded a Spy this round
   (regardless of whether they are still in the round), that player gains 1 token.
6. The round winner gains 1 Access Token.
7. Shuffle and start a new round. The player who most recently won a round goes first.

### Tokens to win
| Players | Tokens needed |
|---|---|
| 2 | 7 |
| 3 | 5 |
| 4 | 4 |
| 5 | 3 |

---

## Move structure

### Standard move
Every turn produces a move: **play a card** + optional **target parameters**.

```json
{
  "card": "guard",
  "target_player_id": "uuid",   // required for: Guard, Priest, Baron, Prince, King
  "guess": "priest"             // required for: Guard only (the card being guessed)
}
```

Cards with no target: **Spy**, **Handmaid**, **Countess**, **Princess**,
and **Prince** when targeting self.

### Chancellor follow-up move
After playing the Chancellor, a second move is required to resolve the effect.
The session enters a `chancellor_pending` sub-phase and waits for this payload:

```json
{
  "card": "chancellor_resolve",
  "keep": "king",
  "return": ["guard", "priest"]   // order matters — bottom of deck
}
```

The engine must reject any other move while `chancellor_pending` is active.

---

## State model (proposed)

### Public state (visible to all players)
```json
{
  "round": 1,
  "phase": "playing" | "chancellor_pending" | "round_over" | "game_over",
  "current_player_id": "uuid",
  "deck_remaining": 10,
  "tokens": { "player_a": 2, "player_b": 1 },
  "eliminated_this_round": ["player_b"],
  "protected": ["player_a"],
  "spy_played_by": ["player_a"],
  "discard_piles": {
    "player_a": ["guard", "baron"],
    "player_b": ["handmaid"]
  },
  "set_aside_visible": ["guard", "guard", "priest"],
  "round_winner_id": "uuid | null",
  "game_winner_id": "uuid | null"
}
```

### Private state (per player — filtered before sending)
```json
{
  "hand": ["prince"]
}
```

### Chancellor pending state (only visible to the Chancellor player)
```json
{
  "chancellor_choices": ["king", "guard"]
}
```

---

## Key system requirements to review

### 1. Private state per player — CRITICAL

The current `GameState` struct sends the same state to all clients:
```go
type GameState struct {
    CurrentPlayerID PlayerID
    Data            any  // sent as-is to all players
}
```

Love Letter requires each player to receive a filtered view of `Data` —
specifically, `hand` must only contain that player's own card, and
`chancellor_choices` must only be visible to the active player.

**Options to evaluate:**
- Add a `FilterState(state GameState, playerID PlayerID) GameState` method to the
  `engine.Game` interface — called per-player before broadcasting
- Keep a single canonical state server-side; derive per-player views at broadcast time
- Store private state separately from public state in `Data` (e.g. nested `private` map
  keyed by player ID, stripped before sending to others)

The WS broadcast in `runtime.go` (`applyMove`, `handleGameOver`) currently does:
```go
hub.Broadcast(roomID, ws.Event{Type: ws.EventMoveApplied, Payload: result})
```
This needs to become per-player sends using `hub.BroadcastToPlayer` for Love Letter.

### 2. Multi-phase turns — Chancellor

The Chancellor introduces a two-step turn: play the card, then resolve it.
The engine needs a way to signal that the current turn is not over yet and
is waiting for a follow-up move from the same player.

Options:
- Add a `Pending bool` or `Phase string` field to `GameState` — the runtime
  checks this before advancing `CurrentPlayerID`
- Treat the follow-up as a separate move type validated against the current phase

### 3. Move parameters beyond `cell`

TicTacToe moves are `{ "cell": 0 }`. Love Letter moves need:
- `card` (which card to play)
- `target_player_id` (optional)
- `guess` (optional, Guard only)
- `chancellor_resolve` payload (Chancellor follow-up only)

The current `move` payload is `Record<string, unknown>` on the frontend and
`[]byte` (raw JSON) on the backend — this already works. No engine interface
change needed here, but the frontend renderer needs to build the correct payload
depending on the card played.

### 4. Countess forced-play rule

The Countess **must** be played if the player also holds a King or Prince.
This is a client-side concern (disable other cards in the UI) but also needs
server-side enforcement in `ValidateMove`.

### 5. Multi-round state

The current session model is single-game — `IsOver` returns true once and
the session is finished. Love Letter needs:
- Round resets (reshuffle, redeal) without ending the session
- Token accumulation across rounds including Spy tokens
- Game-over only when a player reaches the token threshold

**Recommended approach:** single session, engine manages rounds internally.
`IsOver` only returns true when a player has enough tokens. Round history
is embedded in `Data`.

### 6. Priest effect — reveal information

When a Priest is played, the caster sees the target's hand privately.
Options:
- Return the revealed card in `ApplyMove` result inside a `private_info`
  field scoped to the caster's player ID
- Send a separate `BroadcastToPlayer` event to the caster only after the
  move is applied

### 7. Baron effect — private comparison

The Baron causes two players to compare hands. The result (who was
eliminated) is public; neither player's actual card value should be
revealed to third players.

### 8. Spy token — end-of-round check

The Spy token is awarded at the end of the round, not when played. The
engine must track which players played or discarded a Spy during the round
(`spy_played_by` in public state) and apply the bonus during round resolution.
Note: the Spy token is awarded even if that player was eliminated.

### 9. Chancellor — deck manipulation

The Chancellor requires inserting 2 cards at the bottom of the deck in a
chosen order. The deck must be stored as an ordered list in the canonical
state (not just a count). This increases state size slightly but is necessary.

### 10. 2-player variant — set-aside visible cards

In 2-player games, 3 cards are set aside face-up at the start of each round.
These are public information included in `set_aside_visible`.

### 11. Frontend renderer

A full Love Letter renderer needs:
- Hand display (1 card normally, 2 after drawing) — hidden from opponents
- Card selection with effect description tooltip
- Target player picker (for Guard, Priest, Baron, Prince, King)
- Card name picker (Guard guess — dropdown of non-Guard card names)
- Chancellor follow-up UI (pick 1 to keep from 3, order the 2 to return)
- Discard pile per player (face-up, all visible)
- "Protected" indicator per player (Handmaid active)
- Spy indicator (track who has played a Spy this round)
- Token counter per player
- Deck counter
- Set-aside visible cards (2-player only)
- Round summary screen (winner, winning card, Spy token awarded, token totals)
- Game summary screen (overall winner)

### 12. Turn timeout behaviour

Love Letter turns involve decisions (which card to play, who to target).
`PenaltyLoseTurn` does not map to a valid Love Letter game state — skipping
a turn is not defined. Options:
- Auto-play the Handmaid on timeout (safe, always valid)
- `PenaltyLoseGame` — harsher but simpler
- Auto-play the lower-value card on timeout (minimises information leak)

Recommended: auto-play Handmaid if held, otherwise auto-play the lower-value
card. This requires the engine to expose a `DefaultMove(state, playerID) Move`
method, or handle it in the runtime timeout callback.

### 13. Spectator state

Spectators should see:
- All discard piles, deck count, token counts, protected status, spy tracking
- But NOT any player's hand or Chancellor choices

Same per-player filtering as point 1, with spectators receiving a view where
all hands are empty and `chancellor_choices` is absent.

---

## Suggested implementation order

1. **Define `FilterState` on `engine.Game`** — add optional interface method;
   TicTacToe returns state unchanged; Love Letter filters hands and Chancellor choices
2. **Update runtime broadcast** — call `FilterState` per player before
   `BroadcastToPlayer`; fall back to `Broadcast` for games without private state
3. **Implement Love Letter engine** — `Init`, `ValidateMove`, `ApplyMove`, `IsOver`
   with full round management, token tracking, Spy end-of-round check, and
   Chancellor two-step resolution
4. **Add `GameRenderer` registry** — replace the `switch` in `GameRenderer`
   component with a `Record<string, React.FC<RendererProps>>` registry
5. **Build Love Letter renderer** — hand, discard piles, card picker, target picker,
   Chancellor follow-up, round and game summary screens
6. **Pause/Resume UI** — Love Letter sessions span multiple rounds; players need
   pause support before this game is usable in practice

---

## Files to read before starting

### Backend
- `internal/domain/engine/engine.go` — `Game` interface, `GameState`, `Move`, `Result`
- `internal/domain/runtime/runtime.go` — `ApplyMove`, broadcast logic, timeout handling
- `games/tictactoe/tictactoe.go` — reference engine implementation
- `internal/platform/ws/hub.go` — `Broadcast` vs `BroadcastToPlayer`
- `internal/platform/ws/hub_player.go` — per-player broadcast implementation

### Frontend
- `src/components/TicTacToe.tsx` — reference renderer
- `src/pages/Game.tsx` — `GameRenderer` switch, WS event handlers, move mutation
- `src/ws.ts` — `WsPayloadMoveResult` (what the frontend expects from move/game_over)