// Package achievements defines achievement definitions and evaluation logic.
package achievements

// Tier represents a single milestone within an achievement.
type Tier struct {
	Threshold   int
	Name        string
	Description string
}

// Definition describes an achievement that can be earned by players.
type Definition struct {
	Key         string
	Name        string // display name
	Description string // supports {threshold} template for tiered
	GameID      string // "" = global, "tictactoe" = game-specific
	Type        string // "flat" | "tiered"
	Tiers       []Tier
}

const (
	TypeFlat   = "flat"
	TypeTiered = "tiered"
)

// Registry holds all known achievement definitions.
var Registry = []Definition{
	// --- Global achievements ---
	{
		Key:    "games_played",
		Name:   "Player",
		GameID: "",
		Type:   TypeTiered,
		Tiers: []Tier{
			{Threshold: 1, Name: "Newcomer", Description: "Play your first game"},
			{Threshold: 10, Name: "Regular", Description: "Play 10 games"},
			{Threshold: 50, Name: "Dedicated", Description: "Play 50 games"},
			{Threshold: 100, Name: "Veteran", Description: "Play 100 games"},
			{Threshold: 500, Name: "Legend", Description: "Play 500 games"},
		},
	},
	{
		Key:    "games_won",
		Name:   "Winner",
		GameID: "",
		Type:   TypeTiered,
		Tiers: []Tier{
			{Threshold: 1, Name: "First Blood", Description: "Win your first game"},
			{Threshold: 10, Name: "Skilled", Description: "Win 10 games"},
			{Threshold: 50, Name: "Dominant", Description: "Win 50 games"},
			{Threshold: 100, Name: "Champion", Description: "Win 100 games"},
		},
	},
	{
		Key:    "win_streak",
		Name:   "On Fire",
		GameID: "",
		Type:   TypeTiered,
		Tiers: []Tier{
			{Threshold: 3, Name: "Hot Streak", Description: "Win 3 games in a row"},
			{Threshold: 5, Name: "Unstoppable", Description: "Win 5 games in a row"},
			{Threshold: 10, Name: "Legendary", Description: "Win 10 games in a row"},
		},
	},
	{
		Key:         "first_draw",
		Name:        "Stalemate",
		Description: "Draw a game",
		GameID:      "",
		Type:        TypeFlat,
		Tiers: []Tier{
			{Threshold: 1, Name: "Stalemate", Description: "Draw a game"},
		},
	},

	// --- TicTacToe achievements ---
	{
		Key:         "ttt_perfect_game",
		Name:        "Perfect Game",
		Description: "Win a tic-tac-toe game in 3 moves",
		GameID:      "tictactoe",
		Type:        TypeFlat,
		Tiers: []Tier{
			{Threshold: 1, Name: "Perfect Game", Description: "Win in the minimum possible moves"},
		},
	},
	{
		Key:    "ttt_games_played",
		Name:   "Tic-Tac-Toe Fan",
		GameID: "tictactoe",
		Type:   TypeTiered,
		Tiers: []Tier{
			{Threshold: 5, Name: "Beginner", Description: "Play 5 tic-tac-toe games"},
			{Threshold: 25, Name: "Enthusiast", Description: "Play 25 tic-tac-toe games"},
			{Threshold: 100, Name: "Addict", Description: "Play 100 tic-tac-toe games"},
		},
	},
}

// index is a pre-built map for fast lookup by key.
var index map[string]Definition

func init() {
	index = make(map[string]Definition, len(Registry))
	for _, d := range Registry {
		index[d.Key] = d
	}
}

// Get returns the definition for the given key, or false if not found.
func Get(key string) (Definition, bool) {
	d, ok := index[key]
	return d, ok
}

// ForGame returns all definitions that apply to the given game ID.
// Pass "" for global-only achievements.
func ForGame(gameID string) []Definition {
	var out []Definition
	for _, d := range Registry {
		if d.GameID == "" || d.GameID == gameID {
			out = append(out, d)
		}
	}
	return out
}
