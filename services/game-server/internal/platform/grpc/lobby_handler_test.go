package grpchandler

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/testutil"
	lobbyv1 "github.com/recess/shared/proto/lobby/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- stub game + registry (minimal for lobby.Service) ------------------------

type stubGame struct{ id string }

func (g *stubGame) ID() string      { return g.id }
func (g *stubGame) Name() string    { return g.id }
func (g *stubGame) MinPlayers() int { return 2 }
func (g *stubGame) MaxPlayers() int { return 2 }
func (g *stubGame) Init(players []engine.Player) (engine.GameState, error) {
	return engine.GameState{CurrentPlayerID: players[0].ID, Data: map[string]any{}}, nil
}
func (g *stubGame) ValidateMove(engine.GameState, engine.Move) error              { return nil }
func (g *stubGame) ApplyMove(s engine.GameState, _ engine.Move) (engine.GameState, error) { return s, nil }
func (g *stubGame) IsOver(engine.GameState) (bool, engine.Result)                 { return false, engine.Result{} }

type stubRegistry struct{ games map[string]engine.Game }

func (r *stubRegistry) Get(id string) (engine.Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, errors.New("game not found")
	}
	return g, nil
}

func newLobbyTestHandler() (*LobbyHandler, *testutil.FakeStore) {
	st := testutil.NewFakeStore()
	reg := &stubRegistry{games: map[string]engine.Game{"tictactoe": &stubGame{id: "tictactoe"}}}
	lobbySvc := lobby.New(st, reg)
	return NewLobbyHandler(lobbySvc, st), st
}

// --- CreateRankedRoom --------------------------------------------------------

func TestCreateRankedRoom(t *testing.T) {
	h, st := newLobbyTestHandler()
	ctx := context.Background()

	pA, _ := st.CreatePlayer(ctx, "alice")
	pB, _ := st.CreatePlayer(ctx, "bob")

	resp, err := h.CreateRankedRoom(ctx, &lobbyv1.CreateRankedRoomRequest{
		PlayerAId: pA.ID.String(),
		PlayerBId: pB.ID.String(),
		GameId:    "tictactoe",
	})
	if err != nil {
		t.Fatalf("CreateRankedRoom: %v", err)
	}
	if resp.RoomId == "" {
		t.Error("expected non-empty room_id")
	}
	if resp.RoomCode == "" {
		t.Error("expected non-empty room_code")
	}

	// Verify player B was added to the room.
	roomID, _ := uuid.Parse(resp.RoomId)
	players, _ := st.ListRoomPlayers(ctx, roomID)
	if len(players) != 2 {
		t.Fatalf("expected 2 players in room, got %d", len(players))
	}

	// Verify session mode was set to ranked.
	settings, _ := st.GetRoomSettings(ctx, roomID)
	if settings["session_mode"] != string(store.SessionModeRanked) {
		t.Errorf("expected session_mode ranked, got %s", settings["session_mode"])
	}
}

func TestCreateRankedRoom_MissingFields(t *testing.T) {
	h, _ := newLobbyTestHandler()

	cases := []struct {
		name string
		req  *lobbyv1.CreateRankedRoomRequest
	}{
		{"missing all", &lobbyv1.CreateRankedRoomRequest{}},
		{"missing player_b", &lobbyv1.CreateRankedRoomRequest{PlayerAId: uuid.NewString(), GameId: "tictactoe"}},
		{"missing game_id", &lobbyv1.CreateRankedRoomRequest{PlayerAId: uuid.NewString(), PlayerBId: uuid.NewString()}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.CreateRankedRoom(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %s", status.Code(err))
			}
		})
	}
}

func TestCreateRankedRoom_InvalidPlayerAID(t *testing.T) {
	h, _ := newLobbyTestHandler()

	_, err := h.CreateRankedRoom(context.Background(), &lobbyv1.CreateRankedRoomRequest{
		PlayerAId: "bad",
		PlayerBId: uuid.NewString(),
		GameId:    "tictactoe",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestCreateRankedRoom_InvalidPlayerBID(t *testing.T) {
	h, _ := newLobbyTestHandler()

	_, err := h.CreateRankedRoom(context.Background(), &lobbyv1.CreateRankedRoomRequest{
		PlayerAId: uuid.NewString(),
		PlayerBId: "bad",
		GameId:    "tictactoe",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}
