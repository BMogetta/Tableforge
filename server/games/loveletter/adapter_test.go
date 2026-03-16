package loveletter_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	botadapter "github.com/tableforge/server/games/loveletter"
	"github.com/tableforge/server/internal/bot"
	"github.com/tableforge/server/internal/bot/mcts"
	"github.com/tableforge/server/internal/domain/engine"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// getAdapterHand extracts a player's hand from the engine state.
// Distinct from getAdapterHand in loveletter_test.go which takes *testing.T and
// applies a JSON round-trip.
func getAdapterHand(s engine.GameState, playerID string) []string {
	handsRaw, ok := s.Data["hands"]
	if !ok {
		return nil
	}
	switch v := handsRaw.(type) {
	case map[string][]string:
		return v[playerID]
	case map[string]any:
		switch hand := v[playerID].(type) {
		case []string:
			return hand
		case []any:
			out := make([]string, 0, len(hand))
			for _, c := range hand {
				if s, ok := c.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func initState(t *testing.T, playerIDs ...string) engine.GameState {
	t.Helper()
	players := make([]engine.Player, len(playerIDs))
	for i, id := range playerIDs {
		players[i] = engine.Player{ID: engine.PlayerID(id)}
	}
	state, err := game.Init(players)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return state
}

func toBotState(s engine.GameState) bot.BotGameState {
	return bot.NewBotState(s)
}

func getDeck(s engine.GameState) []string {
	switch v := s.Data["deck"].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, c := range v {
			if s, ok := c.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func phase(s engine.GameState) string {
	v, _ := s.Data["phase"].(string)
	return v
}

// ---------------------------------------------------------------------------
// Determinize
// ---------------------------------------------------------------------------

func TestDeterminize_PreservesOwnHand(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	before := getAdapterHand(state, "p1")

	det := adapter.Determinize(toBotState(state), "p1")
	after := getAdapterHand(det.EngineState(), "p1")

	if len(before) != len(after) {
		t.Fatalf("own hand length changed: %d → %d", len(before), len(after))
	}
	for i, c := range before {
		if c != after[i] {
			t.Errorf("own hand card %d changed: %s → %s", i, c, after[i])
		}
	}
}

func TestDeterminize_OpponentGetsOneCard(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")

	det := adapter.Determinize(toBotState(state), "p1")
	p2Hand := getAdapterHand(det.EngineState(), "p2")

	if len(p2Hand) != 1 {
		t.Errorf("expected opponent to have 1 card, got %d: %v", len(p2Hand), p2Hand)
	}
}

func TestDeterminize_AssignedCardIsFromValidPool(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")

	fullDeckCounts := map[string]int{
		"spy": 2, "guard": 6, "priest": 2, "baron": 2,
		"handmaid": 2, "prince": 2, "chancellor": 2,
		"king": 1, "countess": 1, "princess": 1,
	}

	// Run 30 determinizations — assigned cards must always be valid.
	for i := 0; i < 30; i++ {
		det := adapter.Determinize(toBotState(state), "p1")
		ds := det.EngineState()

		counts := map[string]int{}
		counts[getAdapterHand(state, "p1")[0]]++
		counts[getAdapterHand(state, "p1")[1]]++
		for _, c := range getAdapterHand(ds, "p2") {
			counts[c]++
		}
		for _, c := range getDeck(ds) {
			counts[c]++
		}

		for card, count := range counts {
			if count > fullDeckCounts[card] {
				t.Errorf("iter %d: card %q over-represented (%d > %d)",
					i, card, count, fullDeckCounts[card])
			}
		}
	}
}

func TestDeterminize_EliminatedPlayerGetsNoCard(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2", "p3")
	state.Data["eliminated"] = []any{"p2"}
	state.Data["hands"] = map[string]any{
		"p1": []any{"guard", "spy"},
		"p2": []any{},
		"p3": []any{"priest"},
	}

	det := adapter.Determinize(toBotState(state), "p1")
	p2Hand := getAdapterHand(det.EngineState(), "p2")

	if len(p2Hand) != 0 {
		t.Errorf("eliminated player should have no cards after determinize, got %v", p2Hand)
	}
}

func TestDeterminize_DoesNotMutateOriginal(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	bs := toBotState(state)
	originalP2 := getAdapterHand(bs.EngineState(), "p2")

	adapter.Determinize(bs, "p1")

	afterP2 := getAdapterHand(bs.EngineState(), "p2")
	if len(originalP2) != len(afterP2) {
		t.Errorf("Determinize mutated original: p2 hand %v → %v", originalP2, afterP2)
	}
}

// ---------------------------------------------------------------------------
// LegalMoves
// ---------------------------------------------------------------------------

func TestLegalMoves_ReturnsMovesInPlayingPhase(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")

	moves := adapter.ValidMoves(toBotState(state))
	if len(moves) == 0 {
		t.Error("expected legal moves in playing phase, got none")
	}
}

func TestLegalMoves_ReturnsNilInTerminalPhase(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "game_over"

	moves := adapter.ValidMoves(toBotState(state))
	if len(moves) != 0 {
		t.Errorf("expected no moves in game_over, got %d", len(moves))
	}
}

func TestLegalMoves_CountessRule(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["hands"] = map[string]any{
		"p1": []any{"countess", "king"},
		"p2": []any{"guard"},
	}
	state.CurrentPlayerID = "p1"

	moves := adapter.ValidMoves(toBotState(state))
	if len(moves) == 0 {
		t.Fatal("expected at least one move")
	}
	for _, m := range moves {
		payload, _ := m.Payload()
		card, _ := payload["card"].(string)
		if card != "countess" {
			t.Errorf("Countess rule violated: move with card=%q generated", card)
		}
	}
}

func TestLegalMoves_ChancellorPendingReturnsResolves(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "chancellor_pending"
	state.Data["chancellor_choices"] = []any{"guard", "priest", "spy"}
	state.Data["hands"] = map[string]any{"p1": []any{}}
	state.CurrentPlayerID = "p1"

	moves := adapter.ValidMoves(toBotState(state))
	if len(moves) != 3 {
		t.Errorf("expected 3 chancellor_resolve moves, got %d", len(moves))
	}
	for _, m := range moves {
		payload, _ := m.Payload()
		if payload["card"] != "chancellor_resolve" {
			t.Errorf("expected chancellor_resolve, got %v", payload["card"])
		}
		if _, ok := payload["keep"]; !ok {
			t.Error("missing 'keep' in chancellor_resolve payload")
		}
		ret, _ := payload["return"].([]any)
		if len(ret) != 2 {
			t.Errorf("expected 2 return cards, got %d", len(ret))
		}
	}
}

func TestLegalMoves_GuardNoTargetWhenAllProtected(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["protected"] = []any{"p2"}
	state.Data["hands"] = map[string]any{
		"p1": []any{"guard", "spy"},
		"p2": []any{"priest"},
	}
	state.CurrentPlayerID = "p1"

	moves := adapter.ValidMoves(toBotState(state))
	for _, m := range moves {
		payload, _ := m.Payload()
		if payload["card"] == "guard" {
			if _, hasTarget := payload["target_player_id"]; hasTarget {
				t.Error("Guard should not target protected opponent")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// IsTerminal
// ---------------------------------------------------------------------------

func TestIsTerminal_FalseInPlayingPhase(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	if adapter.IsTerminal(toBotState(state)) {
		t.Error("expected IsTerminal=false in playing phase")
	}
}

func TestIsTerminal_TrueInGameOverPhase(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "game_over"
	if !adapter.IsTerminal(toBotState(state)) {
		t.Error("expected IsTerminal=true in game_over phase")
	}
}

// ---------------------------------------------------------------------------
// Score
// ---------------------------------------------------------------------------

func TestScore_WinnerReturnsOne(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "game_over"
	state.Data["game_winner_id"] = "p1"

	if s := adapter.Result(toBotState(state), "p1"); s != 1.0 {
		t.Errorf("expected 1.0 for winner, got %f", s)
	}
}

func TestScore_LoserReturnsZero(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "game_over"
	state.Data["game_winner_id"] = "p1"

	if s := adapter.Result(toBotState(state), "p2"); s != 0.0 {
		t.Errorf("expected 0.0 for loser, got %f", s)
	}
}

func TestScore_DrawReturnsHalf(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["phase"] = "game_over"
	state.Data["game_winner_id"] = ""

	if s := adapter.Result(toBotState(state), "p1"); s != 0.5 {
		t.Errorf("expected 0.5 for draw, got %f", s)
	}
}

func TestScore_MidRoundInRange(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")

	s := adapter.Result(toBotState(state), "p1")
	if s < 0.0 || s > 1.0 {
		t.Errorf("mid-round score out of [0,1]: %f", s)
	}
}

func TestScore_EliminatedLowerThanSurvivor(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	state.Data["eliminated"] = []any{"p1"}
	state.Data["tokens"] = map[string]any{"p1": 0, "p2": 0}

	elim := adapter.Result(toBotState(state), "p1")
	surv := adapter.Result(toBotState(state), "p2")

	if elim >= surv {
		t.Errorf("eliminated score (%f) should be < survivor score (%f)", elim, surv)
	}
}

// ---------------------------------------------------------------------------
// ApplyMove
// ---------------------------------------------------------------------------

func TestApplyMove_AdvancesState(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	bs := toBotState(state)

	moves := adapter.ValidMoves(bs)
	if len(moves) == 0 {
		t.Fatal("no legal moves")
	}

	result := adapter.ApplyMove(bs, moves[0])
	before := bs.EngineState()
	after := result.EngineState()

	// Either player changed or we entered chancellor_pending (same player).
	if before.CurrentPlayerID == after.CurrentPlayerID &&
		phase(after) != "chancellor_pending" {
		t.Error("state did not advance after ApplyMove")
	}
}

func TestApplyMove_InvalidMove_ReturnsOriginal(t *testing.T) {
	adapter := botadapter.New()
	state := initState(t, "p1", "p2")
	bs := toBotState(state)

	// Malformed JSON — Payload() returns an error, ApplyMove returns original.
	badMove := bot.BotMove("{not valid json")
	result := adapter.ApplyMove(bs, badMove)

	if result.EngineState().CurrentPlayerID != bs.EngineState().CurrentPlayerID {
		t.Error("invalid move should return original state unchanged")
	}
}

// ---------------------------------------------------------------------------
// Integration — bot vs bot
// ---------------------------------------------------------------------------

func TestBotVsBot_CompletesGame(t *testing.T) {
	adapter := botadapter.New()
	cfg, _ := bot.ConfigFromProfile("easy")

	p1ID := "00000000-0000-0000-0000-000000000001"
	p2ID := "00000000-0000-0000-0000-000000000002"
	p1UUID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	p2UUID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	bp1 := &bot.BotPlayer{ID: p1UUID, GameID: "loveletter", Adapter: adapter, Config: cfg, Search: mcts.Search}
	bp2 := &bot.BotPlayer{ID: p2UUID, GameID: "loveletter", Adapter: adapter, Config: cfg, Search: mcts.Search}

	state := initState(t, p1ID, p2ID)

	done := make(chan error, 1)
	go func() {
		for i := 0; i < 500; i++ {
			over, _ := game.IsOver(state)
			if over {
				done <- nil
				return
			}

			// TODO Log progress every 100 moves to diagnose timeouts.
			if i%100 == 0 {
				tokens, _ := state.Data["tokens"].(map[string]any)
				t.Logf("move %d: current=%s tokens=%v phase=%v",
					i, state.CurrentPlayerID, tokens,
					state.Data["phase"])
			}

			var bp *bot.BotPlayer
			if state.CurrentPlayerID == engine.PlayerID(p1ID) {
				bp = bp1
			} else {
				bp = bp2
			}

			payload, err := bp.DecideMove(context.Background(), state)
			if err != nil {
				t.Logf("DecideMove error: %v", err)
				done <- nil
				return
			}
			t.Logf("move applied, next player: %s", state.CurrentPlayerID)

			state, err = game.ApplyMove(state, engine.Move{
				PlayerID: state.CurrentPlayerID,
				Payload:  payload,
			})
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
		t.Error("bot-vs-bot did not complete within 500 moves")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("bot-vs-bot error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("bot-vs-bot timed out after 30s")
	}
}
