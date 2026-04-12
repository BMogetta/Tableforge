// Package rootaccess provides a BotAdapter implementation for Root Access.
//
// # Determinization
//
// Root Access is a game of imperfect information — each player can only see
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
package rootaccess

import (
	"math/rand"

	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/shared/platform/randutil"
)

// Adapter implements bot.BotAdapter for Root Access.
//
// When personality has non-zero Aggressiveness or RiskAversion, RolloutPolicy
// switches from uniform random to an epsilon-greedy scoring rule that biases
// rollouts toward the configured playstyle. Zero-valued personality (the
// default) preserves legacy behaviour.
type Adapter struct {
	personality bot.PersonalityProfile
}

// New returns a Root Access BotAdapter with no personality (random rollouts).
// Used by tests and code paths that do not care about bot strength tuning.
func New() *Adapter { return &Adapter{} }

// NewWithProfile returns a Root Access BotAdapter whose rollout policy is
// biased by the given personality profile. Pass bot.Profiles["medium"] (or any
// named profile) for a configured bot; pass a zero PersonalityProfile to get
// the same behaviour as New().
func NewWithProfile(profile bot.PersonalityProfile) *Adapter {
	return &Adapter{personality: profile}
}

// GameID implements bot.BotAdapter.
func (a *Adapter) GameID() string { return "rootaccess" }

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
// own hand + all discard piles + face-up set-aside + debugger choices if active.
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

	// Debugger choices visible only to the active player.
	if s.CurrentPlayerID == playerID {
		choices := getStringSlice(s.Data, "debugger_choices")
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
	case phaseDebuggerPending:
		return debuggerResolveMoves(s)
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

	mustPlayEncryptedKey := containsCard(hand, CardEncryptedKey) &&
		(containsCard(hand, CardSwap) || containsCard(hand, CardReboot))

	var moves []bot.BotMove
	for _, card := range hand {
		c := CardName(card)
		if mustPlayEncryptedKey && c != CardEncryptedKey {
			continue
		}

		switch c {
		case CardPing:
			if len(targets) == 0 {
				moves = append(moves, makeMoveNoTarget(c))
			} else {
				for _, t := range targets {
					for _, guess := range allNonPingCards {
						moves = append(moves, makeGuardMove(t, guess))
					}
				}
			}

		case CardSniffer, CardBufferOverflow, CardSwap:
			if len(targets) == 0 {
				moves = append(moves, makeMoveNoTarget(c))
			} else {
				for _, t := range targets {
					moves = append(moves, makeMoveWithTarget(c, t))
				}
			}

		case CardReboot:
			// Prince can self-target; include self and all non-eliminated opponents.
			allActive := append([]string{playerID}, targets...)
			for _, t := range allActive {
				moves = append(moves, makeMoveWithTarget(c, t))
			}

		default:
			// Spy, Firewall, Encrypted Key, Root, Debugger — no target needed.
			moves = append(moves, makeMoveNoTarget(c))
		}
	}

	if len(moves) == 0 && len(hand) > 0 {
		moves = append(moves, makeMoveNoTarget(CardName(hand[0])))
	}
	return moves
}

func debuggerResolveMoves(s engine.GameState) []bot.BotMove {
	choices := getStringSlice(s.Data, "debugger_choices")
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
			"card":   "debugger_resolve",
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
		"card":             string(CardPing),
		"target_player_id": target,
		"guess":            string(guess),
	})
	return m
}

// ---------------------------------------------------------------------------
// ApplyMove
// ---------------------------------------------------------------------------

// ApplyMove applies a bot move using the Root Access game engine.
func (a *Adapter) ApplyMove(state bot.BotGameState, move bot.BotMove) bot.BotGameState {
	s := state.EngineState()
	payload, err := move.Payload()
	if err != nil {
		return state
	}
	newState, err := (&RootAccess{}).ApplyMove(s, engine.Move{
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

// rolloutEpsilon is the probability of picking a uniformly random move during
// a personality-biased rollout. Keeping it non-zero preserves MCTS exploration
// breadth — a fully greedy rollout collapses the search to a single trajectory.
const rolloutEpsilon = 0.3

// RolloutPolicy selects a move for a simulation playout.
//
// With zero personality (default / tests) it behaves as uniform random —
// identical to mcts.RandomRolloutPolicy. With a configured personality it
// applies epsilon-greedy scoring: with probability rolloutEpsilon it picks
// random (to preserve MCTS breadth), otherwise it picks the highest-scoring
// move under scoreMove.
func (a *Adapter) RolloutPolicy(state bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	if a.personality.Aggressiveness == 0 && a.personality.RiskAversion == 0 {
		return mcts.RandomRolloutPolicy(state, moves)
	}

	if rand.Float64() < rolloutEpsilon {
		return moves[rand.Intn(len(moves))]
	}

	bestIdx := 0
	bestScore := a.scoreMove(moves[0])
	for i := 1; i < len(moves); i++ {
		if s := a.scoreMove(moves[i]); s > bestScore {
			bestScore = s
			bestIdx = i
		}
	}
	return moves[bestIdx]
}

// scoreMove ranks a single move under the current personality.
//
// Two weights interact:
//   - Aggressiveness boosts target-requiring moves (attacks) and penalises
//     targetless plays. 1.0 = always pick attacks when available.
//   - RiskAversion rewards playing Firewall (defence) and penalises playing
//     Encrypted Key (which forces a discard when paired with Swap / Reboot).
//
// Scores are unbounded in principle but kept in roughly [0, 1] for the built-in
// profiles. Callers only consume relative ordering.
func (a *Adapter) scoreMove(m bot.BotMove) float64 {
	payload, err := m.Payload()
	if err != nil {
		return 0
	}
	cardStr, _ := payload["card"].(string)
	_, hasTarget := payload["target_player_id"]

	score := 0.5
	if hasTarget {
		score += 0.5 * a.personality.Aggressiveness
	} else {
		score -= 0.3 * a.personality.Aggressiveness
	}
	switch CardName(cardStr) {
	case CardFirewall:
		score += 0.4 * a.personality.RiskAversion
	case CardEncryptedKey:
		score -= 0.4 * a.personality.RiskAversion
	}
	return score
}
