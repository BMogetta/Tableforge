package loveletter

import (
	"errors"
	"fmt"

	"github.com/tableforge/server/games"
	"github.com/tableforge/server/internal/domain/engine"
	"github.com/tableforge/shared/platform/randutil"
)

func init() {
	games.Register(&LoveLetter{})
}

const gameID = "loveletter"

// Phase constants for the game phase field in state Data.
const (
	phasePlaying           = "playing"
	phaseChancellorPending = "chancellor_pending"
	phaseRoundOver         = "round_over"
	phaseGameOver          = "game_over"
)

// LoveLetter implements engine.Game and engine.StateFilter.
type LoveLetter struct{}

func (g *LoveLetter) ID() string      { return gameID }
func (g *LoveLetter) Name() string    { return "Love Letter" }
func (g *LoveLetter) MinPlayers() int { return 2 }
func (g *LoveLetter) MaxPlayers() int { return 5 }

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

// Init sets up the first round of a new Love Letter game.
func (g *LoveLetter) Init(players []engine.Player) (engine.GameState, error) {
	if len(players) < 2 || len(players) > 5 {
		return engine.GameState{}, fmt.Errorf("loveletter requires 2–5 players, got %d", len(players))
	}

	playerIDs := make([]string, len(players))
	for i, p := range players {
		playerIDs[i] = string(p.ID)
	}

	tokens := make(map[string]int, len(players))
	for _, id := range playerIDs {
		tokens[id] = 0
	}

	state := engine.GameState{
		CurrentPlayerID: players[0].ID,
		Data:            map[string]any{},
	}

	state.Data["players"] = playerIDs
	state.Data["tokens"] = tokens
	state.Data["round"] = 1
	state.Data["phase"] = phasePlaying

	state = dealRound(state, playerIDs, players[0].ID)
	return state, nil
}

// ---------------------------------------------------------------------------
// ValidateMove
// ---------------------------------------------------------------------------

func (g *LoveLetter) ValidateMove(state engine.GameState, move engine.Move) error {
	phase := getString(state.Data, "phase")

	// Penalty move is always valid when issued by the timer (any phase).
	if getCardFromPayload(move.Payload, "card") == CardPenaltyLose {
		return nil
	}

	if move.PlayerID != state.CurrentPlayerID {
		return errors.New("it is not your turn")
	}

	eliminated := getStringSlice(state.Data, "eliminated")
	for _, id := range eliminated {
		if id == string(move.PlayerID) {
			return errors.New("you have been eliminated this round")
		}
	}

	switch phase {
	case phaseChancellorPending:
		return validateChancellorResolve(state, move)
	case phasePlaying:
		return validateStandardMove(state, move)
	default:
		return fmt.Errorf("no moves allowed in phase %q", phase)
	}
}

func validateStandardMove(state engine.GameState, move engine.Move) error {
	card := getCardFromPayload(move.Payload, "card")
	if card == "" {
		return errors.New("move payload must contain 'card'")
	}

	hands := getHands(state.Data)
	hand, ok := hands[string(move.PlayerID)]
	if !ok || len(hand) == 0 {
		return errors.New("player has no cards in hand")
	}

	// Player must hold exactly 2 cards at play time (drew before playing).
	// The hand stored in state is already the post-draw hand (2 cards).
	if !containsCard(hand, card) {
		return fmt.Errorf("card %q is not in your hand", card)
	}

	// Countess must be played if the player holds Countess together with
	// King or Prince. Only enforced when Countess is actually in hand.
	if card != CardCountess && containsCard(hand, CardCountess) {
		if containsCard(hand, CardKing) || containsCard(hand, CardPrince) {
			return errors.New("you must play the Countess when holding a King or Prince")
		}
	}

	// Cards that require a target.
	target := getStringFromPayload(move.Payload, "target_player_id")
	switch card {
	case CardGuard:
		if err := validateTarget(state, move.PlayerID, target, false); err != nil {
			return err
		}
		guess := getCardFromPayload(move.Payload, "guess")
		if guess == "" {
			return errors.New("guard requires a 'guess' card name")
		}
		if !isValidGuess(guess) {
			return fmt.Errorf("invalid guard guess: %q", guess)
		}
	case CardPriest, CardBaron, CardKing:
		if err := validateTarget(state, move.PlayerID, target, false); err != nil {
			return err
		}
	case CardPrince:
		// Prince can target self, so we allow empty target (defaults to self)
		// or any non-eliminated player including self.
		if target != "" && target != string(move.PlayerID) {
			if err := validateTarget(state, move.PlayerID, target, false); err != nil {
				return err
			}
		}
	case CardSpy, CardHandmaid, CardCountess, CardPrincess, CardChancellor:
		// No target required.
	}

	return nil
}

func validateChancellorResolve(state engine.GameState, move engine.Move) error {
	card := getCardFromPayload(move.Payload, "card")
	if card != "chancellor_resolve" {
		return errors.New("waiting for chancellor_resolve move")
	}
	keep := getCardFromPayload(move.Payload, "keep")
	if keep == "" {
		return errors.New("chancellor_resolve requires 'keep'")
	}
	ret := getCardSliceFromPayload(move.Payload, "return")
	if len(ret) != 2 {
		return errors.New("chancellor_resolve requires exactly 2 cards in 'return'")
	}
	choices := getStringSlice(state.Data, "chancellor_choices")
	all := append([]string{string(keep)}, cardNamesToStrings(ret)...)
	if len(choices) != 3 {
		return errors.New("chancellor_choices state is invalid")
	}
	for _, c := range all {
		if !containsString(choices, c) {
			return fmt.Errorf("card %q is not among chancellor choices", c)
		}
	}
	return nil
}

// validateTarget checks that target is a valid, non-protected, non-eliminated
// opponent. If allowSelf is true, the acting player is also a valid target.
func validateTarget(state engine.GameState, actorID engine.PlayerID, target string, allowSelf bool) error {
	if target == "" {
		// Check if there is any valid target available; if not, the card
		// resolves with no effect and the move is still legal.
		return nil
	}
	eliminated := getStringSlice(state.Data, "eliminated")
	protected := getStringSlice(state.Data, "protected")
	players := getStringSlice(state.Data, "players")

	found := false
	for _, id := range players {
		if id == target {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("target player %q not found", target)
	}
	if !allowSelf && target == string(actorID) {
		return errors.New("cannot target yourself with this card")
	}
	for _, id := range eliminated {
		if id == target {
			return fmt.Errorf("target player %q is eliminated", target)
		}
	}
	for _, id := range protected {
		if id == target {
			return fmt.Errorf("target player %q is protected by the Handmaid", target)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ApplyMove
// ---------------------------------------------------------------------------

func (g *LoveLetter) ApplyMove(state engine.GameState, move engine.Move) (engine.GameState, error) {
	card := getCardFromPayload(move.Payload, "card")

	// Penalty: eliminate the active player immediately.
	if card == CardPenaltyLose {
		return applyPenaltyLose(state, move.PlayerID)
	}

	phase := getString(state.Data, "phase")

	if phase == phaseChancellorPending {
		return applyChancellorResolve(state, move)
	}

	return applyStandardMove(state, move, card)
}

func applyStandardMove(state engine.GameState, move engine.Move, card CardName) (engine.GameState, error) {
	playerID := string(move.PlayerID)

	// Clear private reveals from the previous turn before applying this move.
	delete(state.Data, "private_reveals")

	// Remove the played card from hand.
	state = removeFromHand(state, playerID, card)

	// Add card to discard pile.
	state = appendDiscard(state, playerID, card)

	// Princess self-elimination.
	if card == CardPrincess {
		return applyPrincess(state, playerID)
	}

	target := getStringFromPayload(move.Payload, "target_player_id")
	if target == "" && card == CardPrince {
		target = playerID // default self-target for Prince
	}

	var err error
	switch card {
	case CardSpy:
		state = applySpy(state, playerID)
	case CardGuard:
		guess := getCardFromPayload(move.Payload, "guess")
		state = applyGuard(state, target, guess)
	case CardPriest:
		// Effect is informational only — no state change needed beyond
		// the private_reveal field set below.
		state = applyPriest(state, playerID, target)
	case CardBaron:
		state = applyBaron(state, playerID, target)
	case CardHandmaid:
		state = applyHandmaid(state, playerID)
	case CardPrince:
		state, err = applyPrince(state, target)
		if err != nil {
			return state, err
		}
	case CardChancellor:
		return applyChancellor(state, playerID)
	case CardKing:
		state = applyKing(state, playerID, target)
	case CardCountess:
		// No effect beyond being played.
	default:
		// Unknown card — this should never happen in a valid game but guards
		// against corrupted payloads reaching the engine.
		return state, fmt.Errorf("loveletter: unknown card %q", card)
	}

	_ = err

	return advanceTurn(state)
}

// ---------------------------------------------------------------------------
// Card effects
// ---------------------------------------------------------------------------

func applySpy(state engine.GameState, playerID string) engine.GameState {
	spyPlayed := getStringSlice(state.Data, "spy_played_by")
	state.Data["spy_played_by"] = appendUnique(spyPlayed, playerID)
	return state
}

func applyGuard(state engine.GameState, target string, guess CardName) engine.GameState {
	if target == "" {
		return state // no valid target — no effect
	}
	hands := getHands(state.Data)
	hand := hands[target]
	if len(hand) > 0 && CardName(hand[0]) == guess {
		state = eliminatePlayer(state, target)
	}
	return state
}

func applyPriest(state engine.GameState, actorID, target string) engine.GameState {
	if target == "" {
		return state
	}
	hands := getHands(state.Data)
	hand := hands[target]
	revealed := ""
	if len(hand) > 0 {
		revealed = hand[0]
	}
	// Store as private_reveal keyed by actor so FilterState can expose it only
	// to the actor.
	reveals, _ := state.Data["private_reveals"].(map[string]any)
	if reveals == nil {
		reveals = map[string]any{}
	}
	reveals[actorID] = revealed
	state.Data["private_reveals"] = reveals
	return state
}

func applyBaron(state engine.GameState, actorID, target string) engine.GameState {
	if target == "" {
		return state
	}
	hands := getHands(state.Data)
	actorHand := hands[actorID]
	targetHand := hands[target]
	if len(actorHand) == 0 || len(targetHand) == 0 {
		return state
	}
	actorVal := cardValue(CardName(actorHand[0]))
	targetVal := cardValue(CardName(targetHand[0]))
	if actorVal < targetVal {
		state = eliminatePlayer(state, actorID)
	} else if targetVal < actorVal {
		state = eliminatePlayer(state, target)
	}
	// Tie: nothing happens.
	return state
}

func applyHandmaid(state engine.GameState, playerID string) engine.GameState {
	protected := getStringSlice(state.Data, "protected")
	state.Data["protected"] = appendUnique(protected, playerID)
	return state
}

func applyPrince(state engine.GameState, target string) (engine.GameState, error) {
	if target == "" {
		return state, nil
	}
	hands := getHands(state.Data)
	hand := hands[target]
	if len(hand) == 0 {
		return state, nil
	}
	discarded := hand[0]
	// Discard the target's hand.
	state = appendDiscard(state, target, CardName(discarded))
	hands[target] = []string{}
	state.Data["hands"] = handsToAny(hands)

	// If discarded card is Princess, target is eliminated.
	if CardName(discarded) == CardPrincess {
		state = eliminatePlayer(state, target)
		return state, nil
	}

	// Draw a new card for target.
	deck := getStringSlice(state.Data, "deck")
	if len(deck) == 0 {
		// Deck empty: use the face-down set-aside card.
		setAside := getString(state.Data, "set_aside_face_down")
		if setAside != "" {
			hands[target] = []string{setAside}
			state.Data["set_aside_face_down"] = ""
			state.Data["hands"] = handsToAny(hands)
		}
		return state, nil
	}
	newCard := deck[0]
	deck = deck[1:]
	state.Data["deck"] = deck
	hands[target] = []string{newCard}
	state.Data["hands"] = handsToAny(hands)
	return state, nil
}

func applyChancellor(state engine.GameState, playerID string) (engine.GameState, error) {
	hands := getHands(state.Data)
	hand := hands[playerID]
	deck := getStringSlice(state.Data, "deck")

	// Draw up to 2 cards.
	drawn := []string{}
	for i := 0; i < 2 && len(deck) > 0; i++ {
		drawn = append(drawn, deck[0])
		deck = deck[1:]
	}
	state.Data["deck"] = deck

	// Chancellor choices = current hand (1 card) + drawn cards.
	choices := append(hand, drawn...)
	state.Data["chancellor_choices"] = choices

	// Clear hand until resolve.
	hands[playerID] = []string{}
	state.Data["hands"] = handsToAny(hands)
	state.Data["phase"] = phaseChancellorPending

	// CurrentPlayerID stays the same — waiting for chancellor_resolve.
	return state, nil
}

func applyChancellorResolve(state engine.GameState, move engine.Move) (engine.GameState, error) {
	playerID := string(move.PlayerID)
	keep := getCardFromPayload(move.Payload, "keep")
	ret := getCardSliceFromPayload(move.Payload, "return")

	hands := getHands(state.Data)
	hands[playerID] = []string{string(keep)}
	state.Data["hands"] = handsToAny(hands)

	// Place the 2 returned cards at the bottom of the deck in given order.
	deck := getStringSlice(state.Data, "deck")
	for _, c := range ret {
		deck = append(deck, string(c))
	}
	state.Data["deck"] = deck
	delete(state.Data, "chancellor_choices")
	state.Data["phase"] = phasePlaying

	return advanceTurn(state)
}

func applyKing(state engine.GameState, actorID, target string) engine.GameState {
	if target == "" {
		return state
	}
	hands := getHands(state.Data)
	actorHand := hands[actorID]
	targetHand := hands[target]
	hands[actorID] = targetHand
	hands[target] = actorHand
	state.Data["hands"] = handsToAny(hands)
	return state
}

func applyPrincess(state engine.GameState, playerID string) (engine.GameState, error) {
	state = eliminatePlayer(state, playerID)
	return advanceTurn(state)
}

func applyPenaltyLose(state engine.GameState, playerID engine.PlayerID) (engine.GameState, error) {
	state = eliminatePlayer(state, string(playerID))
	return advanceTurn(state)
}

// ---------------------------------------------------------------------------
// Turn advancement and round resolution
// ---------------------------------------------------------------------------

// advanceTurn expires Handmaid protection for the next player (it expires
// when their turn begins), checks whether the round is over, advances to
// the next active player, and draws their turn card if needed.
func advanceTurn(state engine.GameState) (engine.GameState, error) {
	currentPlayer := string(state.CurrentPlayerID)

	active := activePlayers(state)

	// Round over if one or zero players remain.
	if len(active) <= 1 {
		return resolveRound(state)
	}

	// Determine next active player.
	next := nextActivePlayer(state, currentPlayer)

	// Handmaid protection expires when the protected player's turn begins.
	protected := getStringSlice(state.Data, "protected")
	newProtected := []string{}
	for _, id := range protected {
		if id != next {
			newProtected = append(newProtected, id)
		}
	}
	state.Data["protected"] = newProtected

	// Advance turn.
	state.CurrentPlayerID = engine.PlayerID(next)

	// Draw turn card if the next player holds fewer than 2 cards.
	// Prince may have already dealt them a card directly.
	hands := getHands(state.Data)
	if len(hands[next]) < 2 {
		var err error
		state, _, err = drawCard(state, next)
		if err != nil {
			return resolveRound(state)
		}
	}
	return state, nil
}

// resolveRound determines the round winner, awards tokens (including Spy
// bonus), and either starts a new round or sets phase to game_over.
func resolveRound(state engine.GameState) (engine.GameState, error) {
	players := getStringSlice(state.Data, "players")
	active := activePlayers(state)
	tokens := getTokens(state.Data)

	var roundWinner string

	if len(active) == 1 {
		roundWinner = active[0]
	} else {
		// Deck empty — highest hand wins; tie goes to most discarded value.
		roundWinner = highestHandWinner(state, active)
	}

	// Spy token: if exactly one player played or discarded a Spy, they earn
	// an extra token — even if eliminated.
	spyPlayed := getStringSlice(state.Data, "spy_played_by")
	if len(spyPlayed) == 1 {
		tokens[spyPlayed[0]]++
	}

	// Round winner token.
	if roundWinner != "" {
		tokens[roundWinner]++
	}

	state.Data["tokens"] = tokensToAny(tokens)
	state.Data["round_winner_id"] = roundWinner

	// Check game over.
	target := tokensToWin(len(players))
	for _, id := range players {
		if tokens[id] >= target {
			state.Data["game_winner_id"] = id
			state.Data["phase"] = phaseGameOver
			state.CurrentPlayerID = engine.PlayerID(id)
			return state, nil
		}
	}

	// Start next round.
	round := getInt(state.Data, "round")
	state.Data["round"] = round + 1

	// The round winner goes first next round.
	firstPlayer := roundWinner
	if firstPlayer == "" {
		firstPlayer = players[0]
	}

	state = dealRound(state, players, engine.PlayerID(firstPlayer))
	return state, nil
}

// highestHandWinner returns the player with the highest card value among
// active players. Ties are broken by total discard pile value.
func highestHandWinner(state engine.GameState, active []string) string {
	hands := getHands(state.Data)
	discards := getDiscardPiles(state.Data)

	bestPlayer := ""
	bestHandVal := -1
	bestDiscardVal := -1

	for _, id := range active {
		hand := hands[id]
		handVal := 0
		if len(hand) > 0 {
			handVal = cardValue(CardName(hand[0]))
		}
		discardVal := 0
		for _, c := range discards[id] {
			discardVal += cardValue(CardName(c))
		}
		if handVal > bestHandVal || (handVal == bestHandVal && discardVal > bestDiscardVal) {
			bestPlayer = id
			bestHandVal = handVal
			bestDiscardVal = discardVal
		}
	}
	return bestPlayer
}

// TimeoutMove implements engine.TurnTimeoutHandler.
// Returns a penalty_lose move that eliminates the active player via the engine.
func (g *LoveLetter) TimeoutMove() map[string]any {
	return map[string]any{"card": string(CardPenaltyLose)}
}

// ---------------------------------------------------------------------------
// IsOver
// ---------------------------------------------------------------------------

func (g *LoveLetter) IsOver(state engine.GameState) (bool, engine.Result) {
	if getString(state.Data, "phase") != phaseGameOver {
		return false, engine.Result{}
	}
	winnerID := getString(state.Data, "game_winner_id")
	if winnerID == "" {
		return true, engine.Result{Status: engine.ResultDraw}
	}
	pid := engine.PlayerID(winnerID)
	return true, engine.Result{
		Status:   engine.ResultWin,
		WinnerID: &pid,
	}
}

// ---------------------------------------------------------------------------
// FilterState
// ---------------------------------------------------------------------------

func (g *LoveLetter) FilterState(state engine.GameState, playerID engine.PlayerID) engine.GameState {
	filtered := engine.GameState{
		CurrentPlayerID: state.CurrentPlayerID,
		Data:            shallowCopyData(state.Data),
	}

	hands := getHands(state.Data)
	filteredHands := map[string][]string{}

	if playerID == "" {
		// Spectator: empty hands for all players.
		for id := range hands {
			filteredHands[id] = []string{}
		}
	} else {
		// Player: only their own hand.
		for id, hand := range hands {
			if id == string(playerID) {
				filteredHands[id] = hand
			} else {
				filteredHands[id] = []string{}
			}
		}
	}
	filtered.Data["hands"] = handsToAny(filteredHands)

	// chancellor_choices only visible to the active player.
	if playerID == "" || playerID != state.CurrentPlayerID {
		delete(filtered.Data, "chancellor_choices")
	}

	// private_reveals: only show the reveal addressed to this player.
	if playerID == "" {
		delete(filtered.Data, "private_reveals")
	} else {
		reveals, _ := state.Data["private_reveals"].(map[string]any)
		if reveals != nil {
			myReveal, ok := reveals[string(playerID)]
			if ok {
				filtered.Data["private_reveals"] = map[string]any{
					string(playerID): myReveal,
				}
			} else {
				delete(filtered.Data, "private_reveals")
			}
		}
	}

	return filtered
}

// ---------------------------------------------------------------------------
// Round setup helpers
// ---------------------------------------------------------------------------

func dealRound(state engine.GameState, players []string, firstPlayer engine.PlayerID) engine.GameState {
	unshuffled := buildDeck()
	deck, err := randutil.Shuffle(unshuffled)
	if err != nil {
		// Fallback: use unshuffled deck rather than failing Init.
		// crypto/rand failure is extremely rare and non-recoverable.
		deck = unshuffled
	}

	// Set aside one card face-down.
	faceDown := deck[0]
	deck = deck[1:]

	// 2-player variant: set aside 3 more cards face-up.
	faceUp := []string{}
	if len(players) == 2 {
		for i := 0; i < 3; i++ {
			faceUp = append(faceUp, string(deck[0]))
			deck = deck[1:]
		}
	}

	// Deal one card to each player.
	hands := map[string][]string{}
	for _, id := range players {
		hands[id] = []string{string(deck[0])}
		deck = deck[1:]
	}

	state.Data["deck"] = cardNamesToStrings(deck)
	state.Data["hands"] = handsToAny(hands)
	state.Data["set_aside_face_down"] = string(faceDown)
	state.Data["set_aside_visible"] = faceUp
	state.Data["eliminated"] = []string{}
	state.Data["protected"] = []string{}
	state.Data["spy_played_by"] = []string{}
	state.Data["discard_piles"] = map[string]any{}
	state.Data["round_winner_id"] = nil

	state.Data["phase"] = phasePlaying
	state.CurrentPlayerID = firstPlayer

	// Draw the starting card for the first player so they hold 2 cards
	// on their first turn. Subsequent players draw in advanceTurn.
	state, _, _ = drawCard(state, string(firstPlayer))
	return state
}

// ---------------------------------------------------------------------------
// State mutation helpers
// ---------------------------------------------------------------------------

func drawCard(state engine.GameState, playerID string) (engine.GameState, CardName, error) {
	deck := getStringSlice(state.Data, "deck")
	if len(deck) == 0 {
		return state, "", errors.New("deck is empty")
	}
	card := CardName(deck[0])
	deck = deck[1:]
	state.Data["deck"] = deck

	hands := getHands(state.Data)
	hands[playerID] = append(hands[playerID], string(card))
	state.Data["hands"] = handsToAny(hands)
	return state, card, nil
}

func removeFromHand(state engine.GameState, playerID string, card CardName) engine.GameState {
	hands := getHands(state.Data)
	hand := hands[playerID]
	newHand := []string{}
	removed := false
	for _, c := range hand {
		if !removed && c == string(card) {
			removed = true
			continue
		}
		newHand = append(newHand, c)
	}
	hands[playerID] = newHand
	state.Data["hands"] = handsToAny(hands)
	return state
}

func appendDiscard(state engine.GameState, playerID string, card CardName) engine.GameState {
	piles := getDiscardPiles(state.Data)
	piles[playerID] = append(piles[playerID], string(card))
	state.Data["discard_piles"] = discardPilesToAny(piles)
	return state
}

func eliminatePlayer(state engine.GameState, playerID string) engine.GameState {
	eliminated := getStringSlice(state.Data, "eliminated")
	state.Data["eliminated"] = appendUnique(eliminated, playerID)

	// Move remaining hand to discard.
	hands := getHands(state.Data)
	hand := hands[playerID]
	for _, c := range hand {
		state = appendDiscard(state, playerID, CardName(c))
	}
	hands[playerID] = []string{}
	state.Data["hands"] = handsToAny(hands)

	// Track Spy if it was discarded due to elimination.
	for _, c := range hand {
		if CardName(c) == CardSpy {
			spyPlayed := getStringSlice(state.Data, "spy_played_by")
			state.Data["spy_played_by"] = appendUnique(spyPlayed, playerID)
		}
	}

	return state
}

func activePlayers(state engine.GameState) []string {
	players := getStringSlice(state.Data, "players")
	eliminated := getStringSlice(state.Data, "eliminated")
	active := []string{}
	for _, id := range players {
		elim := false
		for _, e := range eliminated {
			if e == id {
				elim = true
				break
			}
		}
		if !elim {
			active = append(active, id)
		}
	}
	return active
}

func nextActivePlayer(state engine.GameState, currentPlayer string) string {
	players := getStringSlice(state.Data, "players")
	active := activePlayers(state)
	if len(active) == 0 {
		return currentPlayer
	}
	// Find current index in full player list, then walk forward to next active.
	idx := 0
	for i, id := range players {
		if id == currentPlayer {
			idx = i
			break
		}
	}
	for i := 1; i <= len(players); i++ {
		candidate := players[(idx+i)%len(players)]
		for _, a := range active {
			if a == candidate {
				return candidate
			}
		}
	}
	return active[0]
}

// ---------------------------------------------------------------------------
// Data accessors (handle JSON round-trip type coercion)
// ---------------------------------------------------------------------------

func getString(data map[string]any, key string) string {
	v, _ := data[key].(string)
	return v
}

func getInt(data map[string]any, key string) int {
	switch v := data[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func getStringSlice(data map[string]any, key string) []string {
	raw, ok := data[key]
	if !ok {
		return []string{}
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return []string{}
}

func getHands(data map[string]any) map[string][]string {
	raw, ok := data["hands"]
	if !ok {
		return map[string][]string{}
	}
	switch v := raw.(type) {
	case map[string][]string:
		return v
	case map[string]any:
		out := make(map[string][]string, len(v))
		for k, val := range v {
			out[k] = toStringSlice(val)
		}
		return out
	}
	return map[string][]string{}
}

func getDiscardPiles(data map[string]any) map[string][]string {
	raw, ok := data["discard_piles"]
	if !ok {
		return map[string][]string{}
	}
	switch v := raw.(type) {
	case map[string][]string:
		return v
	case map[string]any:
		out := make(map[string][]string, len(v))
		for k, val := range v {
			out[k] = toStringSlice(val)
		}
		return out
	}
	return map[string][]string{}
}

func getTokens(data map[string]any) map[string]int {
	raw, ok := data["tokens"]
	if !ok {
		return map[string]int{}
	}
	switch v := raw.(type) {
	case map[string]int:
		return v
	case map[string]any:
		out := make(map[string]int, len(v))
		for k, val := range v {
			switch n := val.(type) {
			case int:
				out[k] = n
			case float64:
				out[k] = int(n)
			}
		}
		return out
	}
	return map[string]int{}
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return []string{}
}

func getStringFromPayload(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return v
}

func getCardFromPayload(payload map[string]any, key string) CardName {
	v, _ := payload[key].(string)
	return CardName(v)
}

func getCardSliceFromPayload(payload map[string]any, key string) []CardName {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []CardName:
		return v
	case []string:
		out := make([]CardName, len(v))
		for i, s := range v {
			out[i] = CardName(s)
		}
		return out
	case []any:
		out := make([]CardName, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, CardName(s))
			}
		}
		return out
	}
	return nil
}

// ---------------------------------------------------------------------------
// Serialization helpers
// ---------------------------------------------------------------------------

func handsToAny(hands map[string][]string) map[string]any {
	out := make(map[string]any, len(hands))
	for k, v := range hands {
		out[k] = v
	}
	return out
}

func discardPilesToAny(piles map[string][]string) map[string]any {
	out := make(map[string]any, len(piles))
	for k, v := range piles {
		out[k] = v
	}
	return out
}

func tokensToAny(tokens map[string]int) map[string]any {
	out := make(map[string]any, len(tokens))
	for k, v := range tokens {
		out[k] = v
	}
	return out
}

func cardNamesToStrings(cards []CardName) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = string(c)
	}
	return out
}

func cardNamesFromStrings(ss []string) []CardName {
	out := make([]CardName, len(ss))
	for i, s := range ss {
		out[i] = CardName(s)
	}
	return out
}

func shallowCopyData(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// Misc helpers
// ---------------------------------------------------------------------------

func containsCard(hand []string, card CardName) bool {
	for _, c := range hand {
		if c == string(card) {
			return true
		}
	}
	return false
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func otherCardInHand(hand []string, played CardName) CardName {
	for _, c := range hand {
		if c != string(played) {
			return CardName(c)
		}
	}
	return ""
}

func appendUnique(ss []string, s string) []string {
	for _, v := range ss {
		if v == s {
			return ss
		}
	}
	return append(ss, s)
}
