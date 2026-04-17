package achievements_test

import (
	"strings"
	"testing"

	"github.com/recess/services/user-service/internal/achievements"
)

// TestRegistry_AllKeysAreI18nPaths verifies every NameKey / DescriptionKey in
// the registry follows the positional scheme consumed by the frontend
// (achievements.{Key}[.tiers.{N}].{name|description}). Protects against
// typos like 'achievement.foo.name' or skipped tiers.
func TestRegistry_AllKeysAreI18nPaths(t *testing.T) {
	for _, def := range achievements.Registry {
		wantName := "achievements." + def.Key + ".name"
		if def.NameKey != wantName {
			t.Errorf("%s: NameKey=%q want %q", def.Key, def.NameKey, wantName)
		}
		if def.Type == achievements.TypeFlat && def.DescriptionKey != "" {
			wantDesc := "achievements." + def.Key + ".description"
			if def.DescriptionKey != wantDesc {
				t.Errorf("%s: DescriptionKey=%q want %q", def.Key, def.DescriptionKey, wantDesc)
			}
		}
		for i, tier := range def.Tiers {
			idx := i + 1
			prefix := "achievements." + def.Key + ".tiers."
			wantTierName := prefix + itoa(idx) + ".name"
			wantTierDesc := prefix + itoa(idx) + ".description"
			if tier.NameKey != wantTierName {
				t.Errorf("%s tier %d: NameKey=%q want %q", def.Key, idx, tier.NameKey, wantTierName)
			}
			if tier.DescriptionKey != wantTierDesc {
				t.Errorf("%s tier %d: DescriptionKey=%q want %q", def.Key, idx, tier.DescriptionKey, wantTierDesc)
			}
			if !strings.HasPrefix(tier.NameKey, "achievements.") {
				t.Errorf("%s tier %d: NameKey must start with achievements.", def.Key, idx)
			}
		}
	}
}

// itoa duplicates the registry's internal helper so the test lives in the
// external _test package and still produces the same formatting.
func itoa(n int) string {
	if n >= 0 && n <= 9 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
