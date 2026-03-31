// Package loveletter provides a BotAdapter implementation for Love Letter.
//
// # Determinization
//
// Love Letter is a game of imperfect information — each player can only see
// their own hand. MCTS requires complete information to evaluate moves, so
// before each search we determinize the hidden state:
//
//  1. Collect all cards visible to the bot: own hand, all discard piles,
//     set-aside face-up cards (2-player variant).
//  2. Compute the unknown pool: full deck composition minus all visible cards.
//  3. Sample without replacement from the unknown pool to assign one card to
//     each active opponent's hand.
//  4. Remaining unknown cards become the determinized deck in shuffled order.
//
// Running multiple MCTS searches over independently sampled worlds (IS-MCTS)
// averages out the uncertainty.
package loveletter

import (
	"github.com/tableforge/server/internal/bot"
	"github.com/tableforge/server/internal/bot/mcts"
	"github.com/tableforge/server/internal/domain/engine"
	"github.com/tableforge/shared/platform/randutil"
)

// Adapter implements bot.BotAdapter for Love Letter.
type Adapter struct{}

// New returns a new Love Letter BotAdapter.
func New() *Adapter { return &Adapter{} }

// GameID implements bot.BotAdapter.
func (a *Adapter) GameID() string { return "loveletter" }

func (a *Adapter) CurrentPlayer(s bot.BotGameState) engine.PlayerID {
	return s.EngineState().CurrentPlayerID
}

// ---------------------------------------------------------------------------
// Determinize
// ---------------------------------------------------------------------------

// Determinize replaces hidden information (opponent hands, deck order) with a
// plausible sample consistent with the bot's observations.
func (a *Adapter) Determinize(state bot.BotGameState, playerID engine.PlayerID) bot.BotGameState {
	s := state.EngineState()

	visible := collectVisible(s, playerID)
	pool := buildUnknownPool(visible)
	shuffled, err := randutil.Shuffle(pool)
	if err != nil {
		shuffled = pool
	}
	pool = shuffled

	// Work on a copy of the engine state — EngineState() returns by value
	// so we can mutate freely and wrap in a new BotGameState at the end.
	cs := state.Clone().EngineState()

	players := getStringSlice(cs.Data, "players")
	eliminated := getStringSlice(cs.Data, "eliminated")
	hands := getHands(cs.Data)

	for _, pid := range players {
		if pid == string(playerID) {
			continue
		}
		if isEliminated(pid, eliminated) {
			continue
		}
		if len(pool) == 0 {
			break
		}
		hands[pid] = []string{pool[0]}
		pool = pool[1:]
	}
	cs.Data["hands"] = handsToAny(hands)
	cs.Data["deck"] = pool

	// Wrap the mutated copy in a new BotGameState.
	return bot.NewBotState(cs)
}

// collectVisible returns all cards observable by playerID:
// own hand + all discard piles + face-up set-aside + chancellor choices if active.
func collectVisible(s engine.GameState, playerID engine.PlayerID) []string {
	var visible []string

	hands := getHands(s.Data)
	if hand, ok := hands[string(playerID)]; ok {
		visible = append(visible, hand...)
	}

	discards := getDiscardPiles(s.Data)
	for _, pile := range discards {
		visible = append(visible, pile...)
	}

	faceUp := getStringSlice(s.Data, "set_aside_visible")
	visible = append(visible, faceUp...)

	// Chancellor choices visible only to the active player.
	if s.CurrentPlayerID == playerID {
		choices := getStringSlice(s.Data, "chancellor_choices")
		visible = append(visible, choices...)
	}

	return visible
}

// buildUnknownPool returns the multiset of cards not yet accounted for in
// the visible set. Uses the full 21-card deckComposition as the universe.
func buildUnknownPool(visible []string) []string {
	counts := make(map[string]int, len(deckComposition))
	for _, c := range deckComposition {
		counts[string(c)]++
	}
	for _, c := range visible {
		if counts[c] > 0 {
			counts[c]--
		}
	}
	pool := make([]string, 0, len(deckComposition))
	for _, c := range deckComposition {
		key := string(c)
		if counts[key] > 0 {
			pool = append(pool, key)
			counts[key]--
		}
	}
	return pool
}

func isEliminated(playerID string, eliminated []string) bool {
	for _, e := range eliminated {
		if e == playerID {
			return true
		}
	}
	return false
}

func isProtected(playerID string, protected []string) bool {
	for _, p := range protected {
		if p == playerID {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ValidMoves
// ---------------------------------------------------------------------------

// ValidMoves returns all legal moves for the current player.
func (a *Adapter) ValidMoves(state bot.BotGameState) []bot.BotMove {
	s := state.EngineState()
	switch getString(s.Data, "phase") {
	case phaseChancellorPending:
		return chancellorResolveMoves(s)
	case phasePlaying:
		return standardMoves(s)
	default:
		return nil
	}
}

func standardMoves(s engine.GameState) []bot.BotMove {
	playerID := string(s.CurrentPlayerID)
	hands := getHands(s.Data)
	hand := hands[playerID]

	players := getStringSlice(s.Data, "players")
	eliminated := getStringSlice(s.Data, "eliminated")
	protected := getStringSlice(s.Data, "protected")

	var targets []string
	for _, pid := range players {
		if pid == playerID {
			continue
		}
		if isEliminated(pid, eliminated) || isProtected(pid, protected) {
			continue
		}
		targets = append(targets, pid)
	}

	mustPlayCountess := containsCard(hand, CardCountess) &&
		(containsCard(hand, CardKing) || containsCard(hand, CardPrince))

	var moves []bot.BotMove
	for _, card := range hand {
		c := CardName(card)
		if mustPlayCountess && c != CardCountess {
			continue
		}

		switch c {
		case CardGuard:
			if len(targets) == 0 {
				moves = append(moves, makeMoveNoTarget(c))
			} else {
				for _, t := range targets {
					for _, guess := range allNonGuardCards {
						moves = append(moves, makeGuardMove(t, guess))
					}
				}
			}

		case CardPriest, CardBaron, CardKing:
			if len(targets) == 0 {
				moves = append(moves, makeMoveNoTarget(c))
			} else {
				for _, t := range targets {
					moves = append(moves, makeMoveWithTarget(c, t))
				}
			}

		case CardPrince:
			// Prince can self-target; include self and all non-eliminated opponents.
			allActive := append([]string{playerID}, targets...)
			for _, t := range allActive {
				moves = append(moves, makeMoveWithTarget(c, t))
			}

		default:
			// Spy, Handmaid, Countess, Princess, Chancellor — no target needed.
			moves = append(moves, makeMoveNoTarget(c))
		}
	}

	if len(moves) == 0 && len(hand) > 0 {
		moves = append(moves, makeMoveNoTarget(CardName(hand[0])))
	}
	return moves
}

func chancellorResolveMoves(s engine.GameState) []bot.BotMove {
	choices := getStringSlice(s.Data, "chancellor_choices")
	if len(choices) != 3 {
		return nil
	}
	var moves []bot.BotMove
	for i, keep := range choices {
		ret := make([]string, 0, 2)
		for j, c := range choices {
			if j != i {
				ret = append(ret, c)
			}
		}
		m, err := bot.MoveFromPayload(map[string]any{
			"card":   "chancellor_resolve",
			"keep":   keep,
			"return": ret,
		})
		if err == nil {
			moves = append(moves, m)
		}
	}
	return moves
}

func makeMoveNoTarget(card CardName) bot.BotMove {
	m, _ := bot.MoveFromPayload(map[string]any{"card": string(card)})
	return m
}

func makeMoveWithTarget(card CardName, target string) bot.BotMove {
	m, _ := bot.MoveFromPayload(map[string]any{
		"card":             string(card),
		"target_player_id": target,
	})
	return m
}

func makeGuardMove(target string, guess CardName) bot.BotMove {
	m, _ := bot.MoveFromPayload(map[string]any{
		"card":             string(CardGuard),
		"target_player_id": target,
		"guess":            string(guess),
	})
	return m
}

// ---------------------------------------------------------------------------
// ApplyMove
// ---------------------------------------------------------------------------

// ApplyMove applies a bot move using the Love Letter game engine.
func (a *Adapter) ApplyMove(state bot.BotGameState, move bot.BotMove) bot.BotGameState {
	s := state.EngineState()
	payload, err := move.Payload()
	if err != nil {
		return state
	}
	newState, err := (&LoveLetter{}).ApplyMove(s, engine.Move{
		PlayerID: s.CurrentPlayerID,
		Payload:  payload,
	})
	if err != nil {
		return state
	}
	return bot.NewBotState(newState)
}

// ---------------------------------------------------------------------------
// IsTerminal
// ---------------------------------------------------------------------------

// IsTerminal returns true when the game phase is game_over.
func (a *Adapter) IsTerminal(state bot.BotGameState) bool {
	return getString(state.EngineState().Data, "phase") == phaseGameOver
}

// ---------------------------------------------------------------------------
// Result
// ---------------------------------------------------------------------------

// Result returns a value in [0, 1] reflecting how good the state is for playerID.
//
// Terminal: 1.0 win, 0.0 loss, 0.5 draw.
// Non-terminal: weighted combination of token progress (0–0.7),
// survival bonus (0.1), and hand value heuristic (0–0.2).
func (a *Adapter) Result(state bot.BotGameState, playerID engine.PlayerID) float64 {
	s := state.EngineState()
	pid := string(playerID)
	players := getStringSlice(s.Data, "players")
	target := float64(tokensToWin(len(players)))
	tokens := getTokens(s.Data)

	if getString(s.Data, "phase") == phaseGameOver {
		winner := getString(s.Data, "game_winner_id")
		if winner == "" {
			return 0.5
		}
		if winner == pid {
			return 1.0
		}
		return 0.0
	}

	tokenResult := (float64(tokens[pid]) / target) * 0.7

	if isEliminated(pid, getStringSlice(s.Data, "eliminated")) {
		return tokenResult
	}

	score := tokenResult + 0.1

	hands := getHands(s.Data)
	if hand := hands[pid]; len(hand) > 0 {
		maxVal := 0
		for _, c := range hand {
			if v := cardValue(CardName(c)); v > maxVal {
				maxVal = v
			}
		}
		score += (float64(maxVal) / 9.0) * 0.2
	}

	return score
}

// ---------------------------------------------------------------------------
// RolloutPolicy
// ---------------------------------------------------------------------------

// RolloutPolicy delegates to the MCTS random rollout policy.
func (a *Adapter) RolloutPolicy(state bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	return mcts.RandomRolloutPolicy(state, moves)
}
