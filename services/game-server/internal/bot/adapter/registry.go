// Package adapter provides a registry that maps game IDs to BotAdapter
// implementations. Add a case here when a new game adapter is implemented.
package adapter

import (
	"fmt"

	loveletter "github.com/tableforge/game-server/games/loveletter"
	"github.com/tableforge/game-server/internal/bot"
	tictactoe "github.com/tableforge/game-server/internal/bot/adapter/tictactoe"
)

// New returns a BotAdapter for the given gameID.
// Returns an error if no adapter is registered for that game.
func New(gameID string) (bot.BotAdapter, error) {
	switch gameID {
	case "tictactoe":
		return tictactoe.New(), nil
	case "loveletter":
		return loveletter.New(), nil
	default:
		return nil, fmt.Errorf("bot/adapter: no adapter registered for game %q", gameID)
	}
}
