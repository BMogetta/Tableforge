package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/shared/testutil"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	dsn := testutil.NewTestDB(t,
		testutil.MigrationInitial,
		testutil.MigrationUserService,
		testutil.MigrationRating,
		testutil.MigrationBotProfile,
	)
	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestCreateAndGetPlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, err := s.CreatePlayer(ctx, "alice")
	if err != nil {
		t.Fatalf("CreatePlayer: %v", err)
	}

	if player.Username != "alice" {
		t.Errorf("expected username alice, got %s", player.Username)
	}

	fetched, err := s.GetPlayer(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetPlayer: %v", err)
	}

	if fetched.ID != player.ID {
		t.Errorf("expected id %s, got %s", player.ID, fetched.ID)
	}
}

func TestCreateRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, err := s.CreatePlayer(ctx, "bob")
	if err != nil {
		t.Fatalf("CreatePlayer: %v", err)
	}

	room, err := s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "ABCD1234",
		GameID:     "chess",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if room.Code != "ABCD1234" {
		t.Errorf("expected code ABCD1234, got %s", room.Code)
	}
	if room.Status != store.RoomStatusWaiting {
		t.Errorf("expected status waiting, got %s", room.Status)
	}
}

func TestRoomPlayerFlow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "carol")
	guest, _ := s.CreatePlayer(ctx, "dave")

	room, err := s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "ROOM0001",
		GameID:     "checkers",
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

	players, err := s.ListRoomPlayers(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListRoomPlayers: %v", err)
	}
	if len(players) != 2 {
		t.Errorf("expected 2 players, got %d", len(players))
	}
}

func TestGameSessionFlow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "eve")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "SESS0001",
		GameID:     "chess",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})

	initialState := []byte(`{"current_player_id":"` + uuid.New().String() + `","data":{}}`)

	session, err := s.CreateGameSession(ctx, room.ID, "chess", initialState, nil, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("CreateGameSession: %v", err)
	}

	newState := []byte(`{"current_player_id":"` + uuid.New().String() + `","data":{"turn":1}}`)
	if err := s.UpdateSessionState(ctx, session.ID, newState); err != nil {
		t.Fatalf("UpdateSessionState: %v", err)
	}

	fetched, err := s.GetGameSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetGameSession: %v", err)
	}
	if fetched.MoveCount != 1 {
		t.Errorf("expected move_count 1, got %d", fetched.MoveCount)
	}
}

func TestMoveStateAfterRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "frank")
	room, _ := s.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "MOVE0001",
		GameID:     "tictactoe",
		OwnerID:    owner.ID,
		MaxPlayers: 2,
	})

	initialState := []byte(`{"current_player_id":"` + owner.ID.String() + `","data":{}}`)
	session, err := s.CreateGameSession(ctx, room.ID, "tictactoe", initialState, nil, store.SessionModeCasual)
	if err != nil {
		t.Fatalf("CreateGameSession: %v", err)
	}

	stateAfter := []byte(`{"current_player_id":"` + owner.ID.String() + `","data":{"board":["X","","","","","","","",""]}}`)
	move, err := s.RecordMove(ctx, store.RecordMoveParams{
		SessionID:  session.ID,
		PlayerID:   owner.ID,
		Payload:    []byte(`{"x":0,"y":0}`),
		StateAfter: stateAfter,
		MoveNumber: 1,
	})
	if err != nil {
		t.Fatalf("RecordMove: %v", err)
	}
	if string(move.StateAfter) != string(stateAfter) {
		t.Errorf("RecordMove: state_after mismatch")
	}

	// Verify ListSessionMoves returns state_after.
	moves, err := s.ListSessionMoves(ctx, session.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListSessionMoves: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if string(moves[0].StateAfter) != string(stateAfter) {
		t.Errorf("ListSessionMoves: state_after mismatch")
	}

	// Verify GetMoveAt returns state_after.
	fetched, err := s.GetMoveAt(ctx, session.ID, 1)
	if err != nil {
		t.Fatalf("GetMoveAt: %v", err)
	}
	if string(fetched.StateAfter) != string(stateAfter) {
		t.Errorf("GetMoveAt: state_after mismatch")
	}
}
