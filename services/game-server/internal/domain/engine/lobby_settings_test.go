package engine

import "testing"

func TestDefaultLobbySettings_HasExpectedKeys(t *testing.T) {
	settings := DefaultLobbySettings()

	expectedKeys := []string{
		"room_visibility",
		"allow_spectators",
		"first_mover_policy",
		"first_mover_seat",
		"rematch_first_mover_policy",
	}

	if len(settings) != len(expectedKeys) {
		t.Fatalf("expected %d default settings, got %d", len(expectedKeys), len(settings))
	}

	for i, s := range settings {
		if s.Key != expectedKeys[i] {
			t.Errorf("settings[%d].Key = %q, want %q", i, s.Key, expectedKeys[i])
		}
		if s.Label == "" {
			t.Errorf("settings[%d] (%s) has empty label", i, s.Key)
		}
		if s.Default == "" {
			t.Errorf("settings[%d] (%s) has empty default", i, s.Key)
		}
	}
}

func TestDefaultLobbySettings_SelectTypesHaveOptions(t *testing.T) {
	for _, s := range DefaultLobbySettings() {
		if s.Type == SettingTypeSelect && len(s.Options) == 0 {
			t.Errorf("select setting %q has no options", s.Key)
		}
	}
}

func TestDefaultLobbySettings_IntTypeHasMinMax(t *testing.T) {
	for _, s := range DefaultLobbySettings() {
		if s.Type == SettingTypeInt {
			if s.Min == nil {
				t.Errorf("int setting %q has no Min", s.Key)
			}
		}
	}
}

func TestIntPtr(t *testing.T) {
	got := intPtr(42)
	if got == nil || *got != 42 {
		t.Errorf("intPtr(42) = %v, want pointer to 42", got)
	}
}
