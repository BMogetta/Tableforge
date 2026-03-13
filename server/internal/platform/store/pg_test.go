package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newTestStore spins up a real Postgres container and runs migrations.
// It registers a cleanup function to terminate the container after the test.
func newTestStore(t *testing.T) store.Store {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("tableforge_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	s, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	for _, path := range []string{
		"../../../db/migrations/001_initial.sql",
		"../../../db/migrations/002_session_events.sql",
		"../../../db/migrations/003_chat_ratings_moderation.sql",
	} {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", path, err)
		}
		if err := s.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("failed to run migration %s: %v", path, err)
		}
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
