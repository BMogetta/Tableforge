package bot

import (
	"fmt"
	"strconv"
	"time"

	"github.com/recess/shared/config"
)

// PersonalityProfile is a named preset that controls bot strength and style.
// The MCTS parameters determine search depth; the heuristic weights are
// passed to the game adapter for use in rollout/evaluation heuristics.
type PersonalityProfile struct {
	// Name is the unique identifier for this profile (e.g. "easy", "hard").
	Name string

	// MCTS parameters.
	Iterations       int
	Determinizations int
	ExplorationC     float64

	// Heuristic weights in [0.0, 1.0].
	// Interpreted per game adapter — the MCTS engine itself ignores them.
	Aggressiveness float64 // 0.0 = passive, 1.0 = aggressive
	RiskAversion   float64 // 0.0 = risk-loving, 1.0 = risk-averse

	// Response-time envelope used by the bot-runner to pace moves so humans
	// perceive the bot as thinking rather than reacting instantly. Total
	// delay is drawn from [MinMoveDelay, MinMoveDelay+MoveDelayJitter] and
	// MCTS compute time counts against it — fast MCTS fills the rest with
	// sleep, slow MCTS submits immediately.
	//
	// Consumers that don't care about pacing (integrated server-side bot,
	// benchmarks) can ignore these fields.
	MinMoveDelay    time.Duration
	MoveDelayJitter time.Duration
}

// Profiles contains the built-in personality presets.
// Keys match the profile Name field.
var Profiles = map[string]PersonalityProfile{
	"easy": {
		Name:             "easy",
		Iterations:       100,
		Determinizations: 5,
		ExplorationC:     2.0,
		Aggressiveness:   0.3,
		RiskAversion:     0.7,
		// Beginners hesitate — longer floor, wider jitter so rhythm varies.
		MinMoveDelay:    900 * time.Millisecond,
		MoveDelayJitter: 1200 * time.Millisecond,
	},
	"medium": {
		Name:             "medium",
		Iterations:       500,
		Determinizations: 10,
		ExplorationC:     1.41,
		Aggressiveness:   0.5,
		RiskAversion:     0.5,
		MinMoveDelay:    1500 * time.Millisecond,
		MoveDelayJitter: 1500 * time.Millisecond,
	},
	"hard": {
		Name:             "hard",
		Iterations:       2000,
		Determinizations: 30,
		ExplorationC:     1.0,
		Aggressiveness:   0.7,
		RiskAversion:     0.3,
		// "Thoughtful" feel — slower floor, comparable jitter.
		MinMoveDelay:    2200 * time.Millisecond,
		MoveDelayJitter: 1800 * time.Millisecond,
	},
	"aggressive": {
		Name:             "aggressive",
		Iterations:       1000,
		Determinizations: 20,
		ExplorationC:     0.8,
		Aggressiveness:   1.0,
		RiskAversion:     0.1,
		// Impulsive — snap decisions, narrow jitter.
		MinMoveDelay:    500 * time.Millisecond,
		MoveDelayJitter: 500 * time.Millisecond,
	},
}

// BotConfig holds the runtime configuration for a single bot instance.
// MaxThinkTime is a hard deadline — the bot must return a move before it
// expires, regardless of how many MCTS iterations have completed.
type BotConfig struct {
	Iterations       int
	Determinizations int
	MaxThinkTime     time.Duration
	ExplorationC     float64
	Personality      PersonalityProfile
}

// DefaultConfig returns a BotConfig with the "medium" personality and
// values loaded from environment variables (falling back to profile defaults).
//
// Environment variables:
//
//	BOT_ITERATIONS         — overrides Personality.Iterations
//	BOT_DETERMINIZATIONS   — overrides Personality.Determinizations
//	BOT_MAX_THINK_TIME     — e.g. "2s", "500ms" (parsed by time.ParseDuration)
//	BOT_EXPLORATION_C      — overrides Personality.ExplorationC
func DefaultConfig() BotConfig {
	profile := Profiles["medium"]
	cfg := BotConfig{
		Iterations:       profile.Iterations,
		Determinizations: profile.Determinizations,
		MaxThinkTime:     2 * time.Second,
		ExplorationC:     profile.ExplorationC,
		Personality:      profile,
	}

	if v := config.Env("BOT_ITERATIONS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Iterations = n
		}
	}
	if v := config.Env("BOT_DETERMINIZATIONS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Determinizations = n
		}
	}
	if v := config.Env("BOT_MAX_THINK_TIME", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MaxThinkTime = d
		}
	}
	if v := config.Env("BOT_EXPLORATION_C", ""); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.ExplorationC = f
		}
	}

	return cfg
}

// ConfigFromProfile returns a BotConfig for the named personality profile.
// Returns an error if the profile name is not found in Profiles.
// MaxThinkTime is loaded from BOT_MAX_THINK_TIME or defaults to 2s.
func ConfigFromProfile(profileName string) (BotConfig, error) {
	profile, ok := Profiles[profileName]
	if !ok {
		return BotConfig{}, fmt.Errorf("bot: unknown personality profile %q", profileName)
	}

	cfg := BotConfig{
		Iterations:       profile.Iterations,
		Determinizations: profile.Determinizations,
		MaxThinkTime:     2 * time.Second,
		ExplorationC:     profile.ExplorationC,
		Personality:      profile,
	}

	if v := config.Env("BOT_MAX_THINK_TIME", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.MaxThinkTime = d
		}
	}

	return cfg, nil
}
