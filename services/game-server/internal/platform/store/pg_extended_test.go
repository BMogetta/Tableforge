package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/platform/store"
)

// --- helpers -----------------------------------------------------------------

// setupRoom creates an owner, a guest, a room with both players seated, and
// returns everything needed for session/voting/result tests.
func setupRoom(t *testing.T, s store.Store) (owner, guest store.Player, room store.Room) {
	t.Helper()
	ctx := context.Background()

	owner, err := s.CreatePlayer(ctx, "owner-"+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("CreatePlayer owner: %v", err)
	}
	guest, err = s.CreatePlayer(ctx, "guest-"+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("CreatePlayer guest: %v", err)
	}
	room, err = s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       uuid.NewString()[:8],
		GameID:     "tictactoe",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := s.AddPlayerToRoom(ctx, room.ID, owner.ID, 0); err != nil {
		t.Fatalf("AddPlayerToRoom owner: %v", err)
	}
	if err := s.AddPlayerToRoom(ctx, room.ID, guest.ID, 1); err != nil {
		t.Fatalf("AddPlayerToRoom guest: %v", err)
	}
	return owner, guest, room
}

func createSession(t *testing.T, s store.Store, roomID uuid.UUID) store.GameSession {
	t.Helper()
	state := []byte(`{"current_player_id":"` + uuid.New().String() + `","data":{}}`)
	session, err := s.CreateGameSession(context.Background(), roomID, "tictactoe", state, nil, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("CreateGameSession: %v", err)
	}
	return session
}

func createSessionWithTimeout(t *testing.T, s store.Store, roomID uuid.UUID, timeout int) store.GameSession {
	t.Helper()
	state := []byte(`{"current_player_id":"` + uuid.New().String() + `","data":{}}`)
	session, err := s.CreateGameSession(context.Background(), roomID, "tictactoe", state, &timeout, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("CreateGameSession: %v", err)
	}
	return session
}

// --- Player tests ------------------------------------------------------------

func TestCreateBotPlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	bot, err := s.CreateBotPlayer(ctx, "bot-alice")
	if err != nil {
		t.Fatalf("CreateBotPlayer: %v", err)
	}
	if !bot.IsBot {
		t.Error("expected IsBot = true")
	}
	if bot.Username != "bot-alice" {
		t.Errorf("expected username bot-alice, got %s", bot.Username)
	}
}

func TestGetPlayerByUsername(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, _ := s.CreatePlayer(ctx, "findme")
	found, err := s.GetPlayerByUsername(ctx, "findme")
	if err != nil {
		t.Fatalf("GetPlayerByUsername: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected %s, got %s", created.ID, found.ID)
	}
}

func TestSoftDeletePlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, _ := s.CreatePlayer(ctx, "deleteme")
	if err := s.SoftDeletePlayer(ctx, p.ID); err != nil {
		t.Fatalf("SoftDeletePlayer: %v", err)
	}

	// GetPlayer should fail (filters deleted_at IS NULL).
	_, err := s.GetPlayer(ctx, p.ID)
	if err == nil {
		t.Error("expected error getting soft-deleted player")
	}
}

// --- Room tests --------------------------------------------------------------

func TestGetRoomByCode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "codeowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "FIND1234", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})

	found, err := s.GetRoomByCode(ctx, "FIND1234")
	if err != nil {
		t.Fatalf("GetRoomByCode: %v", err)
	}
	if found.ID != room.ID {
		t.Errorf("expected room %s, got %s", room.ID, found.ID)
	}
}

func TestUpdateRoomStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "statusowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "STAT0001", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})

	if err := s.UpdateRoomStatus(ctx, room.ID, store.RoomStatusInProgress); err != nil {
		t.Fatalf("UpdateRoomStatus: %v", err)
	}

	fetched, _ := s.GetRoom(ctx, room.ID)
	if fetched.Status != store.RoomStatusInProgress {
		t.Errorf("expected in_progress, got %s", fetched.Status)
	}
}

func TestListAndCountWaitingRooms(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "listowner")

	for i := 0; i < 3; i++ {
		s.CreateRoom(ctx, store.CreateRoomParams{
			Code: uuid.NewString()[:8], GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
		})
	}

	rooms, err := s.ListWaitingRooms(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListWaitingRooms: %v", err)
	}
	if len(rooms) < 3 {
		t.Errorf("expected at least 3 waiting rooms, got %d", len(rooms))
	}

	count, err := s.CountWaitingRooms(ctx)
	if err != nil {
		t.Fatalf("CountWaitingRooms: %v", err)
	}
	if count < 3 {
		t.Errorf("expected at least 3, got %d", count)
	}
}

func TestListWaitingRooms_Pagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "pageowner")
	for i := 0; i < 5; i++ {
		s.CreateRoom(ctx, store.CreateRoomParams{
			Code: uuid.NewString()[:8], GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
		})
	}

	page1, _ := s.ListWaitingRooms(ctx, 2, 0)
	page2, _ := s.ListWaitingRooms(ctx, 2, 2)

	if len(page1) != 2 {
		t.Errorf("page1: expected 2, got %d", len(page1))
	}
	if len(page2) != 2 {
		t.Errorf("page2: expected 2, got %d", len(page2))
	}
	if page1[0].ID == page2[0].ID {
		t.Error("pagination returned same room on different pages")
	}
}

func TestSoftDeleteRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "delroomowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "DEL00001", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})

	if err := s.SoftDeleteRoom(ctx, room.ID); err != nil {
		t.Fatalf("SoftDeleteRoom: %v", err)
	}

	_, err := s.GetRoom(ctx, room.ID)
	if err == nil {
		t.Error("expected error getting soft-deleted room")
	}
}

func TestUpdateRoomOwner(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "oldowner")
	newOwner, _ := s.CreatePlayer(ctx, "newowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "OWN00001", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})

	if err := s.UpdateRoomOwner(ctx, room.ID, newOwner.ID); err != nil {
		t.Fatalf("UpdateRoomOwner: %v", err)
	}

	fetched, _ := s.GetRoom(ctx, room.ID)
	if fetched.OwnerID != newOwner.ID {
		t.Errorf("expected new owner %s, got %s", newOwner.ID, fetched.OwnerID)
	}
}

func TestDeleteRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "harddelowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "HDEL0001", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})
	s.AddPlayerToRoom(ctx, room.ID, owner.ID, 0)

	if err := s.DeleteRoom(ctx, room.ID); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}

	// Room should be finished status.
	fetched, err := s.GetRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoom after DeleteRoom: %v", err)
	}
	if fetched.Status != store.RoomStatusFinished {
		t.Errorf("expected finished, got %s", fetched.Status)
	}

	// Players should be removed.
	players, _ := s.ListRoomPlayers(ctx, room.ID)
	if len(players) != 0 {
		t.Errorf("expected 0 players after DeleteRoom, got %d", len(players))
	}
}

// --- Room settings -----------------------------------------------------------

func TestRoomSettings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "setowner")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: "SET00001", GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
		DefaultSettings: map[string]string{"timeout": "60", "mode": "classic"},
	})

	settings, err := s.GetRoomSettings(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetRoomSettings: %v", err)
	}
	if settings["timeout"] != "60" {
		t.Errorf("expected timeout=60, got %s", settings["timeout"])
	}
	if settings["mode"] != "classic" {
		t.Errorf("expected mode=classic, got %s", settings["mode"])
	}

	// Upsert existing setting.
	if err := s.SetRoomSetting(ctx, room.ID, "timeout", "90"); err != nil {
		t.Fatalf("SetRoomSetting: %v", err)
	}

	settings, _ = s.GetRoomSettings(ctx, room.ID)
	if settings["timeout"] != "90" {
		t.Errorf("expected timeout=90 after upsert, got %s", settings["timeout"])
	}

	// Add new setting.
	if err := s.SetRoomSetting(ctx, room.ID, "spectators", "true"); err != nil {
		t.Fatalf("SetRoomSetting new: %v", err)
	}
	settings, _ = s.GetRoomSettings(ctx, room.ID)
	if settings["spectators"] != "true" {
		t.Errorf("expected spectators=true, got %s", settings["spectators"])
	}
}

// --- Room players ------------------------------------------------------------

func TestRemovePlayerFromRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	_ = owner

	if err := s.RemovePlayerFromRoom(ctx, room.ID, guest.ID); err != nil {
		t.Fatalf("RemovePlayerFromRoom: %v", err)
	}

	players, _ := s.ListRoomPlayers(ctx, room.ID)
	if len(players) != 1 {
		t.Errorf("expected 1 player after removal, got %d", len(players))
	}
}

// --- Session lifecycle -------------------------------------------------------

func TestFinishSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	if err := s.FinishSession(ctx, session.ID); err != nil {
		t.Fatalf("FinishSession: %v", err)
	}

	fetched, _ := s.GetGameSession(ctx, session.ID)
	if fetched.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}

func TestGetActiveSessionByRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	active, err := s.GetActiveSessionByRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetActiveSessionByRoom: %v", err)
	}
	if active.ID != session.ID {
		t.Errorf("expected %s, got %s", session.ID, active.ID)
	}

	// After finishing, no active session.
	s.FinishSession(ctx, session.ID)
	_, err = s.GetActiveSessionByRoom(ctx, room.ID)
	if err == nil {
		t.Error("expected error for no active session")
	}
}

func TestSuspendAndResumeSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	if err := s.SuspendSession(ctx, session.ID, "player disconnect"); err != nil {
		t.Fatalf("SuspendSession: %v", err)
	}

	fetched, _ := s.GetGameSession(ctx, session.ID)
	if fetched.SuspendedAt == nil {
		t.Error("expected SuspendedAt to be set")
	}
	if fetched.SuspendCount != 1 {
		t.Errorf("expected suspend_count 1, got %d", fetched.SuspendCount)
	}

	if err := s.ResumeSession(ctx, session.ID); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}

	fetched, _ = s.GetGameSession(ctx, session.ID)
	if fetched.SuspendedAt != nil {
		t.Error("expected SuspendedAt to be cleared after resume")
	}
	if fetched.SuspendedReason != nil {
		t.Error("expected SuspendedReason to be cleared after resume")
	}
}

func TestSoftDeleteSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	if err := s.SoftDeleteSession(ctx, session.ID); err != nil {
		t.Fatalf("SoftDeleteSession: %v", err)
	}

	_, err := s.GetGameSession(ctx, session.ID)
	if err == nil {
		t.Error("expected error getting soft-deleted session")
	}
}

func TestListActiveSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _, room := setupRoom(t, s)
	createSession(t, s, room.ID)

	sessions, err := s.ListActiveSessions(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListActiveSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 active session, got %d", len(sessions))
	}
}

func TestTouchLastMoveAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	before, _ := s.GetGameSession(ctx, session.ID)
	time.Sleep(10 * time.Millisecond)

	if err := s.TouchLastMoveAt(ctx, session.ID); err != nil {
		t.Fatalf("TouchLastMoveAt: %v", err)
	}

	after, _ := s.GetGameSession(ctx, session.ID)
	if !after.LastMoveAt.After(before.LastMoveAt) {
		t.Error("expected LastMoveAt to advance")
	}
}

func TestCountFinishedSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)

	count, _ := s.CountFinishedSessions(ctx, room.ID)
	if count != 0 {
		t.Errorf("expected 0 finished, got %d", count)
	}

	sess := createSession(t, s, room.ID)
	s.FinishSession(ctx, sess.ID)

	count, _ = s.CountFinishedSessions(ctx, room.ID)
	if count != 1 {
		t.Errorf("expected 1 finished, got %d", count)
	}
}

func TestGetLastFinishedSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)

	sess1 := createSession(t, s, room.ID)
	s.FinishSession(ctx, sess1.ID)

	sess2 := createSession(t, s, room.ID)
	s.FinishSession(ctx, sess2.ID)

	last, err := s.GetLastFinishedSession(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetLastFinishedSession: %v", err)
	}
	if last.ID != sess2.ID {
		t.Errorf("expected last finished to be %s, got %s", sess2.ID, last.ID)
	}
}

func TestGetGameConfig_Default(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	cfg, err := s.GetGameConfig(ctx, "nonexistent-game")
	if err != nil {
		t.Fatalf("GetGameConfig: %v", err)
	}
	if cfg.GameID != "nonexistent-game" {
		t.Errorf("expected game_id nonexistent-game, got %s", cfg.GameID)
	}
	if cfg.DefaultTimeoutSecs != 60 {
		t.Errorf("expected default timeout 60, got %d", cfg.DefaultTimeoutSecs)
	}
}

// --- Timer -------------------------------------------------------------------

func TestListSessionsNeedingTimer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)

	// Session without timeout — should NOT appear.
	createSession(t, s, room.ID)

	// Session with timeout — should appear.
	timerSession := createSessionWithTimeout(t, s, room.ID, 30)

	sessions, err := s.ListSessionsNeedingTimer(ctx)
	if err != nil {
		t.Fatalf("ListSessionsNeedingTimer: %v", err)
	}

	found := false
	for _, gs := range sessions {
		if gs.ID == timerSession.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected session with timeout to appear")
	}
}

func TestListSessionsNeedingTimer_ExcludesSuspended(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	sess := createSessionWithTimeout(t, s, room.ID, 30)
	s.SuspendSession(ctx, sess.ID, "test")

	sessions, _ := s.ListSessionsNeedingTimer(ctx)
	for _, gs := range sessions {
		if gs.ID == sess.ID {
			t.Error("suspended session should not need timer")
		}
	}
}

// --- Results -----------------------------------------------------------------

func TestCreateGameResultAndStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	result, err := s.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID,
		GameID:    "tictactoe",
		WinnerID:  &owner.ID,
		IsDraw:    false,
		EndedBy:   store.EndedByNormal,
		Players: []store.GameResultPlayer{
			{ResultID: uuid.Nil, PlayerID: owner.ID, Seat: 0, Outcome: store.OutcomeWin},
			{ResultID: uuid.Nil, PlayerID: guest.ID, Seat: 1, Outcome: store.OutcomeLoss},
		},
	})
	if err != nil {
		t.Fatalf("CreateGameResult: %v", err)
	}
	if result.EndedBy != store.EndedByNormal {
		t.Errorf("expected ended_by win, got %s", result.EndedBy)
	}
	if result.DurationSecs == nil {
		t.Error("expected duration_secs to be set")
	}

	// GetGameResult.
	fetched, err := s.GetGameResult(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetGameResult: %v", err)
	}
	if fetched.ID != result.ID {
		t.Errorf("expected result %s, got %s", result.ID, fetched.ID)
	}

	// PlayerStats.
	stats, err := s.GetPlayerStats(ctx, owner.ID)
	if err != nil {
		t.Fatalf("GetPlayerStats: %v", err)
	}
	if stats.TotalGames != 1 || stats.Wins != 1 {
		t.Errorf("expected 1 game 1 win, got %d/%d", stats.TotalGames, stats.Wins)
	}

	guestStats, _ := s.GetPlayerStats(ctx, guest.ID)
	if guestStats.Losses != 1 {
		t.Errorf("expected 1 loss for guest, got %d", guestStats.Losses)
	}
}

func TestListPlayerHistory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	s.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID, GameID: "tictactoe",
		WinnerID: &owner.ID, EndedBy: store.EndedByNormal,
		Players: []store.GameResultPlayer{
			{PlayerID: owner.ID, Seat: 0, Outcome: store.OutcomeWin},
			{PlayerID: guest.ID, Seat: 1, Outcome: store.OutcomeLoss},
		},
	})

	history, err := s.ListPlayerHistory(ctx, owner.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListPlayerHistory: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 result, got %d", len(history))
	}
}

func TestListPlayerMatchesAndCount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	s.CreateGameResult(ctx, store.CreateGameResultParams{
		SessionID: session.ID, GameID: "tictactoe",
		WinnerID: &owner.ID, EndedBy: store.EndedByNormal,
		Players: []store.GameResultPlayer{
			{PlayerID: owner.ID, Seat: 0, Outcome: store.OutcomeWin},
			{PlayerID: guest.ID, Seat: 1, Outcome: store.OutcomeLoss},
		},
	})

	matches, err := s.ListPlayerMatches(ctx, owner.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListPlayerMatches: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Outcome != store.OutcomeWin {
		t.Errorf("expected outcome win, got %s", matches[0].Outcome)
	}
	if matches[0].OpponentID == nil || *matches[0].OpponentID != guest.ID {
		t.Errorf("expected opponent_id=%s, got %+v", guest.ID, matches[0].OpponentID)
	}
	if matches[0].OpponentUsername == nil || *matches[0].OpponentUsername != guest.Username {
		t.Errorf("expected opponent_username=%s, got %+v", guest.Username, matches[0].OpponentUsername)
	}
	if matches[0].OpponentIsBot == nil || *matches[0].OpponentIsBot {
		t.Errorf("expected opponent_is_bot=false, got %+v", matches[0].OpponentIsBot)
	}

	count, err := s.CountPlayerMatches(ctx, owner.ID)
	if err != nil {
		t.Fatalf("CountPlayerMatches: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 match, got %d", count)
	}
}

// --- Rematch votes -----------------------------------------------------------

func TestRematchVotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	if err := s.UpsertRematchVote(ctx, session.ID, owner.ID); err != nil {
		t.Fatalf("UpsertRematchVote: %v", err)
	}
	// Idempotent.
	if err := s.UpsertRematchVote(ctx, session.ID, owner.ID); err != nil {
		t.Fatalf("UpsertRematchVote idempotent: %v", err)
	}

	if err := s.UpsertRematchVote(ctx, session.ID, guest.ID); err != nil {
		t.Fatalf("UpsertRematchVote guest: %v", err)
	}

	votes, err := s.ListRematchVotes(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListRematchVotes: %v", err)
	}
	if len(votes) != 2 {
		t.Errorf("expected 2 votes, got %d", len(votes))
	}

	if err := s.DeleteRematchVotes(ctx, session.ID); err != nil {
		t.Fatalf("DeleteRematchVotes: %v", err)
	}
	votes, _ = s.ListRematchVotes(ctx, session.ID)
	if len(votes) != 0 {
		t.Errorf("expected 0 votes after delete, got %d", len(votes))
	}
}

// --- Pause/Resume votes ------------------------------------------------------

func TestPauseVotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	// First vote — not all voted yet.
	allVoted, err := s.VotePause(ctx, session.ID, owner.ID)
	if err != nil {
		t.Fatalf("VotePause: %v", err)
	}
	if allVoted {
		t.Error("expected allVoted=false with 1/2 votes")
	}

	count, _ := s.CountPauseVotes(ctx, session.ID)
	if count != 1 {
		t.Errorf("expected 1 pause vote, got %d", count)
	}

	// Second vote — all voted.
	allVoted, err = s.VotePause(ctx, session.ID, guest.ID)
	if err != nil {
		t.Fatalf("VotePause guest: %v", err)
	}
	if !allVoted {
		t.Error("expected allVoted=true with 2/2 votes")
	}

	// Clear.
	if err := s.ClearPauseVotes(ctx, session.ID); err != nil {
		t.Fatalf("ClearPauseVotes: %v", err)
	}
	count, _ = s.CountPauseVotes(ctx, session.ID)
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestPauseVoteIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	s.VotePause(ctx, session.ID, owner.ID)
	s.VotePause(ctx, session.ID, owner.ID) // idempotent

	count, _ := s.CountPauseVotes(ctx, session.ID)
	if count != 1 {
		t.Errorf("expected 1 vote after duplicate, got %d", count)
	}
}

func TestResumeVotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	allVoted, _ := s.VoteResume(ctx, session.ID, owner.ID)
	if allVoted {
		t.Error("expected allVoted=false")
	}

	count, _ := s.CountResumeVotes(ctx, session.ID)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	allVoted, _ = s.VoteResume(ctx, session.ID, guest.ID)
	if !allVoted {
		t.Error("expected allVoted=true")
	}

	s.ClearResumeVotes(ctx, session.ID)
	count, _ = s.CountResumeVotes(ctx, session.ID)
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestPauseVotesExcludeBots(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	human, _ := s.CreatePlayer(ctx, "human-"+uuid.NewString()[:8])
	bot, _ := s.CreateBotPlayer(ctx, "bot-"+uuid.NewString()[:8])

	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "tictactoe", OwnerID: human.ID, MaxPlayers: 2,
	})
	s.AddPlayerToRoom(ctx, room.ID, human.ID, 0)
	s.AddPlayerToRoom(ctx, room.ID, bot.ID, 1)

	session := createSession(t, s, room.ID)

	// Only 1 human — single vote should trigger allVoted.
	allVoted, err := s.VotePause(ctx, session.ID, human.ID)
	if err != nil {
		t.Fatalf("VotePause: %v", err)
	}
	if !allVoted {
		t.Error("expected allVoted=true with 1 human voting (bot excluded)")
	}
}

func TestForceCloseSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)
	s.SuspendSession(ctx, session.ID, "test")

	if err := s.ForceCloseSession(ctx, session.ID); err != nil {
		t.Fatalf("ForceCloseSession: %v", err)
	}

	fetched, _ := s.GetGameSession(ctx, session.ID)
	if fetched.FinishedAt == nil {
		t.Error("expected FinishedAt set after force close")
	}
	if fetched.SuspendedAt != nil {
		t.Error("expected SuspendedAt cleared after force close")
	}
}

func TestForceCloseSession_AlreadyFinished(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)
	s.FinishSession(ctx, session.ID)

	err := s.ForceCloseSession(ctx, session.ID)
	if err == nil {
		t.Error("expected error force-closing already finished session")
	}
}

// --- Ready handshake ---------------------------------------------------------

func TestReadyVotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, guest, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	allReady, err := s.VoteReady(ctx, session.ID, owner.ID)
	if err != nil {
		t.Fatalf("VoteReady: %v", err)
	}
	if allReady {
		t.Error("expected allReady=false with 1/2")
	}

	allReady, err = s.VoteReady(ctx, session.ID, guest.ID)
	if err != nil {
		t.Fatalf("VoteReady guest: %v", err)
	}
	if !allReady {
		t.Error("expected allReady=true with 2/2")
	}

	// Verify ready_players array.
	fetched, _ := s.GetGameSession(ctx, session.ID)
	if len(fetched.ReadyPlayers) != 2 {
		t.Errorf("expected 2 ready players, got %d", len(fetched.ReadyPlayers))
	}
}

func TestReadyVoteIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	s.VoteReady(ctx, session.ID, owner.ID)
	s.VoteReady(ctx, session.ID, owner.ID)

	fetched, _ := s.GetGameSession(ctx, session.ID)
	if len(fetched.ReadyPlayers) != 1 {
		t.Errorf("expected 1 ready player after duplicate vote, got %d", len(fetched.ReadyPlayers))
	}
}

func TestClearReadyVotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)
	s.VoteReady(ctx, session.ID, owner.ID)

	if err := s.ClearReadyVotes(ctx, session.ID); err != nil {
		t.Fatalf("ClearReadyVotes: %v", err)
	}

	fetched, _ := s.GetGameSession(ctx, session.ID)
	if len(fetched.ReadyPlayers) != 0 {
		t.Errorf("expected 0 ready players after clear, got %d", len(fetched.ReadyPlayers))
	}
}

func TestReadyVotesExcludeBots(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	human, _ := s.CreatePlayer(ctx, "readyhuman-"+uuid.NewString()[:8])
	bot, _ := s.CreateBotPlayer(ctx, "readybot-"+uuid.NewString()[:8])

	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "tictactoe", OwnerID: human.ID, MaxPlayers: 2,
	})
	s.AddPlayerToRoom(ctx, room.ID, human.ID, 0)
	s.AddPlayerToRoom(ctx, room.ID, bot.ID, 1)

	session := createSession(t, s, room.ID)

	allReady, err := s.VoteReady(ctx, session.ID, human.ID)
	if err != nil {
		t.Fatalf("VoteReady: %v", err)
	}
	if !allReady {
		t.Error("expected allReady=true with 1 human (bot excluded)")
	}
}

// --- Session events ----------------------------------------------------------

func TestBulkCreateAndListSessionEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _, room := setupRoom(t, s)
	session := createSession(t, s, room.ID)

	now := time.Now().UTC()
	events := []store.CreateSessionEventParams{
		{SessionID: session.ID, StreamID: "evt-1", EventType: "move", PlayerID: &owner.ID, Payload: json.RawMessage(`{"x":0}`), OccurredAt: now},
		{SessionID: session.ID, StreamID: "evt-2", EventType: "move", PlayerID: &owner.ID, Payload: json.RawMessage(`{"x":1}`), OccurredAt: now.Add(time.Second)},
	}

	if err := s.BulkCreateSessionEvents(ctx, events); err != nil {
		t.Fatalf("BulkCreateSessionEvents: %v", err)
	}

	fetched, err := s.ListSessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListSessionEvents: %v", err)
	}
	if len(fetched) != 2 {
		t.Errorf("expected 2 events, got %d", len(fetched))
	}

	// Idempotent — re-insert should not create duplicates.
	s.BulkCreateSessionEvents(ctx, events)
	fetched, _ = s.ListSessionEvents(ctx, session.ID)
	if len(fetched) != 2 {
		t.Errorf("expected 2 events after idempotent re-insert, got %d", len(fetched))
	}
}

func TestBulkCreateSessionEvents_Empty(t *testing.T) {
	s := newTestStore(t)
	// Empty slice should be a no-op.
	if err := s.BulkCreateSessionEvents(context.Background(), nil); err != nil {
		t.Fatalf("BulkCreateSessionEvents empty: %v", err)
	}
}

// --- Admin stats -------------------------------------------------------------

func TestAdminStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	createSession(t, s, room.ID)

	rooms, err := s.CountActiveRooms(ctx)
	if err != nil {
		t.Fatalf("CountActiveRooms: %v", err)
	}
	if rooms < 1 {
		t.Errorf("expected at least 1 active room, got %d", rooms)
	}

	sessions, err := s.CountActiveSessions(ctx)
	if err != nil {
		t.Fatalf("CountActiveSessions: %v", err)
	}
	if sessions < 1 {
		t.Errorf("expected at least 1 active session, got %d", sessions)
	}

	players, err := s.CountTotalPlayers(ctx)
	if err != nil {
		t.Fatalf("CountTotalPlayers: %v", err)
	}
	if players < 2 {
		t.Errorf("expected at least 2 players, got %d", players)
	}

	today, err := s.CountSessionsToday(ctx)
	if err != nil {
		t.Fatalf("CountSessionsToday: %v", err)
	}
	if today < 1 {
		t.Errorf("expected at least 1 session today, got %d", today)
	}
}

// --- Cleanup -----------------------------------------------------------------

func TestCleanupOrphanRooms(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "cleanupowner")

	// Create a waiting room with no players (orphan).
	s.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "chess", OwnerID: owner.ID, MaxPlayers: 2,
	})

	// With a very short maxAge and freshly created rooms, nothing should be cleaned.
	cleaned, err := s.CleanupOrphanRooms(ctx, 1*time.Hour, 1*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOrphanRooms: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned (too recent), got %d", cleaned)
	}
}

// --- Ranked session ----------------------------------------------------------

func TestCreateRankedSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, room := setupRoom(t, s)
	state := []byte(`{"current_player_id":"` + uuid.New().String() + `","data":{}}`)
	session, err := s.CreateGameSession(ctx, room.ID, "tictactoe", state, nil, store.SessionModeRanked)
	if err != nil {
		t.Fatalf("CreateGameSession ranked: %v", err)
	}
	if session.Mode != store.SessionModeRanked {
		t.Errorf("expected mode ranked, got %s", session.Mode)
	}
}
