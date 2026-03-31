package bot_test

import (
	"os"
	"testing"
	"time"

	"github.com/tableforge/game-server/internal/bot"
)

func TestDefaultConfig_UsesMedianProfile(t *testing.T) {
	// Clear any env vars that might affect the result.
	os.Unsetenv("BOT_ITERATIONS")
	os.Unsetenv("BOT_DETERMINIZATIONS")
	os.Unsetenv("BOT_MAX_THINK_TIME")
	os.Unsetenv("BOT_EXPLORATION_C")

	cfg := bot.DefaultConfig()

	if cfg.Personality.Name != "medium" {
		t.Errorf("expected medium profile, got %s", cfg.Personality.Name)
	}
	if cfg.Iterations != 500 {
		t.Errorf("expected 500 iterations, got %d", cfg.Iterations)
	}
	if cfg.Determinizations != 10 {
		t.Errorf("expected 10 determinizations, got %d", cfg.Determinizations)
	}
	if cfg.MaxThinkTime != 2*time.Second {
		t.Errorf("expected 2s think time, got %s", cfg.MaxThinkTime)
	}
}

func TestDefaultConfig_EnvVarsOverride(t *testing.T) {
	os.Setenv("BOT_ITERATIONS", "999")
	os.Setenv("BOT_DETERMINIZATIONS", "15")
	os.Setenv("BOT_MAX_THINK_TIME", "3s")
	os.Setenv("BOT_EXPLORATION_C", "1.2")
	defer func() {
		os.Unsetenv("BOT_ITERATIONS")
		os.Unsetenv("BOT_DETERMINIZATIONS")
		os.Unsetenv("BOT_MAX_THINK_TIME")
		os.Unsetenv("BOT_EXPLORATION_C")
	}()

	cfg := bot.DefaultConfig()

	if cfg.Iterations != 999 {
		t.Errorf("expected 999 iterations, got %d", cfg.Iterations)
	}
	if cfg.Determinizations != 15 {
		t.Errorf("expected 15 determinizations, got %d", cfg.Determinizations)
	}
	if cfg.MaxThinkTime != 3*time.Second {
		t.Errorf("expected 3s, got %s", cfg.MaxThinkTime)
	}
	if cfg.ExplorationC != 1.2 {
		t.Errorf("expected 1.2, got %f", cfg.ExplorationC)
	}
}

func TestDefaultConfig_InvalidEnvVarsIgnored(t *testing.T) {
	os.Setenv("BOT_ITERATIONS", "not-a-number")
	os.Setenv("BOT_MAX_THINK_TIME", "not-a-duration")
	defer func() {
		os.Unsetenv("BOT_ITERATIONS")
		os.Unsetenv("BOT_MAX_THINK_TIME")
	}()

	cfg := bot.DefaultConfig()

	// Should fall back to profile defaults.
	if cfg.Iterations != 500 {
		t.Errorf("expected fallback to 500, got %d", cfg.Iterations)
	}
	if cfg.MaxThinkTime != 2*time.Second {
		t.Errorf("expected fallback to 2s, got %s", cfg.MaxThinkTime)
	}
}

func TestConfigFromProfile_KnownProfiles(t *testing.T) {
	os.Unsetenv("BOT_MAX_THINK_TIME")

	profiles := []string{"easy", "medium", "hard", "aggressive"}
	for _, name := range profiles {
		cfg, err := bot.ConfigFromProfile(name)
		if err != nil {
			t.Errorf("ConfigFromProfile(%q): unexpected error: %v", name, err)
			continue
		}
		if cfg.Personality.Name != name {
			t.Errorf("expected profile %q, got %q", name, cfg.Personality.Name)
		}
		if cfg.Iterations <= 0 {
			t.Errorf("profile %q: iterations must be > 0", name)
		}
		if cfg.Determinizations <= 0 {
			t.Errorf("profile %q: determinizations must be > 0", name)
		}
		if cfg.ExplorationC <= 0 {
			t.Errorf("profile %q: exploration constant must be > 0", name)
		}
	}
}

func TestConfigFromProfile_UnknownProfile(t *testing.T) {
	_, err := bot.ConfigFromProfile("legendary")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestProfiles_AllHaveValidParameters(t *testing.T) {
	for name, p := range bot.Profiles {
		if p.Iterations <= 0 {
			t.Errorf("profile %q: iterations must be > 0", name)
		}
		if p.Determinizations <= 0 {
			t.Errorf("profile %q: determinizations must be > 0", name)
		}
		if p.ExplorationC <= 0 {
			t.Errorf("profile %q: exploration constant must be > 0", name)
		}
		if p.Aggressiveness < 0 || p.Aggressiveness > 1 {
			t.Errorf("profile %q: aggressiveness must be in [0,1]", name)
		}
		if p.RiskAversion < 0 || p.RiskAversion > 1 {
			t.Errorf("profile %q: risk aversion must be in [0,1]", name)
		}
	}
}
