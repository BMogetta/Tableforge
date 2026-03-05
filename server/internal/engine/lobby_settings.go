package engine

// SettingType describes how a lobby setting should be rendered and validated.
type SettingType string

const (
	// SettingTypeSelect renders as a dropdown with a fixed list of options.
	SettingTypeSelect SettingType = "select"

	// SettingTypeInt renders as a numeric input with optional min/max bounds.
	SettingTypeInt SettingType = "int"
)

// SettingOption is one choice inside a SettingTypeSelect setting.
type SettingOption struct {
	// Value is the raw string stored in room_settings.
	Value string `json:"value"`

	// Label is the human-readable text shown in the UI.
	Label string `json:"label"`
}

// LobbySetting describes a single configurable room setting declared by a game.
// The lobby uses this to render the settings UI and validate incoming values.
type LobbySetting struct {
	// Key is the room_settings.key value (e.g. "first_mover_policy").
	Key string `json:"key"`

	// Label is the human-readable name shown in the UI (e.g. "First move").
	Label string `json:"label"`

	// Description is optional help text shown below the control.
	Description string `json:"description,omitempty"`

	// Type determines the control type and validation rules.
	Type SettingType `json:"type"`

	// Default is the value inserted into room_settings when the room is created.
	Default string `json:"default"`

	// Options is the list of valid choices for SettingTypeSelect settings.
	// Ignored for other types.
	Options []SettingOption `json:"options,omitempty"`

	// Min and Max are the inclusive bounds for SettingTypeInt settings.
	// Ignored for other types.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// LobbySettingsProvider is an optional interface a Game can implement to declare
// which room settings it exposes in the lobby UI.
//
// If a game does not implement this interface, the lobby falls back to the
// platform-level defaults (first_mover_policy = "random", etc.).
//
// The returned slice is ordered — the UI renders settings in that order.
type LobbySettingsProvider interface {
	LobbySettings() []LobbySetting
}

// DefaultLobbySettings returns the platform-level settings that apply to every
// game, regardless of whether it implements LobbySettingsProvider.
// Games that implement LobbySettingsProvider receive these in addition to their
// own settings (the lobby merges them, with game settings taking precedence on
// key collisions).
func DefaultLobbySettings() []LobbySetting {
	return []LobbySetting{
		{
			Key:         "room_visibility",
			Label:       "Visibility",
			Description: "Public rooms appear in the lobby list. Private rooms are hidden — only joinable by code.",
			Type:        SettingTypeSelect,
			Default:     "public",
			Options: []SettingOption{
				{Value: "public", Label: "Public"},
				{Value: "private", Label: "Private"},
			},
		},
		{
			Key:         "allow_spectators",
			Label:       "Spectators",
			Description: "Allow other players to watch the game without participating.",
			Type:        SettingTypeSelect,
			Default:     "no",
			Options: []SettingOption{
				{Value: "no", Label: "Not allowed"},
				{Value: "yes", Label: "Allowed"},
			},
		},
		{
			Key:         "first_mover_policy",
			Label:       "First move",
			Description: "Who takes the first turn at the start of a game.",
			Type:        SettingTypeSelect,
			Default:     "random",
			Options: []SettingOption{
				{Value: "random", Label: "Random"},
				{Value: "fixed", Label: "Fixed seat"},
				{Value: "game_default", Label: "Decided by the game"},
			},
		},
		{
			Key:         "first_mover_seat",
			Label:       "First mover seat",
			Description: "Which seat goes first when policy is set to Fixed seat (0 = host).",
			Type:        SettingTypeInt,
			Default:     "0",
			Min:         intPtr(0),
			// Max is intentionally nil here — the upper bound is game.MaxPlayers()-1
			// and must be validated in lobby.validateSetting against the room's game.
		},
		{
			Key:         "rematch_first_mover_policy",
			Label:       "Rematch first move",
			Description: "Who takes the first turn on rematches.",
			Type:        SettingTypeSelect,
			Default:     "random",
			Options: []SettingOption{
				{Value: "random", Label: "Random"},
				{Value: "fixed", Label: "Fixed seat"},
				{Value: "game_default", Label: "Decided by the game"},
				{Value: "winner_first", Label: "Winner goes first"},
				{Value: "loser_first", Label: "Loser goes first"},
				{Value: "winner_chooses", Label: "Winner chooses"},
				{Value: "loser_chooses", Label: "Loser chooses"},
			},
		},
	}
}

func intPtr(n int) *int { return &n }
