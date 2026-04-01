// Package games provides the plugin registry for all available board games.
// To add a new game, implement engine.Game and call Register() in an init() function.
package games

import (
	"fmt"

	"github.com/recess/game-server/internal/domain/engine"
)

var registry = map[string]engine.Game{}

// Register adds a game to the registry.
// It panics if a game with the same ID is registered twice.
func Register(g engine.Game) {
	if _, exists := registry[g.ID()]; exists {
		panic(fmt.Sprintf("game already registered: %s", g.ID()))
	}
	registry[g.ID()] = g
}

// Get returns a registered game by ID.
func Get(id string) (engine.Game, error) {
	g, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("game not found: %s", id)
	}
	return g, nil
}

// All returns all registered games.
func All() []engine.Game {
	list := make([]engine.Game, 0, len(registry))
	for _, g := range registry {
		list = append(list, g)
	}
	return list
}

// registryAdapter wraps the package-level registry to satisfy engine.Registry.
type registryAdapter struct{}

func (registryAdapter) Get(id string) (engine.Game, error) {
	return Get(id)
}

// DefaultRegistry returns an engine.Registry backed by the global plugin registry.
func DefaultRegistry() engine.Registry {
	return registryAdapter{}
}
