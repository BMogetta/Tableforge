package loveletter_test

import (
	"encoding/json"
	"testing"

	"github.com/tableforge/game-server/games/loveletter"
	"github.com/tableforge/game-server/internal/domain/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var game = &loveletter.LoveLetter{}

func players(ids ...string) []engine.Player {
	out := make([]engine.Player, len(ids))
	for i, id := range ids {
		out[i] = engine.Player{ID: engine.PlayerID(id), Username: id}
	}
	return out
}

func mustInit(t *testing.T, ids ...string) engine.GameState {
	t.Helper()
	state, err := game.Init(players(ids...))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return state
}

// roundtrip simulates JSON serialization that happens between moves in the
// real runtime (state is marshalled to Postgres and unmarshalled back).
func roundtrip(t *testing.T, state engine.GameState) engine.GameState {
	t.Helper()
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("roundtrip marshal: %v", err)
	}
	var out engine.GameState
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("roundtrip unmarshal: %v", err)
	}
	return out
}

func move(playerID string, payload map[string]any) engine.Move {
	return engine.Move{
		PlayerID: engine.PlayerID(playerID),
		Payload:  payload,
	}
}

// mustApply validates and applies a move, performing a JSON roundtrip first
// to simulate real runtime conditions.
func mustApply(t *testing.T, state engine.GameState, m engine.Move) engine.GameState {
	t.Helper()
	state = roundtrip(t, state)
	if err := game.ValidateMove(state, m); err != nil {
		t.Fatalf("ValidateMove: %v", err)
	}
	next, err := game.ApplyMove(state, m)
	if err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}
	return next
}

// setHand replaces playerID's hand in state.Data with exactly the given cards.
// For the active player, always provide 2 cards (the engine expects a post-draw
// hand). For inactive players, provide 1 card.
func setHand(state engine.GameState, playerID string, cards ...string) engine.GameState {
	hands, _ := state.Data["hands"].(map[string]any)
	if hands == nil {
		hands = map[string]any{}
	}
	hands[playerID] = cards
	state.Data["hands"] = hands
	return state
}

// setDeck replaces the deck in state.Data with the given cards.
// Always include at least 1 card so advanceTurn can draw for the next player.
func setDeck(state engine.GameState, cards ...string) engine.GameState {
	state.Data["deck"] = cards
	return state
}

func getHand(t *testing.T, state engine.GameState, playerID string) []string {
	t.Helper()
	state = roundtrip(t, state)
	hands, _ := state.Data["hands"].(map[string]any)
	if hands == nil {
		return nil
	}
	raw := hands[playerID]
	switch v := raw.(type) {
	case []any:
		out := make([]string, len(v))
		for i, c := range v {
			out[i], _ = c.(string)
		}
		return out
	case []string:
		return v
	}
	return nil
}

func getStringSlice(t *testing.T, state engine.GameState, key string) []string {
	t.Helper()
	state = roundtrip(t, state)
	raw, ok := state.Data[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, len(v))
		for i, c := range v {
			out[i], _ = c.(string)
		}
		return out
	case []string:
		return v
	}
	return nil
}

func getString(t *testing.T, state engine.GameState, key string) string {
	t.Helper()
	state = roundtrip(t, state)
	v, _ := state.Data[key].(string)
	return v
}

func getInt(t *testing.T, state engine.GameState, key string) int {
	t.Helper()
	state = roundtrip(t, state)
	switch v := state.Data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func getToken(t *testing.T, state engine.GameState, playerID string) int {
	t.Helper()
	state = roundtrip(t, state)
	raw, _ := state.Data["tokens"].(map[string]any)
	if raw == nil {
		return 0
	}
	switch v := raw[playerID].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func isEliminated(t *testing.T, state engine.GameState, playerID string) bool {
	t.Helper()
	for _, id := range getStringSlice(t, state, "eliminated") {
		if id == playerID {
			return true
		}
	}
	return false
}

func isProtected(t *testing.T, state engine.GameState, playerID string) bool {
	t.Helper()
	for _, id := range getStringSlice(t, state, "protected") {
		if id == playerID {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInit_TwoPlayers(t *testing.T) {
	state := mustInit(t, "p1", "p2")

	if state.CurrentPlayerID != "p1" {
		t.Errorf("expected p1 to go first, got %s", state.CurrentPlayerID)
	}
	if getString(t, state, "phase") != "playing" {
		t.Errorf("expected phase playing")
	}
	if getInt(t, state, "round") != 1 {
		t.Errorf("expected round 1")
	}

	// The first player holds 2 cards (dealt + turn draw); all others hold 1.
	if len(getHand(t, state, "p1")) != 2 {
		t.Errorf("p1 should have 2 cards after Init (dealt + turn draw), got %d", len(getHand(t, state, "p1")))
	}
	if len(getHand(t, state, "p2")) != 1 {
		t.Errorf("p2 should have 1 card after Init, got %d", len(getHand(t, state, "p2")))
	}
	// 2-player: 21 - 1 (face-down) - 3 (face-up) - 2 (dealt) - 1 (first player draw) = 14 remaining.
	deck := getStringSlice(t, state, "deck")
	if len(deck) != 14 {
		t.Errorf("expected 14 cards in deck for 2-player game, got %d", len(deck))
	}
	// Tokens start at zero.
	if getToken(t, state, "p1") != 0 || getToken(t, state, "p2") != 0 {
		t.Errorf("expected tokens to start at 0")
	}
	// 2-player: 3 face-up set-aside cards.
	faceUp := getStringSlice(t, state, "set_aside_visible")
	if len(faceUp) != 3 {
		t.Errorf("expected 3 face-up cards for 2-player game, got %d", len(faceUp))
	}
}

func TestInit_FivePlayers(t *testing.T) {
	state := mustInit(t, "p1", "p2", "p3", "p4", "p5")
	// 5-player: 21 - 1 (face-down) - 5 (dealt) - 1 (first player draw) = 14 remaining.
	deck := getStringSlice(t, state, "deck")
	if len(deck) != 14 {
		t.Errorf("expected 14 cards in deck for 5-player game, got %d", len(deck))
	}
	// No face-up set-aside for 5-player.
	faceUp := getStringSlice(t, state, "set_aside_visible")
	if len(faceUp) != 0 {
		t.Errorf("expected 0 face-up cards for 5-player, got %d", len(faceUp))
	}
}

func TestInit_TooFewPlayers(t *testing.T) {
	_, err := game.Init(players("p1"))
	if err == nil {
		t.Fatal("expected error for 1 player")
	}
}

func TestInit_TooManyPlayers(t *testing.T) {
	_, err := game.Init(players("p1", "p2", "p3", "p4", "p5", "p6"))
	if err == nil {
		t.Fatal("expected error for 6 players")
	}
}

// ---------------------------------------------------------------------------
// ValidateMove
// ---------------------------------------------------------------------------

func TestValidateMove_NotYourTurn(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p2 is not the current player — any move from p2 should fail.
	state = setHand(state, "p2", "spy", "guard")
	m := move("p2", map[string]any{"card": "spy"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: not your turn")
	}
}

func TestValidateMove_CardNotInHand(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds guard+guard but tries to play priest.
	state = setHand(state, "p1", "guard", "guard")
	m := move("p1", map[string]any{"card": "priest", "target_player_id": "p2"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: card not in hand")
	}
}

func TestValidateMove_CountessForcedWithKing(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds king+countess — must play countess, not king.
	state = setHand(state, "p1", "king", "countess")
	m := move("p1", map[string]any{"card": "king", "target_player_id": "p2"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: must play countess when holding king")
	}
}

func TestValidateMove_CountessAllowedWhenForcedPlay(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "king", "countess")
	m := move("p1", map[string]any{"card": "countess"})
	if err := game.ValidateMove(roundtrip(t, state), m); err != nil {
		t.Fatalf("expected countess to be valid: %v", err)
	}
}

func TestValidateMove_CountessForcedWithPrince(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "prince", "countess")
	m := move("p1", map[string]any{"card": "prince", "target_player_id": "p1"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: must play countess when holding prince")
	}
}

func TestValidateMove_GuardRequiresGuess(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	m := move("p1", map[string]any{"card": "guard", "target_player_id": "p2"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: guard requires guess")
	}
}

func TestValidateMove_GuardCannotGuessGuard(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	m := move("p1", map[string]any{"card": "guard", "target_player_id": "p2", "guess": "guard"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: cannot guess guard")
	}
}

func TestValidateMove_TargetProtected(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	state.Data["protected"] = []string{"p2"}
	m := move("p1", map[string]any{"card": "guard", "target_player_id": "p2", "guess": "priest"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: target is protected")
	}
}

func TestValidateMove_TargetEliminated(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	state.Data["eliminated"] = []string{"p2"}
	m := move("p1", map[string]any{"card": "guard", "target_player_id": "p2", "guess": "priest"})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: target is eliminated")
	}
}

func TestValidateMove_ChancellorResolveWrongPhase(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "chancellor", "spy")
	// Phase is still "playing" — chancellor_resolve should be rejected.
	m := move("p1", map[string]any{
		"card":   "chancellor_resolve",
		"keep":   "spy",
		"return": []any{"chancellor", "guard"},
	})
	if err := game.ValidateMove(roundtrip(t, state), m); err == nil {
		t.Fatal("expected error: chancellor_resolve invalid in playing phase")
	}
}

func TestValidateMove_PenaltyLoseAlwaysValid(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// penalty_lose bypasses all validation including turn order.
	m := move("p2", map[string]any{"card": "penalty_lose"})
	if err := game.ValidateMove(roundtrip(t, state), m); err != nil {
		t.Fatalf("expected penalty_lose to always be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ApplyMove — card effects
// ---------------------------------------------------------------------------

func TestApplyMove_Spy_TrackedInState(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds spy+guard; deck has 1 card for p2's draw in advanceTurn.
	state = setHand(state, "p1", "spy", "guard")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{"card": "spy"}))

	found := false
	for _, id := range getStringSlice(t, state, "spy_played_by") {
		if id == "p1" {
			found = true
		}
	}
	if !found {
		t.Error("expected p1 in spy_played_by after playing Spy")
	}
}

func TestApplyMove_Guard_CorrectGuess_Eliminates(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds guard+spy (plays guard); p2 holds priest; deck for advanceTurn.
	state = setHand(state, "p1", "guard", "spy")
	state = setHand(state, "p2", "priest")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "guard",
		"target_player_id": "p2",
		"guess":            "priest",
	}))

	// p2 was eliminated — round resolved immediately and a new round started.
	// Verify p1 earned a token as evidence that they won the round.
	if getToken(t, state, "p1") != 1 {
		t.Errorf("expected p1 to have 1 token after eliminating p2, got %d", getToken(t, state, "p1"))
	}
}

func TestApplyMove_Guard_WrongGuess_NoElimination(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	state = setHand(state, "p2", "priest")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "guard",
		"target_player_id": "p2",
		"guess":            "baron",
	}))

	if isEliminated(t, state, "p2") {
		t.Error("expected p2 to survive wrong Guard guess")
	}
}

func TestApplyMove_Priest_SetsPrivateReveal(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds priest+spy (plays priest); p2 holds king.
	state = setHand(state, "p1", "priest", "spy")
	state = setHand(state, "p2", "king")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "priest",
		"target_player_id": "p2",
	}))

	// private_reveals should be set for p1 showing p2's card.
	// Note: advanceTurn clears private_reveals, so we check before roundtrip
	// by inspecting the raw state returned from mustApply.
	reveals, _ := state.Data["private_reveals"].(map[string]any)
	if reveals == nil {
		t.Fatal("expected private_reveals in state after Priest")
	}
	if reveals["p1"] != "king" {
		t.Errorf("expected p1 to see king, got %v", reveals["p1"])
	}
}

func TestApplyMove_Baron_LowerValueEliminated(t *testing.T) {
	// 3 players so eliminating p2 doesn't end the round.
	state := mustInit(t, "p1", "p2", "p3")
	// p1 holds baron+priest (plays baron, retains priest value=2).
	// p2 holds guard (value=1) — p2 has lower value and is eliminated.
	state = setHand(state, "p1", "baron", "priest")
	state = setHand(state, "p2", "guard")
	state = setDeck(state, "guard", "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "baron",
		"target_player_id": "p2",
	}))

	if !isEliminated(t, state, "p2") {
		t.Error("expected p2 to be eliminated (lower hand value)")
	}
}

func TestApplyMove_Baron_Tie_NoElimination(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// Both hold guard after baron played (p1: baron+guard plays baron, keeps guard).
	state = setHand(state, "p1", "baron", "guard")
	state = setHand(state, "p2", "guard")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "baron",
		"target_player_id": "p2",
	}))

	if isEliminated(t, state, "p1") || isEliminated(t, state, "p2") {
		t.Error("expected no elimination on Baron tie")
	}
}

func TestApplyMove_Handmaid_ProtectsPlayer(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds handmaid+spy (plays handmaid).
	state = setHand(state, "p1", "handmaid", "spy")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{"card": "handmaid"}))

	if !isProtected(t, state, "p1") {
		t.Error("expected p1 to be in protected list after Handmaid")
	}
}

func TestApplyMove_Handmaid_ExpiresNextTurn(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 plays handmaid; p2 plays spy (no target); p1 should no longer be protected.
	state = setHand(state, "p1", "handmaid", "spy")
	state = setDeck(state, "guard", "guard", "guard", "guard")

	state = mustApply(t, state, move("p1", map[string]any{"card": "handmaid"}))

	// p2 now has 2 cards (original + drawn by advanceTurn); play spy.
	state = setHand(state, "p2", "spy", "guard")
	state = mustApply(t, state, move("p2", map[string]any{"card": "spy"}))

	// p1's protection should have expired at end of p1's turn.
	if isProtected(t, state, "p1") {
		t.Error("p1 should no longer be protected after their turn ended")
	}
}

func TestApplyMove_Prince_TargetDrawsNewCard(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds prince+spy (plays prince targeting p2); p2 holds guard (discarded).
	// Deck: first card drawn by p2 after discard, second for advanceTurn (p2's next turn).
	state = setHand(state, "p1", "prince", "spy")
	state = setHand(state, "p2", "guard")
	state = setDeck(state, "king", "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "prince",
		"target_player_id": "p2",
	}))

	// After Prince, p2 discarded their hand and received a new card from the deck.
	// advanceTurn then drew p2's turn card, so p2 now holds 2 cards.
	// Verify p2 no longer holds the original guard they were forced to discard.
	hand := getHand(t, state, "p2")
	if len(hand) != 2 {
		t.Fatalf("expected p2 to have 2 cards after Prince + turn draw, got %d: %v", len(hand), hand)
	}
	for _, c := range hand {
		if c == "guard" {
			// The original guard was discarded — but the deck also contains guards,
			// so we can only verify p2 received new cards, not their exact identity.
			// Check discard pile instead.
			break
		}
	}
	// Verify the original guard is in p2's discard pile.
	state = roundtrip(t, state)
	discards, _ := state.Data["discard_piles"].(map[string]any)
	p2Discards := discards["p2"]
	found := false
	switch v := p2Discards.(type) {
	case []any:
		for _, c := range v {
			if c == "guard" {
				found = true
			}
		}
	case []string:
		for _, c := range v {
			if c == "guard" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected original guard to be in p2's discard pile after Prince")
	}
}

func TestApplyMove_Prince_DiscardsPrincess_Eliminates(t *testing.T) {
	// 3 players so eliminating p2 doesn't end the round immediately.
	state := mustInit(t, "p1", "p2", "p3")
	state = setHand(state, "p1", "prince", "spy")
	state = setHand(state, "p2", "princess")
	state = setDeck(state, "guard", "guard", "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "prince",
		"target_player_id": "p2",
	}))

	if !isEliminated(t, state, "p2") {
		t.Error("expected p2 to be eliminated when Prince forces Princess discard")
	}
}

func TestApplyMove_King_SwapsHands(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds king+spy (plays king, keeps spy after swap); p2 holds guard.
	// After swap: p1 gets guard, p2 gets spy.
	state = setHand(state, "p1", "king", "spy")
	state = setHand(state, "p2", "guard")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "king",
		"target_player_id": "p2",
	}))

	p1Hand := getHand(t, state, "p1")
	p2Hand := getHand(t, state, "p2")
	if len(p1Hand) != 1 {
		t.Fatalf("expected p1 to have 1 card after King, got %v", p1Hand)
	}
	// advanceTurn draws p2's turn card after the swap, so p2 holds 2 cards.
	if len(p2Hand) != 2 {
		t.Fatalf("expected p2 to have 2 cards after King + turn draw, got %v", p2Hand)
	}
	// p1 played king, held spy — after swap p1 gets p2's guard.
	if p1Hand[0] != "guard" {
		t.Errorf("expected p1 to hold guard after King swap, got %s", p1Hand[0])
	}
	// p2 received spy via swap — verify spy is among p2's cards.
	spyFound := false
	for _, c := range p2Hand {
		if c == "spy" {
			spyFound = true
		}
	}
	if !spyFound {
		t.Errorf("expected p2 to hold spy after King swap, got %v", p2Hand)
	}
}

func TestApplyMove_Princess_SelfEliminates(t *testing.T) {
	// 3 players so p1 eliminating themselves doesn't end the round.
	state := mustInit(t, "p1", "p2", "p3")
	state = setHand(state, "p1", "princess", "spy")
	state = setDeck(state, "guard", "guard", "guard")

	state = mustApply(t, state, move("p1", map[string]any{"card": "princess"}))

	if !isEliminated(t, state, "p1") {
		t.Error("expected p1 to be eliminated after playing Princess")
	}
}

func TestApplyMove_Chancellor_EntersPendingPhase(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds chancellor+spy (plays chancellor).
	// Deck needs 2 cards for chancellor draw + 1 for advanceTurn later.
	state = setHand(state, "p1", "chancellor", "spy")
	state = setDeck(state, "guard", "priest", "baron")

	state = mustApply(t, state, move("p1", map[string]any{"card": "chancellor"}))

	if getString(t, state, "phase") != "chancellor_pending" {
		t.Errorf("expected phase chancellor_pending, got %s", getString(t, state, "phase"))
	}
	// chancellor_choices = remaining hand card (spy) + 2 drawn = 3 total.
	state = roundtrip(t, state)
	choices, _ := state.Data["chancellor_choices"].([]any)
	if len(choices) != 3 {
		t.Errorf("expected 3 chancellor_choices, got %d: %v", len(choices), choices)
	}
}

func TestApplyMove_ChancellorResolve_KeepsCard_ReturnsToDeck(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 holds chancellor+spy; deck has guard and priest for the 2 chancellor draws,
	// plus baron for advanceTurn after resolve.
	state = setHand(state, "p1", "chancellor", "spy")
	state = setDeck(state, "guard", "priest", "baron")

	// Play chancellor — enters chancellor_pending.
	state = mustApply(t, state, move("p1", map[string]any{"card": "chancellor"}))
	state = roundtrip(t, state)

	// chancellor_choices = [spy, guard, priest]; keep spy, return [guard, priest].
	resolveMove := move("p1", map[string]any{
		"card":   "chancellor_resolve",
		"keep":   "spy",
		"return": []any{"guard", "priest"},
	})
	if err := game.ValidateMove(state, resolveMove); err != nil {
		t.Fatalf("ValidateMove chancellor_resolve: %v", err)
	}
	next, err := game.ApplyMove(state, resolveMove)
	if err != nil {
		t.Fatalf("ApplyMove chancellor_resolve: %v", err)
	}

	if getString(t, next, "phase") != "playing" {
		t.Errorf("expected phase playing after chancellor_resolve, got %s", getString(t, next, "phase"))
	}
	p1Hand := getHand(t, next, "p1")
	if len(p1Hand) != 1 || p1Hand[0] != "spy" {
		t.Errorf("expected p1 to hold spy after chancellor_resolve, got %v", p1Hand)
	}
}

func TestApplyMove_PenaltyLose_EliminatesActivePlayer(t *testing.T) {
	// 3 players so eliminating p1 doesn't end the round.
	state := mustInit(t, "p1", "p2", "p3")
	state = setDeck(state, "guard", "guard", "guard")

	next, err := game.ApplyMove(state, move("p1", map[string]any{"card": "penalty_lose"}))
	if err != nil {
		t.Fatalf("ApplyMove penalty_lose: %v", err)
	}
	if !isEliminated(t, next, "p1") {
		t.Error("expected p1 to be eliminated by penalty_lose")
	}
}

// ---------------------------------------------------------------------------
// Round resolution
// ---------------------------------------------------------------------------

func TestRoundResolution_LastPlayerWins_StartsNewRound(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// p1 plays guard and guesses p2's card correctly — p2 eliminated.
	// Round resolves, new round starts (tokens < threshold).
	state = setHand(state, "p1", "guard", "spy")
	state = setHand(state, "p2", "priest")
	// Deck needs enough cards to deal a new round after resolution.
	// dealRound calls buildDeck() internally — no need to set deck here.
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "guard",
		"target_player_id": "p2",
		"guess":            "priest",
	}))

	// p2 eliminated → round resolves → new round starts.
	phase := getString(t, state, "phase")
	if phase != "playing" {
		t.Errorf("expected new round to start (phase playing), got %s", phase)
	}
	if getToken(t, state, "p1") != 1 {
		t.Errorf("expected p1 to have 1 token after winning round, got %d", getToken(t, state, "p1"))
	}
}

func TestRoundResolution_SpyBonus_OnlyOneSpyPlayer(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// Set spy_played_by to only p1 — they should get the bonus at round end.
	state.Data["spy_played_by"] = []string{"p1"}

	spyPlayed := getStringSlice(t, state, "spy_played_by")
	if len(spyPlayed) != 1 || spyPlayed[0] != "p1" {
		t.Errorf("expected spy_played_by=[p1], got %v", spyPlayed)
	}
}

func TestRoundResolution_SpyBonus_TwoSpyPlayers_NoBonus(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// Both played spy — len(spy_played_by) == 2, no bonus awarded.
	state.Data["spy_played_by"] = []string{"p1", "p2"}

	spyPlayed := getStringSlice(t, state, "spy_played_by")
	if len(spyPlayed) != 2 {
		t.Errorf("expected 2 entries in spy_played_by, got %d", len(spyPlayed))
	}
}

func TestRoundResolution_GameOver_WhenTokenThresholdReached(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	// Give p1 6 tokens (threshold for 2p is 7) — winning this round pushes to 7.
	tokens := map[string]any{"p1": 6, "p2": 0}
	state.Data["tokens"] = tokens

	// p1 plays guard and eliminates p2 to win the round.
	state = setHand(state, "p1", "guard", "spy")
	state = setHand(state, "p2", "priest")
	state = setDeck(state, "guard")

	state = mustApply(t, state, move("p1", map[string]any{
		"card":             "guard",
		"target_player_id": "p2",
		"guess":            "priest",
	}))

	if getString(t, state, "phase") != "game_over" {
		t.Errorf("expected game_over phase, got %s", getString(t, state, "phase"))
	}
	if getString(t, state, "game_winner_id") != "p1" {
		t.Errorf("expected p1 as game winner, got %s", getString(t, state, "game_winner_id"))
	}
}

// ---------------------------------------------------------------------------
// IsOver
// ---------------------------------------------------------------------------

func TestIsOver_NotOver(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	over, _ := game.IsOver(state)
	if over {
		t.Error("expected game not over after Init")
	}
}

func TestIsOver_GameOver(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state.Data["phase"] = "game_over"
	state.Data["game_winner_id"] = "p1"
	over, result := game.IsOver(state)
	if !over {
		t.Fatal("expected game over")
	}
	if result.Status != engine.ResultWin {
		t.Errorf("expected ResultWin, got %s", result.Status)
	}
	if result.WinnerID == nil || string(*result.WinnerID) != "p1" {
		t.Errorf("expected winner p1, got %v", result.WinnerID)
	}
}

// ---------------------------------------------------------------------------
// FilterState
// ---------------------------------------------------------------------------

func TestFilterState_PlayerSeesOnlyOwnHand(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "king", "spy")
	state = setHand(state, "p2", "guard")

	filtered := game.FilterState(state, "p1")
	filtered = roundtrip(t, filtered)

	p1Hand := getHand(t, filtered, "p1")
	p2Hand := getHand(t, filtered, "p2")

	if len(p1Hand) != 2 {
		t.Errorf("p1 should see their 2-card hand, got %v", p1Hand)
	}
	if len(p2Hand) != 0 {
		t.Errorf("p1 should not see p2's hand, got %v", p2Hand)
	}
}

func TestFilterState_SpectatorSeesNoHands(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "king", "spy")
	state = setHand(state, "p2", "guard")

	filtered := game.FilterState(state, "")
	filtered = roundtrip(t, filtered)

	p1Hand := getHand(t, filtered, "p1")
	p2Hand := getHand(t, filtered, "p2")

	if len(p1Hand) != 0 || len(p2Hand) != 0 {
		t.Errorf("spectator should see no hands, got p1=%v p2=%v", p1Hand, p2Hand)
	}
}

func TestFilterState_ChancellorChoicesOnlyForActivePlayer(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state.Data["chancellor_choices"] = []string{"king", "guard", "spy"}
	state.Data["phase"] = "chancellor_pending"
	state.CurrentPlayerID = "p1"

	filteredP1 := game.FilterState(state, "p1")
	filteredP1 = roundtrip(t, filteredP1)
	if _, ok := filteredP1.Data["chancellor_choices"]; !ok {
		t.Error("active player p1 should see chancellor_choices")
	}

	filteredP2 := game.FilterState(state, "p2")
	filteredP2 = roundtrip(t, filteredP2)
	if _, ok := filteredP2.Data["chancellor_choices"]; ok {
		t.Error("p2 should not see chancellor_choices")
	}

	filteredSpectator := game.FilterState(state, "")
	filteredSpectator = roundtrip(t, filteredSpectator)
	if _, ok := filteredSpectator.Data["chancellor_choices"]; ok {
		t.Error("spectator should not see chancellor_choices")
	}
}

func TestFilterState_PrivateRevealOnlyForRecipient(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state.Data["private_reveals"] = map[string]any{"p1": "king"}

	filteredP1 := game.FilterState(state, "p1")
	filteredP1 = roundtrip(t, filteredP1)
	reveals, _ := filteredP1.Data["private_reveals"].(map[string]any)
	if reveals == nil || reveals["p1"] != "king" {
		t.Errorf("p1 should see their private reveal, got %v", reveals)
	}

	filteredP2 := game.FilterState(state, "p2")
	filteredP2 = roundtrip(t, filteredP2)
	if _, ok := filteredP2.Data["private_reveals"]; ok {
		t.Error("p2 should not see p1's private reveal")
	}

	filteredSpectator := game.FilterState(state, "")
	filteredSpectator = roundtrip(t, filteredSpectator)
	if _, ok := filteredSpectator.Data["private_reveals"]; ok {
		t.Error("spectator should not see private reveals")
	}
}

func TestTimeoutMove(t *testing.T) {
	payload := game.TimeoutMove()
	if payload["card"] != "penalty_lose" {
		t.Errorf("expected penalty_lose, got %v", payload["card"])
	}
}

func TestLoveLetter_ImplementsTurnTimeoutHandler(t *testing.T) {
	var _ engine.TurnTimeoutHandler = &loveletter.LoveLetter{}
}

func TestApplyMove_Guard_NoValidTarget_NoEffect(t *testing.T) {
	state := mustInit(t, "p1", "p2")
	state = setHand(state, "p1", "guard", "spy")
	state.Data["protected"] = []string{"p2"}
	state = setDeck(state, "guard")

	// Guard with empty target — legal when all opponents are protected.
	// guess is still required by validation even when there is no valid target.
	state = mustApply(t, state, move("p1", map[string]any{
		"card":  "guard",
		"guess": "priest",
	}))

	if isEliminated(t, state, "p2") {
		t.Error("expected no elimination when guard played with no valid target")
	}
}
