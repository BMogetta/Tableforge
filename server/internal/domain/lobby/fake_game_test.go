package lobby_test

import (
	"errors"

	"github.com/tableforge/server/internal/domain/engine"
)

// fakeRegistry satisfies lobby.GameRegistry.
type fakeRegistry struct {
	games map[string]engine.Game
}

func newFakeRegistry(games ...engine.Game) *fakeRegistry {
	r := &fakeRegistry{games: make(map[string]engine.Game)}
	for _, g := range games {
		r.games[g.ID()] = g
	}
	return r
}

func (r *fakeRegistry) Get(id string) (engine.Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, errors.New("game not found: " + id)
	}
	return g, nil
}

// stubGame is a minimal engine.Game implementation for tests.
type stubGame struct {
	id         string
	minPlayers int
	maxPlayers int
}

func (g *stubGame) ID() string      { return g.id }
func (g *stubGame) Name() string    { return g.id }
func (g *stubGame) MinPlayers() int { return g.minPlayers }
func (g *stubGame) MaxPlayers() int { return g.maxPlayers }

func (g *stubGame) Init(players []engine.Player) (engine.GameState, error) {
	return engine.GameState{
		CurrentPlayerID: players[0].ID,
		Data:            map[string]any{},
	}, nil
}

func (g *stubGame) ValidateMove(state engine.GameState, move engine.Move) error {
	return nil
}

func (g *stubGame) ApplyMove(state engine.GameState, move engine.Move) (engine.GameState, error) {
	return state, nil
}

func (g *stubGame) IsOver(state engine.GameState) (bool, engine.Result) {
	return false, engine.Result{}
}
