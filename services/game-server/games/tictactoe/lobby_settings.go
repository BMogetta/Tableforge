package tictactoe

import "github.com/recess/game-server/internal/domain/engine"

// LobbySettings implements engine.LobbySettingsProvider.
// TicTacToe uses the platform defaults without modification.
// Add game-specific settings here when needed (e.g. board size, winning streak).
func (g *TicTacToe) LobbySettings() []engine.LobbySetting {
	return engine.DefaultLobbySettings()
}
