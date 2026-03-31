package queue

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBanDurationForOffense(t *testing.T) {
	tests := []struct {
		offense  int
		expected time.Duration
	}{
		{1, 5 * time.Minute},
		{2, 25 * time.Minute},
		{3, 125 * time.Minute},
		{4, 625 * time.Minute},
		{5, 1440 * time.Minute}, // capped at 24h
		{6, 1440 * time.Minute}, // still capped
		{10, 1440 * time.Minute},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := BanDurationForOffense(tt.offense)
			if got != tt.expected {
				t.Errorf("BanDurationForOffense(%d) = %v, want %v", tt.offense, got, tt.expected)
			}
		})
	}
}

func TestBanDurationForOffense_Zero(t *testing.T) {
	// offense 0 (and negative) should be treated as offense 1 → 5 min
	for _, offense := range []int{0, -1, -100} {
		got := BanDurationForOffense(offense)
		want := 5 * time.Minute
		if got != want {
			t.Errorf("BanDurationForOffense(%d) = %v, want %v", offense, got, want)
		}
	}
}

func TestEstimatedWait(t *testing.T) {
	tests := []struct {
		position int
		expected int
	}{
		{1, 10},
		{5, 50},
		{10, 100},
		{0, 0},
	}

	for _, tt := range tests {
		got := estimatedWait(tt.position)
		if got != tt.expected {
			t.Errorf("estimatedWait(%d) = %d, want %d", tt.position, got, tt.expected)
		}
	}
}

func TestKeyHelpers(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	if got := metaKey(id); got != "queue:meta:00000000-0000-0000-0000-000000000001" {
		t.Errorf("metaKey = %q", got)
	}
	if got := pendingKey(id); got != "queue:pending:00000000-0000-0000-0000-000000000001" {
		t.Errorf("pendingKey = %q", got)
	}
	if got := declinesKey(id); got != "queue:declines:00000000-0000-0000-0000-000000000001" {
		t.Errorf("declinesKey = %q", got)
	}
	if got := banKey(id); got != "queue:ban:00000000-0000-0000-0000-000000000001" {
		t.Errorf("banKey = %q", got)
	}
}

func TestErrBanned_Error(t *testing.T) {
	err := &ErrBanned{RetryAfterSecs: 300}
	want := "queue ban active: retry after 300 seconds"
	if got := err.Error(); got != want {
		t.Errorf("ErrBanned.Error() = %q, want %q", got, want)
	}
}
