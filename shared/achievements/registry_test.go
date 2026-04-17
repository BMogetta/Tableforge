package achievements_test

import (
	"strings"
	"testing"

	"github.com/recess/shared/achievements"
	"github.com/recess/shared/events"
)

// seed installs a minimal set of definitions for tests that exercise the
// public API in isolation from any specific plugin package.
func seed(t *testing.T) {
	t.Helper()
	t.Cleanup(achievements.Reset)
	achievements.Reset()

	achievements.Register(achievements.Definition{
		Key:     "games_played",
		NameKey: achievements.NameKey("games_played"),
		GameID:  "",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("games_played", 1), DescriptionKey: achievements.TierDescriptionKey("games_played", 1)},
			{Threshold: 10, NameKey: achievements.TierNameKey("games_played", 2), DescriptionKey: achievements.TierDescriptionKey("games_played", 2)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			return cur.Progress + 1, true
		},
	})

	achievements.Register(achievements.Definition{
		Key:            "first_draw",
		NameKey:        achievements.NameKey("first_draw"),
		DescriptionKey: achievements.DescriptionKey("first_draw"),
		GameID:         "",
		Type:           achievements.TypeFlat,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("first_draw", 1), DescriptionKey: achievements.TierDescriptionKey("first_draw", 1)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, outcome string) (int, bool) {
			if outcome != "draw" {
				return cur.Progress, false
			}
			return 1, true
		},
	})

	achievements.Register(achievements.Definition{
		Key:     "ttt_only",
		NameKey: achievements.NameKey("ttt_only"),
		GameID:  "tictactoe",
		Type:    achievements.TypeTiered,
		Tiers: []achievements.Tier{
			{Threshold: 5, NameKey: achievements.TierNameKey("ttt_only", 1), DescriptionKey: achievements.TierDescriptionKey("ttt_only", 1)},
		},
		ComputeProgress: func(cur achievements.Progress, _ events.GameSessionFinished, _ string) (int, bool) {
			return cur.Progress + 1, true
		},
	})
}

func TestRegister_RejectsDuplicateKey(t *testing.T) {
	t.Cleanup(achievements.Reset)
	achievements.Reset()
	def := achievements.Definition{
		Key:     "dup",
		NameKey: achievements.NameKey("dup"),
		Type:    achievements.TypeFlat,
		Tiers: []achievements.Tier{
			{Threshold: 1, NameKey: achievements.TierNameKey("dup", 1), DescriptionKey: achievements.TierDescriptionKey("dup", 1)},
		},
	}
	achievements.Register(def)

	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	achievements.Register(def)
}

func TestRegister_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		def  achievements.Definition
	}{
		{"empty key", achievements.Definition{NameKey: "x", Type: achievements.TypeFlat, Tiers: []achievements.Tier{{Threshold: 1}}}},
		{"no name", achievements.Definition{Key: "x", Type: achievements.TypeFlat, Tiers: []achievements.Tier{{Threshold: 1}}}},
		{"bad type", achievements.Definition{Key: "x", NameKey: "x", Type: "weird", Tiers: []achievements.Tier{{Threshold: 1}}}},
		{"no tiers", achievements.Definition{Key: "x", NameKey: "x", Type: achievements.TypeFlat}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(achievements.Reset)
			achievements.Reset()
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic for %s", tc.name)
				}
			}()
			achievements.Register(tc.def)
		})
	}
}

func TestForGame_IncludesGlobalsAndGameSpecific(t *testing.T) {
	seed(t)
	global := achievements.ForGame("")
	if len(global) != 2 {
		t.Errorf("expected 2 globals, got %d", len(global))
	}
	ttt := achievements.ForGame("tictactoe")
	if len(ttt) != 3 {
		t.Errorf("expected 2 globals + 1 ttt, got %d", len(ttt))
	}
	other := achievements.ForGame("chess")
	if len(other) != 2 {
		t.Errorf("expected 2 globals (no chess-specific registered), got %d", len(other))
	}
}

func TestAll_OrdersGlobalsFirst(t *testing.T) {
	seed(t)
	all := achievements.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 definitions, got %d", len(all))
	}
	// Globals come first (empty GameID).
	for i, d := range all {
		if d.GameID == "" && i >= 2 {
			t.Errorf("global %q at position %d — globals must precede game-specific", d.Key, i)
		}
	}
	if all[2].GameID == "" {
		t.Error("expected a game-specific definition at the end")
	}
}

func TestGet_NotFound(t *testing.T) {
	seed(t)
	if _, ok := achievements.Get("nonexistent"); ok {
		t.Error("expected Get to miss on unknown key")
	}
}

func TestKeys_MatchPositionalScheme(t *testing.T) {
	seed(t)
	for _, d := range achievements.All() {
		want := "achievements." + d.Key + ".name"
		if d.NameKey != want {
			t.Errorf("%s: NameKey=%q want %q", d.Key, d.NameKey, want)
		}
		for i, tier := range d.Tiers {
			idx := i + 1
			wantTierName := "achievements." + d.Key + ".tiers." + itoa(idx) + ".name"
			wantTierDesc := "achievements." + d.Key + ".tiers." + itoa(idx) + ".description"
			if tier.NameKey != wantTierName {
				t.Errorf("%s tier %d: NameKey=%q want %q", d.Key, idx, tier.NameKey, wantTierName)
			}
			if tier.DescriptionKey != wantTierDesc {
				t.Errorf("%s tier %d: DescriptionKey=%q want %q", d.Key, idx, tier.DescriptionKey, wantTierDesc)
			}
			if !strings.HasPrefix(tier.NameKey, "achievements.") {
				t.Errorf("%s tier %d: NameKey must start with achievements.", d.Key, idx)
			}
		}
	}
}

// itoa duplicates the package-internal helper so this external _test package
// produces the same formatting when building expected keys.
func itoa(n int) string {
	if n >= 0 && n <= 9 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
