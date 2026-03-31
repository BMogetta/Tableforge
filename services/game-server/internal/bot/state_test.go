package bot_test

import (
	"encoding/json"
	"testing"

	"github.com/tableforge/game-server/internal/bot"
	"github.com/tableforge/game-server/internal/domain/engine"
)

func makeState(data map[string]any) engine.GameState {
	return engine.GameState{
		CurrentPlayerID: "p1",
		Data:            data,
	}
}

// ---------------------------------------------------------------------------
// NewBotState / EngineState
// ---------------------------------------------------------------------------

func TestNewBotState_EngineState(t *testing.T) {
	s := makeState(map[string]any{"board": []any{"X", "", ""}})
	bs := bot.NewBotState(s)
	got := bs.EngineState()
	if got.CurrentPlayerID != "p1" {
		t.Errorf("expected p1, got %s", got.CurrentPlayerID)
	}
}

// ---------------------------------------------------------------------------
// Clone — independence
// ---------------------------------------------------------------------------

func TestClone_IsIndependent(t *testing.T) {
	s := makeState(map[string]any{
		"board": []any{"X", "", ""},
		"marks": map[string]any{"p1": "X", "p2": "O"},
	})
	bs := bot.NewBotState(s)
	clone := bs.Clone()

	// Mutate the clone's Data — original must not change.
	cloneData := clone.EngineState().Data
	cloneData["board"] = []any{"X", "O", ""}

	origBoard := bs.EngineState().Data["board"].([]any)
	if origBoard[1] != "" {
		t.Error("Clone shares Data with original — not a deep copy")
	}
}

func TestClone_MapsAreIndependent(t *testing.T) {
	s := makeState(map[string]any{
		"marks": map[string]any{"p1": "X", "p2": "O"},
	})
	bs := bot.NewBotState(s)
	clone := bs.Clone()

	cloneMarks := clone.EngineState().Data["marks"].(map[string]any)
	cloneMarks["p1"] = "O"

	origMarks := bs.EngineState().Data["marks"].(map[string]any)
	if origMarks["p1"] != "X" {
		t.Error("Clone shares marks map with original — not a deep copy")
	}
}

func TestClone_CurrentPlayerIDIsPreserved(t *testing.T) {
	s := makeState(map[string]any{})
	bs := bot.NewBotState(s)
	clone := bs.Clone()
	if clone.EngineState().CurrentPlayerID != "p1" {
		t.Errorf("Clone did not preserve CurrentPlayerID")
	}
}

func TestClone_NestedStructure(t *testing.T) {
	// Simulate a Love Letter state with nested hands map.
	s := makeState(map[string]any{
		"hands": map[string]any{
			"p1": []any{"guard"},
			"p2": []any{"priest"},
		},
	})
	bs := bot.NewBotState(s)
	clone := bs.Clone()

	// Mutate clone's hands.
	cloneHands := clone.EngineState().Data["hands"].(map[string]any)
	cloneHands["p1"] = []any{"princess"}

	origHands := bs.EngineState().Data["hands"].(map[string]any)
	origP1 := origHands["p1"].([]any)
	if origP1[0] != "guard" {
		t.Error("Clone shares nested hands map with original")
	}
}

// ---------------------------------------------------------------------------
// BotMove serialization
// ---------------------------------------------------------------------------

func TestMoveFromPayload_RoundTrip(t *testing.T) {
	payload := map[string]any{
		"card":             "guard",
		"target_player_id": "p2",
		"guess":            "priest",
	}
	m, err := bot.MoveFromPayload(payload)
	if err != nil {
		t.Fatalf("MoveFromPayload: %v", err)
	}

	got, err := m.Payload()
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}

	if got["card"] != "guard" {
		t.Errorf("expected card=guard, got %v", got["card"])
	}
	if got["guess"] != "priest" {
		t.Errorf("expected guess=priest, got %v", got["guess"])
	}
}

func TestMoveFromPayload_IsComparable(t *testing.T) {
	p1 := map[string]any{"card": "spy"}
	p2 := map[string]any{"card": "spy"}

	m1, _ := bot.MoveFromPayload(p1)
	m2, _ := bot.MoveFromPayload(p2)

	// BotMove is a string — same payload must produce equal moves.
	if m1 != m2 {
		t.Errorf("same payload produced different BotMove values: %q vs %q", m1, m2)
	}
}

func TestMoveFromPayload_InvalidJSON(t *testing.T) {
	var m bot.BotMove = "not-json"
	_, err := m.Payload()
	if err == nil {
		t.Fatal("expected error for invalid JSON move")
	}
}

func TestMoveFromPayload_EmptyPayload(t *testing.T) {
	m, err := bot.MoveFromPayload(map[string]any{})
	if err != nil {
		t.Fatalf("MoveFromPayload empty: %v", err)
	}
	got, err := m.Payload()
	if err != nil {
		t.Fatalf("Payload empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

// Verify that Clone produces a state that survives a second JSON round-trip
// (as would happen when the runtime persists it to Postgres).
func TestClone_SurvivesDoubleRoundTrip(t *testing.T) {
	s := makeState(map[string]any{
		"round": float64(1),
		"phase": "playing",
		"hands": map[string]any{
			"p1": []any{"guard"},
		},
	})
	bs := bot.NewBotState(s)
	clone := bs.Clone()

	b, err := json.Marshal(clone.EngineState())
	if err != nil {
		t.Fatalf("marshal clone: %v", err)
	}
	var recovered engine.GameState
	if err := json.Unmarshal(b, &recovered); err != nil {
		t.Fatalf("unmarshal clone: %v", err)
	}
	if recovered.CurrentPlayerID != "p1" {
		t.Errorf("recovered CurrentPlayerID mismatch")
	}
}
