package lobby_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/platform/store"
)

func createPlayer(t *testing.T, s *fakeStore, username string) store.Player {
	t.Helper()
	p, err := s.CreatePlayer(context.Background(), username)
	if err != nil {
		t.Fatalf("createPlayer %s: %v", username, err)
	}
	return p
}

// --- CreateRoom --------------------------------------------------------------

func TestCreateRoom_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, err := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if view.Room.GameID != "chess" {
		t.Errorf("expected game_id chess, got %s", view.Room.GameID)
	}
	if view.Room.Status != store.RoomStatusWaiting {
		t.Errorf("expected status waiting, got %s", view.Room.Status)
	}
	if len(view.Players) != 1 {
		t.Errorf("expected 1 player in room, got %d", len(view.Players))
	}
	if view.Players[0].ID != owner.ID {
		t.Errorf("expected owner in seat 0")
	}
}

func TestCreateRoom_UnknownGame(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	_, err := svc.CreateRoom(context.Background(), "unknown_game", owner.ID, nil)
	if err != lobby.ErrGameNotFound {
		t.Errorf("expected ErrGameNotFound, got %v", err)
	}
}

// --- JoinRoom ----------------------------------------------------------------

func TestJoinRoom_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	joined, err := svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if len(joined.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(joined.Players))
	}
}

func TestJoinRoom_NotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.JoinRoom(context.Background(), "ZZZZZZ", uuid.New())
	if err != lobby.ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestJoinRoom_Full(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")
	extra := createPlayer(t, s, "carol")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	_, err := svc.JoinRoom(context.Background(), view.Room.Code, extra.ID)
	if err != lobby.ErrRoomFull {
		t.Errorf("expected ErrRoomFull, got %v", err)
	}
}

func TestJoinRoom_AlreadyInRoom(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	_, err := svc.JoinRoom(context.Background(), view.Room.Code, owner.ID)
	if err != lobby.ErrAlreadyInRoom {
		t.Errorf("expected ErrAlreadyInRoom, got %v", err)
	}
}

// --- LeaveRoom ---------------------------------------------------------------

func TestLeaveRoom_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	if _, err := svc.LeaveRoom(context.Background(), view.Room.ID, guest.ID); err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	updated, _ := svc.GetRoom(context.Background(), view.Room.ID)
	if len(updated.Players) != 1 {
		t.Errorf("expected 1 player after leave, got %d", len(updated.Players))
	}
}

// --- StartGame ---------------------------------------------------------------

func TestStartGame_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	session, err := svc.StartGame(context.Background(), view.Room.ID, owner.ID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	if session.GameID != "chess" {
		t.Errorf("expected game_id chess, got %s", session.GameID)
	}

	updated, _ := svc.GetRoom(context.Background(), view.Room.ID)
	if updated.Room.Status != store.RoomStatusInProgress {
		t.Errorf("expected status in_progress, got %s", updated.Room.Status)
	}
}

func TestStartGame_NotOwner(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	_, err := svc.StartGame(context.Background(), view.Room.ID, guest.ID)
	if err != lobby.ErrNotOwner {
		t.Errorf("expected ErrNotOwner, got %v", err)
	}
}

func TestStartGame_NotEnoughPlayers(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	_, err := svc.StartGame(context.Background(), view.Room.ID, owner.ID)
	if err != lobby.ErrNotEnoughPlayer {
		t.Errorf("expected ErrNotEnoughPlayer, got %v", err)
	}
}

func TestStartGame_AlreadyStarted(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	svc.StartGame(context.Background(), view.Room.ID, owner.ID)

	_, err := svc.StartGame(context.Background(), view.Room.ID, owner.ID)
	if err != lobby.ErrRoomNotWaiting {
		t.Errorf("expected ErrRoomNotWaiting, got %v", err)
	}
}
