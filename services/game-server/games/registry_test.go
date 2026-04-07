package games

import (
	"testing"

	"github.com/recess/game-server/internal/domain/engine"
)

// fakeGame implements engine.Game for testing.
type fakeGame struct {
	id string
}

func (g *fakeGame) ID() string      { return g.id }
func (g *fakeGame) Name() string    { return g.id }
func (g *fakeGame) MinPlayers() int { return 2 }
func (g *fakeGame) MaxPlayers() int { return 4 }
func (g *fakeGame) Init([]engine.Player) (engine.GameState, error) {
	return engine.GameState{}, nil
}
func (g *fakeGame) ValidateMove(engine.GameState, engine.Move) error { return nil }
func (g *fakeGame) ApplyMove(s engine.GameState, _ engine.Move) (engine.GameState, error) {
	return s, nil
}
func (g *fakeGame) IsOver(engine.GameState) (bool, engine.Result) {
	return false, engine.Result{}
}

func TestRegisterAndGet(t *testing.T) {
	// Save and restore registry state to avoid polluting other tests.
	saved := registry
	registry = map[string]engine.Game{}
	defer func() { registry = saved }()

	Register(&fakeGame{id: "test-game"})

	g, err := Get("test-game")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if g.ID() != "test-game" {
		t.Errorf("expected test-game, got %s", g.ID())
	}
}

func TestGet_NotFound(t *testing.T) {
	saved := registry
	registry = map[string]engine.Game{}
	defer func() { registry = saved }()

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("expected error for missing game")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	saved := registry
	registry = map[string]engine.Game{}
	defer func() { registry = saved }()

	Register(&fakeGame{id: "dup"})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	Register(&fakeGame{id: "dup"})
}

func TestAll(t *testing.T) {
	saved := registry
	registry = map[string]engine.Game{}
	defer func() { registry = saved }()

	Register(&fakeGame{id: "a"})
	Register(&fakeGame{id: "b"})

	all := All()
	if len(all) != 2 {
		t.Errorf("expected 2 games, got %d", len(all))
	}
}

func TestDefaultRegistry(t *testing.T) {
	saved := registry
	registry = map[string]engine.Game{}
	defer func() { registry = saved }()

	Register(&fakeGame{id: "via-adapter"})

	r := DefaultRegistry()
	g, err := r.Get("via-adapter")
	if err != nil {
		t.Fatalf("DefaultRegistry.Get: %v", err)
	}
	if g.ID() != "via-adapter" {
		t.Errorf("expected via-adapter, got %s", g.ID())
	}
}
