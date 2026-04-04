package lobby_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/platform/store"
)

// --- CreateRoom (additional) -------------------------------------------------

func TestCreateRoom_SettingsPopulated(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, err := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if len(view.Settings) == 0 {
		t.Error("expected default settings to be populated")
	}
	if view.Settings["room_visibility"] != "public" {
		t.Errorf("expected room_visibility=public, got %s", view.Settings["room_visibility"])
	}
	if view.Settings["first_mover_policy"] != "random" {
		t.Errorf("expected first_mover_policy=random, got %s", view.Settings["first_mover_policy"])
	}
}

func TestCreateRoom_WithTurnTimeout(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	timeout := 30

	view, err := svc.CreateRoom(context.Background(), "chess", owner.ID, &timeout)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if view.Room.GameID != "chess" {
		t.Errorf("expected game_id chess, got %s", view.Room.GameID)
	}
}

func TestCreateRoom_OwnerIsSeat0(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, err := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if len(view.Players) != 1 {
		t.Fatalf("expected 1 player, got %d", len(view.Players))
	}
	if view.Players[0].Seat != 0 {
		t.Errorf("expected owner at seat 0, got seat %d", view.Players[0].Seat)
	}
}

// --- JoinRoom (additional) ---------------------------------------------------

func TestJoinRoom_GuestGetsSeat1(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	joined, err := svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	for _, p := range joined.Players {
		if p.ID == guest.ID && p.Seat != 1 {
			t.Errorf("expected guest at seat 1, got seat %d", p.Seat)
		}
	}
}

func TestJoinRoom_CaseInsensitiveCode(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	// Join using uppercase — should work since codes are generated uppercase
	// and JoinRoom uppercases input.
	joined, err := svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if len(joined.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(joined.Players))
	}
}

func TestJoinRoom_RoomNotWaiting(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")
	third := createPlayer(t, s, "carol")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	// Start the game to transition the room out of waiting.
	svc.StartGame(context.Background(), view.Room.ID, owner.ID, store.SessionModeCasual)

	_, err := svc.JoinRoom(context.Background(), view.Room.Code, third.ID)
	if err != lobby.ErrRoomNotWaiting {
		t.Errorf("expected ErrRoomNotWaiting, got %v", err)
	}
}

// --- LeaveRoom (additional) --------------------------------------------------

func TestLeaveRoom_OwnerLeaves_TransfersOwnership(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	result, err := svc.LeaveRoom(context.Background(), view.Room.ID, owner.ID)
	if err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	if result.RoomClosed {
		t.Error("expected room to remain open")
	}
	if result.NewOwnerID == nil {
		t.Fatal("expected new owner to be assigned")
	}
	if *result.NewOwnerID != guest.ID {
		t.Errorf("expected new owner to be bob (%s), got %s", guest.ID, *result.NewOwnerID)
	}
}

func TestLeaveRoom_LastPlayer_ClosesRoom(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	result, err := svc.LeaveRoom(context.Background(), view.Room.ID, owner.ID)
	if err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	if !result.RoomClosed {
		t.Error("expected room to be closed when last player leaves")
	}
}

func TestLeaveRoom_NonOwner_NoOwnerChange(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	result, err := svc.LeaveRoom(context.Background(), view.Room.ID, guest.ID)
	if err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	if result.RoomClosed {
		t.Error("expected room to remain open")
	}
	if result.NewOwnerID != nil {
		t.Error("expected no owner change when non-owner leaves")
	}
}

func TestLeaveRoom_RoomNotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.LeaveRoom(context.Background(), uuid.New(), uuid.New())
	if err != lobby.ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestLeaveRoom_RoomNotWaiting(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	svc.StartGame(context.Background(), view.Room.ID, owner.ID, store.SessionModeCasual)

	_, err := svc.LeaveRoom(context.Background(), view.Room.ID, guest.ID)
	if err != lobby.ErrRoomNotWaiting {
		t.Errorf("expected ErrRoomNotWaiting, got %v", err)
	}
}

// --- GetRoom -----------------------------------------------------------------

func TestGetRoom_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	got, err := svc.GetRoom(context.Background(), view.Room.ID)
	if err != nil {
		t.Fatalf("GetRoom: %v", err)
	}

	if got.Room.ID != view.Room.ID {
		t.Errorf("expected room ID %s, got %s", view.Room.ID, got.Room.ID)
	}
}

func TestGetRoom_NotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.GetRoom(context.Background(), uuid.New())
	if err != lobby.ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

// --- ListWaitingRooms --------------------------------------------------------

func TestListWaitingRooms_Empty(t *testing.T) {
	svc, _ := newTestService()

	rooms, err := svc.ListWaitingRooms(context.Background())
	if err != nil {
		t.Fatalf("ListWaitingRooms: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms, got %d", len(rooms))
	}
}

func TestListWaitingRooms_OnlyWaiting(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	// Create two rooms.
	view1, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.CreateRoom(context.Background(), "chess", guest.ID, nil)

	// Start game in first room (transitions to in_progress).
	svc.JoinRoom(context.Background(), view1.Room.Code, guest.ID)
	svc.StartGame(context.Background(), view1.Room.ID, owner.ID, store.SessionModeCasual)

	rooms, err := svc.ListWaitingRooms(context.Background())
	if err != nil {
		t.Fatalf("ListWaitingRooms: %v", err)
	}

	if len(rooms) != 1 {
		t.Errorf("expected 1 waiting room, got %d", len(rooms))
	}
}

func TestListWaitingRooms_PrivateRoomCodeRedacted(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	// Set room to private.
	s.SetRoomSetting(context.Background(), view.Room.ID, "room_visibility", "private")

	rooms, err := svc.ListWaitingRooms(context.Background())
	if err != nil {
		t.Fatalf("ListWaitingRooms: %v", err)
	}

	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}
	if rooms[0].Room.Code != "" {
		t.Error("expected code to be redacted for private rooms")
	}
}

// --- UpdateRoomSetting -------------------------------------------------------

func TestUpdateRoomSetting_Success(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "room_visibility", "private")
	if err != nil {
		t.Fatalf("UpdateRoomSetting: %v", err)
	}

	settings, _ := s.GetRoomSettings(context.Background(), view.Room.ID)
	if settings["room_visibility"] != "private" {
		t.Errorf("expected private, got %s", settings["room_visibility"])
	}
}

func TestUpdateRoomSetting_NotOwner(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, guest.ID, "room_visibility", "private")
	if err != lobby.ErrNotOwner {
		t.Errorf("expected ErrNotOwner, got %v", err)
	}
}

func TestUpdateRoomSetting_UnknownKey(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "unknown_key", "value")
	if err == nil {
		t.Fatal("expected error for unknown setting key")
	}
}

func TestUpdateRoomSetting_InvalidValue(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "room_visibility", "bad_value")
	if err == nil {
		t.Fatal("expected error for invalid setting value")
	}
}

func TestUpdateRoomSetting_RoomNotFound(t *testing.T) {
	svc, _ := newTestService()

	err := svc.UpdateRoomSetting(context.Background(), uuid.New(), uuid.New(), "room_visibility", "private")
	if err != lobby.ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestUpdateRoomSetting_RoomNotWaiting(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)
	svc.StartGame(context.Background(), view.Room.ID, owner.ID, store.SessionModeCasual)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "room_visibility", "private")
	if err != lobby.ErrRoomNotWaiting {
		t.Errorf("expected ErrRoomNotWaiting, got %v", err)
	}
}

func TestUpdateRoomSetting_IntSetting_Valid(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "first_mover_seat", "1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpdateRoomSetting_IntSetting_OutOfRange(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	// Chess maxPlayers is 2, so seat 2 is out of range (max is 1).
	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "first_mover_seat", "2")
	if err == nil {
		t.Fatal("expected error for out of range seat")
	}
}

func TestUpdateRoomSetting_IntSetting_NotAnInt(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)

	err := svc.UpdateRoomSetting(context.Background(), view.Room.ID, owner.ID, "first_mover_seat", "not_a_number")
	if err == nil {
		t.Fatal("expected error for non-integer value")
	}
}

// --- StartGame (additional) --------------------------------------------------

func TestStartGame_RoomNotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.StartGame(context.Background(), uuid.New(), uuid.New(), store.SessionModeCasual)
	if err != lobby.ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestStartGame_SessionHasCorrectMode(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	session, err := svc.StartGame(context.Background(), view.Room.ID, owner.ID, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if session.Mode != store.SessionModeCasual {
		t.Errorf("expected mode casual, got %s", session.Mode)
	}
}

func TestStartGame_SessionHasState(t *testing.T) {
	svc, s := newTestService()
	owner := createPlayer(t, s, "alice")
	guest := createPlayer(t, s, "bob")

	view, _ := svc.CreateRoom(context.Background(), "chess", owner.ID, nil)
	svc.JoinRoom(context.Background(), view.Room.Code, guest.ID)

	session, err := svc.StartGame(context.Background(), view.Room.ID, owner.ID, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if len(session.State) == 0 {
		t.Error("expected non-empty initial state")
	}
}
