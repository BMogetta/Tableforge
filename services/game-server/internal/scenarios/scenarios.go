// Package scenarios embeds debug game-state fixtures and exposes a tiny
// registry. Fixtures live under data/{gameID}/{name}.json and use the
// placeholders __PLAYER_1__ … __PLAYER_N__ where real player IDs should be
// substituted at apply time.
package scenarios

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed data
var dataFS embed.FS

// Summary is the lightweight listing shape returned to the UI.
type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type fixture struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	State       json.RawMessage `json:"state"`
}

// List returns every scenario registered for a given game ID, sorted by file
// name. Returns an empty slice (not an error) if the game has no scenarios.
func List(gameID string) ([]Summary, error) {
	dir := path.Join("data", gameID)
	entries, err := dataFS.ReadDir(dir)
	if err != nil {
		// Missing directory simply means no scenarios for this game.
		return []Summary{}, nil
	}
	out := make([]Summary, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := fs.ReadFile(dataFS, path.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("scenarios: read %s: %w", e.Name(), err)
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("scenarios: parse %s: %w", e.Name(), err)
		}
		out = append(out, Summary{
			ID:          strings.TrimSuffix(e.Name(), ".json"),
			Name:        f.Name,
			Description: f.Description,
		})
	}
	return out, nil
}

// Resolve loads a fixture by id and substitutes the __PLAYER_N__ placeholders
// (1-indexed) with the supplied IDs. Returns the engine.GameState JSON, ready
// to feed into runtime.LoadState.
func Resolve(gameID, scenarioID string, playerIDs []string) ([]byte, error) {
	if strings.ContainsAny(scenarioID, "/\\.") {
		return nil, fmt.Errorf("scenarios: invalid scenario id %q", scenarioID)
	}
	raw, err := fs.ReadFile(dataFS, path.Join("data", gameID, scenarioID+".json"))
	if err != nil {
		return nil, fmt.Errorf("scenarios: not found: %s/%s", gameID, scenarioID)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("scenarios: parse %s: %w", scenarioID, err)
	}
	state := string(f.State)
	for i, id := range playerIDs {
		state = strings.ReplaceAll(state, fmt.Sprintf("__PLAYER_%d__", i+1), id)
	}
	return []byte(state), nil
}
