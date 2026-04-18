package consumer

import (
	"context"
	"log/slog"
	"testing"
)

// stubFlags implements featureflags.Checker without a real Unleash client.
type stubFlags struct{ enabled bool }

func (s stubFlags) IsEnabled(_ string, def bool) bool {
	if !s.enabled {
		return false
	}
	return def
}

// TestHandleSessionFinished_FlagOff_ShortCircuits verifies that when
// achievements-enabled is OFF, the consumer skips evaluation entirely —
// if it didn't, the nil store + publisher would panic on the first access.
// A successful no-panic return is the whole proof.
func TestHandleSessionFinished_FlagOff_ShortCircuits(t *testing.T) {
	c := New(nil, nil, nil, slog.Default(), stubFlags{enabled: false})

	payload := `{"session_id":"00000000-0000-0000-0000-000000000001","players":[{"player_id":"00000000-0000-0000-0000-000000000002"}]}`

	if err := c.handleSessionFinished(context.Background(), payload); err != nil {
		t.Fatalf("handleSessionFinished with flag OFF returned err: %v", err)
	}
}

// TestHandleSessionFinished_MalformedPayload_FlagOff_StillErrors confirms
// parse errors escape before the gate — we want broken events to surface
// regardless of flag state.
func TestHandleSessionFinished_MalformedPayload_FlagOff_StillErrors(t *testing.T) {
	c := New(nil, nil, nil, slog.Default(), stubFlags{enabled: false})

	if err := c.handleSessionFinished(context.Background(), "not-json"); err == nil {
		t.Fatal("expected parse error for malformed payload even with flag OFF")
	}
}
