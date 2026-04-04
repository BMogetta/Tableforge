package rootaccess_test

import (
	"testing"
)

func TestLobbySettings(t *testing.T) {
	settings := game.LobbySettings()
	found := false
	for _, s := range settings {
		if s.Key == "player_count" {
			found = true
			if s.Min == nil || *s.Min != 2 {
				t.Errorf("expected min=2, got %v", s.Min)
			}
			if s.Max == nil || *s.Max != 5 {
				t.Errorf("expected max=5, got %v", s.Max)
			}
		}
	}
	if !found {
		t.Error("expected player_count setting")
	}
}
