// Package adapter hosts a plugin registry that maps game IDs to BotAdapter
// factories. The registry itself knows no specific game — each adapter
// package registers itself from an init() function, so adding a new game
// requires blank-importing the adapter from the binary's main package
// (mirroring how engine games register via games.Register).
package adapter

import (
	"fmt"
	"sync"

	"github.com/recess/game-server/internal/bot"
)

// Factory is how an adapter package exposes itself to the registry.
// NewWithProfile is optional: adapters whose rollout policy does not depend
// on personality may leave it nil, in which case NewWithProfile falls back
// to New and ignores the profile.
type Factory struct {
	New            func() bot.BotAdapter
	NewWithProfile func(profile bot.PersonalityProfile) bot.BotAdapter
}

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register stores an adapter factory under gameID. Intended for adapter
// package init() functions. Panics on duplicate IDs so conflicts surface at
// startup, not silently at call time.
func Register(gameID string, f Factory) {
	if f.New == nil {
		panic(fmt.Sprintf("bot/adapter: Factory.New is nil for %q", gameID))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[gameID]; exists {
		panic(fmt.Sprintf("bot/adapter: duplicate registration for game %q", gameID))
	}
	registry[gameID] = f
}

// New returns a BotAdapter for the given gameID with default rollouts.
func New(gameID string) (bot.BotAdapter, error) {
	mu.RLock()
	f, ok := registry[gameID]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bot/adapter: no adapter registered for game %q", gameID)
	}
	return f.New(), nil
}

// NewWithProfile returns a BotAdapter whose rollout policy is biased by the
// given personality profile. Adapters without personality support silently
// ignore the profile.
func NewWithProfile(gameID string, profile bot.PersonalityProfile) (bot.BotAdapter, error) {
	mu.RLock()
	f, ok := registry[gameID]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bot/adapter: no adapter registered for game %q", gameID)
	}
	if f.NewWithProfile != nil {
		return f.NewWithProfile(profile), nil
	}
	return f.New(), nil
}
