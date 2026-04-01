package loveletter

import "github.com/recess/game-server/internal/domain/engine"

// LobbySettings implements engine.LobbySettingsProvider.
func (g *LoveLetter) LobbySettings() []engine.LobbySetting {
	min2, max5 := 2, 5
	return append([]engine.LobbySetting{
		{
			Key:         "player_count",
			Label:       "Players",
			Description: "Number of players in the game (2–5).",
			Type:        engine.SettingTypeInt,
			Default:     "2",
			Min:         &min2,
			Max:         &max5,
		},
	}, engine.DefaultLobbySettings()...)
}
