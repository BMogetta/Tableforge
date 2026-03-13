package store_test

import (
	"context"
	"testing"

	"github.com/tableforge/server/internal/platform/store"
)

// --- Helpers -----------------------------------------------------------------

// setupRoom creates an owner, a room, and adds both owner and guest as players.
func setupRoom(t *testing.T, s store.Store) (owner store.Player, guest store.Player, room store.Room) {
	t.Helper()
	ctx := context.Background()

	owner, _ = s.CreatePlayer(ctx, "alice")
	guest, _ = s.CreatePlayer(ctx, "bob")

	room, _ = s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "CHAT0001",
		GameID:     "chess",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})

	_ = s.AddPlayerToRoom(ctx, room.ID, owner.ID, 0)
	_ = s.AddPlayerToRoom(ctx, room.ID, guest.ID, 1)

	return owner, guest, room
}

// --- Room chat ---------------------------------------------------------------

func TestSaveAndGetRoomMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner, guest, room := setupRoom(t, s)

	msg1, err := s.SaveRoomMessage(ctx, room.ID, owner.ID, "hello")
	if err != nil {
		t.Fatalf("SaveRoomMessage: %v", err)
	}
	if msg1.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", msg1.Content)
	}

	_, err = s.SaveRoomMessage(ctx, room.ID, guest.ID, "world")
	if err != nil {
		t.Fatalf("SaveRoomMessage: %v", err)
	}

	messages, err := s.GetRoomMessages(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoomMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "hello" {
		t.Errorf("expected first message 'hello', got %q", messages[0].Content)
	}
	if messages[1].Content != "world" {
		t.Errorf("expected second message 'world', got %q", messages[1].Content)
	}
}

func TestHideRoomMessage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner, _, room := setupRoom(t, s)

	msg, err := s.SaveRoomMessage(ctx, room.ID, owner.ID, "offensive content")
	if err != nil {
		t.Fatalf("SaveRoomMessage: %v", err)
	}
	if msg.Hidden {
		t.Fatal("expected message to not be hidden initially")
	}

	if err := s.HideRoomMessage(ctx, msg.ID); err != nil {
		t.Fatalf("HideRoomMessage: %v", err)
	}

	messages, err := s.GetRoomMessages(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoomMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if !messages[0].Hidden {
		t.Error("expected message to be hidden")
	}
}

func TestReportRoomMessage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner, _, room := setupRoom(t, s)

	msg, err := s.SaveRoomMessage(ctx, room.ID, owner.ID, "suspicious content")
	if err != nil {
		t.Fatalf("SaveRoomMessage: %v", err)
	}

	if err := s.ReportRoomMessage(ctx, msg.ID); err != nil {
		t.Fatalf("ReportRoomMessage: %v", err)
	}

	messages, err := s.GetRoomMessages(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoomMessages: %v", err)
	}
	if !messages[0].Reported {
		t.Error("expected message to be reported")
	}
}

// --- Player mutes ------------------------------------------------------------

func TestMuteAndUnmutePlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	muter, _ := s.CreatePlayer(ctx, "carol")
	muted, _ := s.CreatePlayer(ctx, "dave")

	if err := s.MutePlayer(ctx, muter.ID, muted.ID); err != nil {
		t.Fatalf("MutePlayer: %v", err)
	}

	mutes, err := s.GetMutedPlayers(ctx, muter.ID)
	if err != nil {
		t.Fatalf("GetMutedPlayers: %v", err)
	}
	if len(mutes) != 1 {
		t.Fatalf("expected 1 mute, got %d", len(mutes))
	}
	if mutes[0].MutedID != muted.ID {
		t.Errorf("expected muted_id %s, got %s", muted.ID, mutes[0].MutedID)
	}

	if err := s.UnmutePlayer(ctx, muter.ID, muted.ID); err != nil {
		t.Fatalf("UnmutePlayer: %v", err)
	}

	mutes, err = s.GetMutedPlayers(ctx, muter.ID)
	if err != nil {
		t.Fatalf("GetMutedPlayers after unmute: %v", err)
	}
	if len(mutes) != 0 {
		t.Errorf("expected 0 mutes after unmute, got %d", len(mutes))
	}
}

func TestMutePlayer_Idempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	muter, _ := s.CreatePlayer(ctx, "eve")
	muted, _ := s.CreatePlayer(ctx, "frank")

	// Muting twice should not error.
	if err := s.MutePlayer(ctx, muter.ID, muted.ID); err != nil {
		t.Fatalf("MutePlayer first: %v", err)
	}
	if err := s.MutePlayer(ctx, muter.ID, muted.ID); err != nil {
		t.Fatalf("MutePlayer second (idempotent): %v", err)
	}

	mutes, _ := s.GetMutedPlayers(ctx, muter.ID)
	if len(mutes) != 1 {
		t.Errorf("expected 1 mute after double mute, got %d", len(mutes))
	}
}

// --- Ratings -----------------------------------------------------------------

func TestUpsertAndGetRating(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "grace")

	r := store.Rating{
		PlayerID:      player.ID,
		GameID:        "chess",
		MMR:           1600,
		DisplayRating: 1600,
		GamesPlayed:   5,
		WinStreak:     2,
		LossStreak:    0,
	}

	if err := s.UpsertRating(ctx, r); err != nil {
		t.Fatalf("UpsertRating: %v", err)
	}

	fetched, err := s.GetRating(ctx, player.ID, "chess")
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if fetched.MMR != 1600 {
		t.Errorf("expected MMR 1600, got %f", fetched.MMR)
	}
	if fetched.GamesPlayed != 5 {
		t.Errorf("expected games_played 5, got %d", fetched.GamesPlayed)
	}

	// Upsert again — should update, not insert.
	r.MMR = 1650
	r.GamesPlayed = 6
	if err := s.UpsertRating(ctx, r); err != nil {
		t.Fatalf("UpsertRating update: %v", err)
	}

	fetched, err = s.GetRating(ctx, player.ID, "chess")
	if err != nil {
		t.Fatalf("GetRating after update: %v", err)
	}
	if fetched.MMR != 1650 {
		t.Errorf("expected MMR 1650 after update, got %f", fetched.MMR)
	}
}

func TestGetRatingLeaderboard(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p1, _ := s.CreatePlayer(ctx, "henry")
	p2, _ := s.CreatePlayer(ctx, "iris")
	p3, _ := s.CreatePlayer(ctx, "jack")

	_ = s.UpsertRating(ctx, store.Rating{PlayerID: p1.ID, GameID: "chess", MMR: 1800, DisplayRating: 1800})
	_ = s.UpsertRating(ctx, store.Rating{PlayerID: p2.ID, GameID: "chess", MMR: 1500, DisplayRating: 1500})
	_ = s.UpsertRating(ctx, store.Rating{PlayerID: p3.ID, GameID: "chess", MMR: 1650, DisplayRating: 1650})

	entries, err := s.GetRatingLeaderboard(ctx, "chess", 10)
	if err != nil {
		t.Fatalf("GetRatingLeaderboard: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Should be ordered by MMR descending.
	if entries[0].PlayerID != p1.ID {
		t.Errorf("expected first place player %s, got %s", p1.ID, entries[0].PlayerID)
	}
	if entries[1].PlayerID != p3.ID {
		t.Errorf("expected second place player %s, got %s", p3.ID, entries[1].PlayerID)
	}
	if entries[2].PlayerID != p2.ID {
		t.Errorf("expected third place player %s, got %s", p2.ID, entries[2].PlayerID)
	}
}

func TestGetRatingLeaderboard_Limit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i, name := range []string{"k1", "k2", "k3", "k4", "k5"} {
		p, _ := s.CreatePlayer(ctx, name)
		_ = s.UpsertRating(ctx, store.Rating{PlayerID: p.ID, GameID: "chess", MMR: float64(1500 + i*10)})
	}

	entries, err := s.GetRatingLeaderboard(ctx, "chess", 3)
	if err != nil {
		t.Fatalf("GetRatingLeaderboard with limit: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with limit 3, got %d", len(entries))
	}
}
