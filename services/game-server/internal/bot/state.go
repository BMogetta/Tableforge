package bot

import (
	"encoding/json"
	"fmt"

	"github.com/recess/game-server/internal/domain/engine"
)

// concreteState wraps engine.GameState and implements BotGameState.
// Clone uses a JSON round-trip for correctness — the same encoding the runtime
// uses when persisting state to Postgres, so any type that survives that
// round-trip is safe here.
type concreteState struct {
	state engine.GameState
}

// NewBotState wraps an engine.GameState in a BotGameState.
func NewBotState(s engine.GameState) BotGameState {
	return &concreteState{state: s}
}

func (c *concreteState) EngineState() engine.GameState {
	return c.state
}

// Clone performs a deep copy via JSON round-trip.
// This is intentionally conservative — correctness over performance.
// A hand-written clone per game can replace this if profiling shows it is a
// bottleneck (expected: ~20 000 clones per move decision at hard difficulty).
func (c *concreteState) Clone() BotGameState {
	b, err := json.Marshal(c.state)
	if err != nil {
		// This should never happen — engine.GameState is always JSON-serializable
		// because the runtime already marshals it to Postgres on every move.
		panic(fmt.Sprintf("bot: Clone: marshal: %v", err))
	}
	var copy engine.GameState
	if err := json.Unmarshal(b, &copy); err != nil {
		panic(fmt.Sprintf("bot: Clone: unmarshal: %v", err))
	}
	return &concreteState{state: copy}
}

// Payload deserializes a BotMove back into the map[string]any payload
// expected by the runtime's ApplyMove. Returns an error if the move is
// not valid JSON.
func (m BotMove) Payload() (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(m), &payload); err != nil {
		return nil, fmt.Errorf("bot: BotMove.Payload: %w", err)
	}
	return payload, nil
}

// MoveFromPayload serializes a map[string]any payload into a BotMove.
// Returns an error if the payload cannot be marshaled.
func MoveFromPayload(payload map[string]any) (BotMove, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("bot: MoveFromPayload: %w", err)
	}
	return BotMove(b), nil
}
