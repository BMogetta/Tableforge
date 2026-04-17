package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/recess/shared/achievements"
	_ "github.com/recess/shared/achievements/games/tictactoe"
	_ "github.com/recess/shared/achievements/global"
)

// TestListAchievementDefinitions_ReturnsRegistry verifies the definitions
// endpoint serializes the aggregated Registry — i18n keys only, no English
// text, ComputeProgress closures stripped.
func TestListAchievementDefinitions_ReturnsRegistry(t *testing.T) {
	router := newTestRouter(newMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/achievements/definitions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Definitions []struct {
			Key            string `json:"key"`
			NameKey        string `json:"name_key"`
			DescriptionKey string `json:"description_key,omitempty"`
			GameID         string `json:"game_id"`
			Type           string `json:"type"`
			Tiers          []struct {
				Threshold      int    `json:"threshold"`
				NameKey        string `json:"name_key"`
				DescriptionKey string `json:"description_key"`
			} `json:"tiers"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Definitions) == 0 {
		t.Fatal("expected at least one definition")
	}

	// Every response entry must match what's in the server-side registry by key.
	gotByKey := map[string]struct{}{}
	for _, d := range resp.Definitions {
		gotByKey[d.Key] = struct{}{}
		def, ok := achievements.Get(d.Key)
		if !ok {
			t.Errorf("response carries %q but registry has no such entry", d.Key)
			continue
		}
		if d.NameKey != def.NameKey {
			t.Errorf("%s: name_key=%q want %q", d.Key, d.NameKey, def.NameKey)
		}
		if d.Type != def.Type {
			t.Errorf("%s: type=%q want %q", d.Key, d.Type, def.Type)
		}
		if d.GameID != def.GameID {
			t.Errorf("%s: game_id=%q want %q", d.Key, d.GameID, def.GameID)
		}
		if len(d.Tiers) != len(def.Tiers) {
			t.Errorf("%s: got %d tiers, want %d", d.Key, len(d.Tiers), len(def.Tiers))
		}
	}

	// Every registered key must appear in the response — no filtering / drift.
	for _, def := range achievements.All() {
		if _, ok := gotByKey[def.Key]; !ok {
			t.Errorf("response missing %q from registry", def.Key)
		}
	}
}
