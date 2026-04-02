package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/notification-service/internal/store"
	"github.com/recess/shared/testutil"
)

func newTestStore(t *testing.T) (*store.Store, uuid.UUID) {
	t.Helper()
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Insert a player to satisfy the FK constraint on notifications.
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	playerID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO players (id, username) VALUES ($1, $2)`,
		playerID, "testplayer",
	)
	if err != nil {
		t.Fatalf("failed to insert player: %v", err)
	}

	return s, playerID
}

func createNotification(t *testing.T, s *store.Store, playerID uuid.UUID, ntype store.NotificationType) store.Notification {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"from_player_id": uuid.New().String(), "from_username": "someone"})
	n, err := s.Create(context.Background(), store.CreateParams{
		PlayerID: playerID,
		Type:     ntype,
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return n
}

func TestCreateAndGet(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	payload, _ := json.Marshal(store.PayloadFriendRequest{
		FromPlayerID: uuid.New().String(),
		FromUsername:  "alice",
	})

	created, err := s.Create(ctx, store.CreateParams{
		PlayerID: playerID,
		Type:     store.NotificationTypeFriendRequest,
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.PlayerID != playerID {
		t.Errorf("expected player_id %s, got %s", playerID, created.PlayerID)
	}
	if created.Type != store.NotificationTypeFriendRequest {
		t.Errorf("expected type friend_request, got %s", created.Type)
	}
	if created.ReadAt != nil {
		t.Error("expected read_at to be nil")
	}

	fetched, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected id %s, got %s", created.ID, fetched.ID)
	}
}

func TestListUnreadOnly(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	n1 := createNotification(t, s, playerID, store.NotificationTypeFriendRequest)
	createNotification(t, s, playerID, store.NotificationTypeRoomInvitation)

	// Mark the first one as read.
	if err := s.MarkRead(ctx, n1.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	// List unread only.
	unread, err := s.List(ctx, playerID, false, time.Time{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread, got %d", len(unread))
	}
	if unread[0].Type != store.NotificationTypeRoomInvitation {
		t.Errorf("expected room_invitation, got %s", unread[0].Type)
	}
}

func TestListIncludeRead(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	n1 := createNotification(t, s, playerID, store.NotificationTypeFriendRequest)
	createNotification(t, s, playerID, store.NotificationTypeBanIssued)

	if err := s.MarkRead(ctx, n1.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	// Include read with a cutoff far in the past — should include the read one.
	all, err := s.List(ctx, playerID, true, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(all))
	}
}

func TestMarkReadIdempotent(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	n := createNotification(t, s, playerID, store.NotificationTypeFriendRequest)

	if err := s.MarkRead(ctx, n.ID); err != nil {
		t.Fatalf("MarkRead first call: %v", err)
	}

	// Second call should not error (WHERE read_at IS NULL prevents update).
	if err := s.MarkRead(ctx, n.ID); err != nil {
		t.Fatalf("MarkRead second call: %v", err)
	}

	fetched, _ := s.Get(ctx, n.ID)
	if fetched.ReadAt == nil {
		t.Error("expected read_at to be set")
	}
}

func TestSetAction(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	n := createNotification(t, s, playerID, store.NotificationTypeFriendRequest)

	if err := s.SetAction(ctx, n.ID, "accepted"); err != nil {
		t.Fatalf("SetAction: %v", err)
	}

	fetched, _ := s.Get(ctx, n.ID)
	if fetched.ActionTaken == nil || *fetched.ActionTaken != "accepted" {
		t.Errorf("expected action_taken=accepted, got %v", fetched.ActionTaken)
	}

	// Second call should be a no-op (WHERE action_taken IS NULL).
	if err := s.SetAction(ctx, n.ID, "declined"); err != nil {
		t.Fatalf("SetAction second call: %v", err)
	}

	fetched, _ = s.Get(ctx, n.ID)
	if *fetched.ActionTaken != "accepted" {
		t.Errorf("expected action_taken to remain accepted, got %s", *fetched.ActionTaken)
	}
}

func TestCreateWithActionExpiry(t *testing.T) {
	s, playerID := newTestStore(t)
	ctx := context.Background()

	expiry := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	payload, _ := json.Marshal(store.PayloadRoomInvitation{
		FromPlayerID: uuid.New().String(),
		FromUsername:  "bob",
		RoomID:       uuid.New().String(),
		RoomCode:     "ABCD",
		GameID:       "tictactoe",
		GameName:     "Tic Tac Toe",
	})

	n, err := s.Create(ctx, store.CreateParams{
		PlayerID:        playerID,
		Type:            store.NotificationTypeRoomInvitation,
		Payload:         payload,
		ActionExpiresAt: &expiry,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if n.ActionExpiresAt == nil {
		t.Fatal("expected action_expires_at to be set")
	}
	if !n.ActionExpiresAt.Truncate(time.Microsecond).Equal(expiry) {
		t.Errorf("expected %v, got %v", expiry, *n.ActionExpiresAt)
	}
}

func TestListIsolationBetweenPlayers(t *testing.T) {
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	pool, _ := pgxpool.New(ctx, dsn)
	defer pool.Close()

	player1, player2 := uuid.New(), uuid.New()
	pool.Exec(ctx, `INSERT INTO players (id, username) VALUES ($1, $2)`, player1, "p1")
	pool.Exec(ctx, `INSERT INTO players (id, username) VALUES ($1, $2)`, player2, "p2")

	createNotification(t, s, player1, store.NotificationTypeFriendRequest)
	createNotification(t, s, player1, store.NotificationTypeBanIssued)
	createNotification(t, s, player2, store.NotificationTypeRoomInvitation)

	list1, _ := s.List(ctx, player1, false, time.Time{})
	list2, _ := s.List(ctx, player2, false, time.Time{})

	if len(list1) != 2 {
		t.Errorf("player1: expected 2, got %d", len(list1))
	}
	if len(list2) != 1 {
		t.Errorf("player2: expected 1, got %d", len(list2))
	}
}
