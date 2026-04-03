package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/services/chat-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	roomMessages  map[uuid.UUID][]store.RoomMessage
	directMsgs    []store.DirectMessage
	participants  map[string]bool // key: "roomID:playerID"
	hidden        map[uuid.UUID]bool
	reported      map[uuid.UUID]bool
	readMessages  map[uuid.UUID]bool
	unreadCounts  map[uuid.UUID]int
	saveDMErr     error
}

func newMockStore() *mockStore {
	return &mockStore{
		roomMessages: make(map[uuid.UUID][]store.RoomMessage),
		participants: make(map[string]bool),
		hidden:       make(map[uuid.UUID]bool),
		reported:     make(map[uuid.UUID]bool),
		readMessages: make(map[uuid.UUID]bool),
		unreadCounts: make(map[uuid.UUID]int),
	}
}

func (m *mockStore) participantKey(roomID, playerID uuid.UUID) string {
	return roomID.String() + ":" + playerID.String()
}

func (m *mockStore) SaveRoomMessage(_ context.Context, roomID, playerID uuid.UUID, content string) (store.RoomMessage, error) {
	msg := store.RoomMessage{
		ID:        uuid.New(),
		RoomID:    roomID,
		PlayerID:  playerID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	m.roomMessages[roomID] = append(m.roomMessages[roomID], msg)
	return msg, nil
}

func (m *mockStore) GetRoomMessages(_ context.Context, roomID uuid.UUID) ([]store.RoomMessage, error) {
	return m.roomMessages[roomID], nil
}

func (m *mockStore) HideRoomMessage(_ context.Context, messageID uuid.UUID) error {
	if m.hidden[messageID] {
		return nil
	}
	m.hidden[messageID] = true
	return nil
}

func (m *mockStore) ReportRoomMessage(_ context.Context, messageID uuid.UUID) error {
	m.reported[messageID] = true
	return nil
}

func (m *mockStore) IsRoomParticipant(_ context.Context, roomID, playerID uuid.UUID) (bool, error) {
	return m.participants[m.participantKey(roomID, playerID)], nil
}

func (m *mockStore) SaveDM(_ context.Context, senderID, receiverID uuid.UUID, content string) (store.DirectMessage, error) {
	if m.saveDMErr != nil {
		return store.DirectMessage{}, m.saveDMErr
	}
	msg := store.DirectMessage{
		ID:         uuid.New(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		CreatedAt:  time.Now(),
	}
	m.directMsgs = append(m.directMsgs, msg)
	return msg, nil
}

func (m *mockStore) GetDMHistory(_ context.Context, playerA, playerB uuid.UUID) ([]store.DirectMessage, error) {
	var result []store.DirectMessage
	for _, msg := range m.directMsgs {
		if (msg.SenderID == playerA && msg.ReceiverID == playerB) ||
			(msg.SenderID == playerB && msg.ReceiverID == playerA) {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (m *mockStore) MarkDMRead(_ context.Context, messageID uuid.UUID) error {
	m.readMessages[messageID] = true
	return nil
}

func (m *mockStore) GetUnreadDMCount(_ context.Context, playerID uuid.UUID) (int, error) {
	return m.unreadCounts[playerID], nil
}

func (m *mockStore) ListDMConversations(_ context.Context, _ uuid.UUID) ([]store.DMConversation, error) {
	return []store.DMConversation{}, nil
}

func (m *mockStore) ReportDM(_ context.Context, messageID uuid.UUID) error {
	m.reported[messageID] = true
	return nil
}

// --- stub publisher ----------------------------------------------------------

type stubPublisher struct {
	roomEvents   []any
	playerEvents []any
}

func newStubPublisher() *Publisher {
	// Publisher requires a redis.Client, but we won't actually publish in tests.
	// Use nil — the test won't call publish since we override at handler level.
	// Instead, we use a no-op publisher wrapper.
	return nil
}

// --- helpers -----------------------------------------------------------------

func newTestRouter(st store.Store) http.Handler {
	// Stub publisher with nil redis — events logged but not published.
	pub := &Publisher{rdb: nil}
	noopAuth := func(next http.Handler) http.Handler { return next }
	return NewRouter(st, pub, noopAuth, nil, "chat-service-test")
}

func withAuth(r *http.Request, playerID uuid.UUID, role string) *http.Request {
	ctx := sharedmw.ContextWithPlayer(r.Context(), playerID, "testuser", role)
	return r.WithContext(ctx)
}

func postJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func deleteJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return v
}

// --- room message tests ------------------------------------------------------

func TestSendRoomMessage(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	playerID := uuid.New()
	st.participants[st.participantKey(roomID, playerID)] = true

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages", playerID, sharedmw.RolePlayer, map[string]string{
		"content": "hello room",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	msgs := st.roomMessages[roomID]
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message stored, got %d", len(msgs))
	}
	if msgs[0].Content != "hello room" {
		t.Errorf("expected content 'hello room', got %q", msgs[0].Content)
	}
}

func TestSendRoomMessage_SpectatorBlocked(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	playerID := uuid.New()
	// NOT adding to participants — spectator

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages", playerID, sharedmw.RolePlayer, map[string]string{
		"content": "hello",
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for spectator, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendRoomMessage_EmptyContent(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	playerID := uuid.New()
	st.participants[st.participantKey(roomID, playerID)] = true

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages", playerID, sharedmw.RolePlayer, map[string]string{
		"content": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty content, got %d", rec.Code)
	}
}

func TestGetRoomMessages(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	playerID := uuid.New()
	st.roomMessages[roomID] = []store.RoomMessage{
		{ID: uuid.New(), RoomID: roomID, PlayerID: playerID, Content: "msg1", CreatedAt: time.Now()},
		{ID: uuid.New(), RoomID: roomID, PlayerID: playerID, Content: "msg2", CreatedAt: time.Now()},
	}

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages", playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	msgs := decodeResponse[[]store.RoomMessage](t, rec)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestHideRoomMessage_ManagerOnly(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	messageID := uuid.New()
	playerID := uuid.New()

	router := newTestRouter(st)

	// Player role should be forbidden
	rec := deleteJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages/"+messageID.String(), playerID, sharedmw.RolePlayer)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for player role, got %d", rec.Code)
	}

	// Manager role should succeed
	rec = deleteJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages/"+messageID.String(), playerID, sharedmw.RoleManager)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for manager, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- DM tests ----------------------------------------------------------------

func TestSendDM(t *testing.T) {
	st := newMockStore()
	sender := uuid.New()
	receiver := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/players/"+receiver.String()+"/dm", sender, sharedmw.RolePlayer, map[string]string{
		"content": "hey",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(st.directMsgs) != 1 {
		t.Fatalf("expected 1 DM stored, got %d", len(st.directMsgs))
	}
	if st.directMsgs[0].SenderID != sender {
		t.Errorf("expected sender %s, got %s", sender, st.directMsgs[0].SenderID)
	}
}

func TestSendDM_ToSelf(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/players/"+playerID.String()+"/dm", playerID, sharedmw.RolePlayer, map[string]string{
		"content": "talking to myself",
	})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for self-DM, got %d", rec.Code)
	}
}

func TestGetDMHistory(t *testing.T) {
	st := newMockStore()
	playerA := uuid.New()
	playerB := uuid.New()
	st.directMsgs = []store.DirectMessage{
		{ID: uuid.New(), SenderID: playerA, ReceiverID: playerB, Content: "hi", CreatedAt: time.Now()},
		{ID: uuid.New(), SenderID: playerB, ReceiverID: playerA, Content: "hey", CreatedAt: time.Now()},
	}

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerA.String()+"/dm/"+playerB.String(), playerA, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	msgs := decodeResponse[[]store.DirectMessage](t, rec)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestGetDMHistory_Forbidden(t *testing.T) {
	st := newMockStore()
	playerA := uuid.New()
	playerB := uuid.New()
	intruder := uuid.New()

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerA.String()+"/dm/"+playerB.String(), intruder, sharedmw.RolePlayer)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-participant, got %d", rec.Code)
	}
}

func TestGetDMHistory_OwnerCanView(t *testing.T) {
	st := newMockStore()
	playerA := uuid.New()
	playerB := uuid.New()
	owner := uuid.New()

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerA.String()+"/dm/"+playerB.String(), owner, sharedmw.RoleOwner)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for owner, got %d", rec.Code)
	}
}

func TestGetUnreadDMCount(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()
	st.unreadCounts[playerID] = 5

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerID.String()+"/dm/unread", playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := decodeResponse[map[string]int](t, rec)
	if body["count"] != 5 {
		t.Errorf("expected count 5, got %d", body["count"])
	}
}

func TestGetUnreadDMCount_ForbiddenForOthers(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()
	otherID := uuid.New()

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerID.String()+"/dm/unread", otherID, sharedmw.RolePlayer)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// --- conversations -----------------------------------------------------------

func TestListDMConversations(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerID.String()+"/dm/conversations", playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.DMConversation](t, rec)
	if len(resp) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(resp))
	}
}

func TestListDMConversations_Forbidden(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()
	otherID := uuid.New()

	router := newTestRouter(st)
	rec := getJSONAs(t, router, "/api/v1/players/"+playerID.String()+"/dm/conversations", otherID, sharedmw.RolePlayer)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// --- mark DM read ------------------------------------------------------------

func TestMarkDMRead(t *testing.T) {
	st := newMockStore()
	caller := uuid.New()
	messageID := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/dm/"+messageID.String()+"/read", caller, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if !st.readMessages[messageID] {
		t.Error("expected message to be marked as read")
	}
}

// --- report room message -----------------------------------------------------

func TestReportRoomMessage(t *testing.T) {
	st := newMockStore()
	roomID := uuid.New()
	messageID := uuid.New()
	playerID := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/rooms/"+roomID.String()+"/messages/"+messageID.String()+"/report", playerID, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if !st.reported[messageID] {
		t.Error("expected message to be reported")
	}
}

// --- report DM ---------------------------------------------------------------

func TestReportDM(t *testing.T) {
	st := newMockStore()
	playerA := uuid.New()
	playerB := uuid.New()
	messageID := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/players/"+playerA.String()+"/dm/"+playerB.String()+"/report", playerA, sharedmw.RolePlayer, map[string]string{
		"message_id": messageID.String(),
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if !st.reported[messageID] {
		t.Error("expected DM to be reported")
	}
}

func TestReportDM_Forbidden(t *testing.T) {
	st := newMockStore()
	playerA := uuid.New()
	playerB := uuid.New()
	intruder := uuid.New()
	messageID := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/players/"+playerA.String()+"/dm/"+playerB.String()+"/report", intruder, sharedmw.RolePlayer, map[string]string{
		"message_id": messageID.String(),
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// --- send DM edge cases ------------------------------------------------------

func TestSendDM_EmptyContent(t *testing.T) {
	st := newMockStore()
	sender := uuid.New()
	receiver := uuid.New()

	router := newTestRouter(st)
	rec := postJSONAs(t, router, "/api/v1/players/"+receiver.String()+"/dm", sender, sharedmw.RolePlayer, map[string]string{
		"content": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty content, got %d", rec.Code)
	}
}
