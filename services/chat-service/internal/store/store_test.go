package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/services/chat-service/internal/store"
	"github.com/recess/shared/testutil"
)

type testEnv struct {
	store store.Store
	pool  *pgxpool.Pool
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return testEnv{store: s, pool: pool}
}

func (e testEnv) seedPlayer(t *testing.T, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO players (id, username) VALUES ($1, $2)`, id, username)
	if err != nil {
		t.Fatalf("seedPlayer: %v", err)
	}
	return id
}

func (e testEnv) seedRoom(t *testing.T, ownerID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	code := uuid.New().String()[:8]
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO rooms (id, code, game_id, owner_id, max_players) VALUES ($1, $2, 'tictactoe', $3, 2)`,
		id, code, ownerID)
	if err != nil {
		t.Fatalf("seedRoom: %v", err)
	}
	return id
}

func (e testEnv) addPlayerToRoom(t *testing.T, roomID, playerID uuid.UUID, seat int) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO room_players (room_id, player_id, seat) VALUES ($1, $2, $3)`,
		roomID, playerID, seat)
	if err != nil {
		t.Fatalf("addPlayerToRoom: %v", err)
	}
}

// --- Room messages -----------------------------------------------------------

func TestSaveAndGetRoomMessages(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.seedPlayer(t, "alice")
	roomID := env.seedRoom(t, owner)

	msg, err := env.store.SaveRoomMessage(ctx, roomID, owner, "hello room")
	if err != nil {
		t.Fatalf("SaveRoomMessage: %v", err)
	}
	if msg.Content != "hello room" {
		t.Errorf("expected 'hello room', got %q", msg.Content)
	}
	if msg.Hidden || msg.Reported {
		t.Error("expected hidden=false and reported=false")
	}

	messages, err := env.store.GetRoomMessages(ctx, roomID, 100, 0)
	if err != nil {
		t.Fatalf("GetRoomMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ID != msg.ID {
		t.Errorf("message ID mismatch")
	}
}

func TestHideRoomMessage(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.seedPlayer(t, "bob")
	roomID := env.seedRoom(t, owner)

	msg, _ := env.store.SaveRoomMessage(ctx, roomID, owner, "bad message")

	if err := env.store.HideRoomMessage(ctx, msg.ID); err != nil {
		t.Fatalf("HideRoomMessage: %v", err)
	}

	messages, _ := env.store.GetRoomMessages(ctx, roomID, 100, 0)
	if !messages[0].Hidden {
		t.Error("expected message to be hidden")
	}
}

func TestHideRoomMessageNotFound(t *testing.T) {
	env := newTestEnv(t)
	err := env.store.HideRoomMessage(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for non-existent message")
	}
}

func TestReportRoomMessage(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.seedPlayer(t, "carol")
	roomID := env.seedRoom(t, owner)
	msg, _ := env.store.SaveRoomMessage(ctx, roomID, owner, "offensive")

	if err := env.store.ReportRoomMessage(ctx, msg.ID); err != nil {
		t.Fatalf("ReportRoomMessage: %v", err)
	}

	messages, _ := env.store.GetRoomMessages(ctx, roomID, 100, 0)
	if !messages[0].Reported {
		t.Error("expected message to be reported")
	}
}

// --- IsRoomParticipant -------------------------------------------------------

func TestIsRoomParticipant(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.seedPlayer(t, "dave")
	spectator := env.seedPlayer(t, "eve")
	roomID := env.seedRoom(t, owner)
	env.addPlayerToRoom(t, roomID, owner, 0)

	ok, err := env.store.IsRoomParticipant(ctx, roomID, owner)
	if err != nil {
		t.Fatalf("IsRoomParticipant: %v", err)
	}
	if !ok {
		t.Error("expected owner to be a participant")
	}

	ok, _ = env.store.IsRoomParticipant(ctx, roomID, spectator)
	if ok {
		t.Error("expected spectator to not be a participant")
	}
}

// --- Direct messages ---------------------------------------------------------

func TestSaveAndGetDMHistory(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	dm1, err := env.store.SaveDM(ctx, alice, bob, "hey bob")
	if err != nil {
		t.Fatalf("SaveDM: %v", err)
	}
	if dm1.SenderID != alice || dm1.ReceiverID != bob {
		t.Error("sender/receiver mismatch")
	}

	dm2, _ := env.store.SaveDM(ctx, bob, alice, "hey alice")

	history, err := env.store.GetDMHistory(ctx, alice, bob, 50, 0)
	if err != nil {
		t.Fatalf("GetDMHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	// Ordered by created_at ASC (chronological).
	if history[0].ID != dm1.ID || history[1].ID != dm2.ID {
		t.Error("expected messages in chronological order")
	}
}

func TestMarkDMReadAndUnreadCount(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	env.store.SaveDM(ctx, alice, bob, "msg 1")
	dm2, _ := env.store.SaveDM(ctx, alice, bob, "msg 2")

	count, err := env.store.GetUnreadDMCount(ctx, bob)
	if err != nil {
		t.Fatalf("GetUnreadDMCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 unread, got %d", count)
	}

	senderID, marked, err := env.store.MarkDMRead(ctx, dm2.ID, bob)
	if err != nil {
		t.Fatalf("MarkDMRead: %v", err)
	}
	if !marked {
		t.Fatal("expected marked=true for unread DM")
	}
	if senderID != alice {
		t.Errorf("expected sender %s, got %s", alice, senderID)
	}

	// Re-marking must be a no-op and report marked=false.
	if _, marked, err := env.store.MarkDMRead(ctx, dm2.ID, bob); err != nil || marked {
		t.Errorf("expected marked=false on already-read DM, got marked=%v err=%v", marked, err)
	}

	count, _ = env.store.GetUnreadDMCount(ctx, bob)
	if count != 1 {
		t.Errorf("expected 1 unread after marking one read, got %d", count)
	}
}

func TestListDMConversations(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")
	carol := env.seedPlayer(t, "carol")

	env.store.SaveDM(ctx, alice, bob, "hi bob")
	env.store.SaveDM(ctx, carol, alice, "hi alice")

	convos, err := env.store.ListDMConversations(ctx, alice)
	if err != nil {
		t.Fatalf("ListDMConversations: %v", err)
	}
	if len(convos) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(convos))
	}

	// Most recent first (carol→alice was last).
	if convos[0].OtherPlayerID != carol {
		t.Errorf("expected first conversation to be with carol")
	}
	if convos[0].OtherUsername != "carol" {
		t.Errorf("expected username carol, got %s", convos[0].OtherUsername)
	}
}

func TestListDMConversationsUnreadCount(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	env.store.SaveDM(ctx, bob, alice, "msg 1")
	env.store.SaveDM(ctx, bob, alice, "msg 2")

	convos, _ := env.store.ListDMConversations(ctx, alice)
	if len(convos) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convos))
	}
	if convos[0].UnreadCount != 2 {
		t.Errorf("expected 2 unread, got %d", convos[0].UnreadCount)
	}
}

func TestReportDM(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	dm, _ := env.store.SaveDM(ctx, alice, bob, "offensive dm")

	if err := env.store.ReportDM(ctx, dm.ID, alice, bob); err != nil {
		t.Fatalf("ReportDM: %v", err)
	}

	history, _ := env.store.GetDMHistory(ctx, alice, bob, 50, 0)
	if !history[0].Reported {
		t.Error("expected DM to be reported")
	}
}

func TestReportDMNotFound(t *testing.T) {
	env := newTestEnv(t)
	err := env.store.ReportDM(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if err == nil {
		t.Error("expected error for non-existent DM")
	}
}
