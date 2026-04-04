// Package adapter provides a registry that maps game IDs to BotAdapter
// implementations. Add a case here when a new game adapter is implemented.
package adapter

import (
	"fmt"

	rootaccess "github.com/recess/game-server/games/rootaccess"
	"github.com/recess/game-server/internal/bot"
	tictactoe "github.com/recess/game-server/internal/bot/adapter/tictactoe"
)

// New returns a BotAdapter for the given gameID.
// Returns an error if no adapter is registered for that game.
func New(gameID string) (bot.BotAdapter, error) {
	switch gameID {
	case "tictactoe":
		return tictactoe.New(), nil
	case "rootaccess":
		return rootaccess.New(), nil
	default:
		return nil, fmt.Errorf("bot/adapter: no adapter registered for game %q", gameID)
	}
}
