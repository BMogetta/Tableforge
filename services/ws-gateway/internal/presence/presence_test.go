package presence

import (
	"testing"

	"github.com/google/uuid"
)

func TestKey(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got := key(id)
	want := "presence:11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Errorf("key(%s) = %q, want %q", id, got, want)
	}
}

func TestListOnline_Empty(t *testing.T) {
	s := &Store{} // rdb is nil but won't be called for empty slice
	result, err := s.ListOnline(nil, []uuid.UUID{})
	if err != nil {
		t.Fatalf("ListOnline empty: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}
