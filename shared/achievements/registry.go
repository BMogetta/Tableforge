package achievements

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Definition{}
	order    []string // preserves registration order so listings are stable
)

// Register adds a Definition to the global registry. Intended for init()
// bodies in leaf packages (shared/achievements/global, ...games/{game}).
// Panics on duplicate keys so conflicts surface at startup, not silently.
func Register(def Definition) {
	if def.Key == "" {
		panic("achievements: Register called with empty Key")
	}
	if def.NameKey == "" {
		panic(fmt.Sprintf("achievements: Register(%q) missing NameKey", def.Key))
	}
	if def.Type != TypeFlat && def.Type != TypeTiered {
		panic(fmt.Sprintf("achievements: Register(%q) invalid Type %q", def.Key, def.Type))
	}
	if len(def.Tiers) == 0 {
		panic(fmt.Sprintf("achievements: Register(%q) has no Tiers", def.Key))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[def.Key]; exists {
		panic(fmt.Sprintf("achievements: duplicate registration for %q", def.Key))
	}
	registry[def.Key] = def
	order = append(order, def.Key)
}

// Get returns the Definition for the given key, or false if not registered.
func Get(key string) (Definition, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[key]
	return d, ok
}

// All returns every registered Definition, sorted so globals come first
// (GameID == "") and game-specific definitions follow grouped by GameID.
// Within a group, registration order is preserved.
func All() []Definition {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Definition, 0, len(registry))
	for _, k := range order {
		out = append(out, registry[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GameID == out[j].GameID {
			return false // preserve original order within group
		}
		if out[i].GameID == "" {
			return true
		}
		if out[j].GameID == "" {
			return false
		}
		return out[i].GameID < out[j].GameID
	})
	return out
}

// ForGame returns all Definitions that apply to the given game ID, always
// including globals. Pass "" for global-only.
func ForGame(gameID string) []Definition {
	mu.RLock()
	defer mu.RUnlock()
	var out []Definition
	for _, k := range order {
		d := registry[k]
		if d.GameID == "" || d.GameID == gameID {
			out = append(out, d)
		}
	}
	return out
}

// Reset clears the registry. Test-only: guarded by `testing` import so it
// cannot accidentally run in production code paths.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]Definition{}
	order = nil
}
