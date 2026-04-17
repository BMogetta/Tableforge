package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/notification-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
	sharedws "github.com/recess/shared/ws"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	notifications map[uuid.UUID]store.Notification
	actions       map[uuid.UUID]string
}

func newMockStore() *mockStore {
	return &mockStore{
		notifications: make(map[uuid.UUID]store.Notification),
		actions:       make(map[uuid.UUID]string),
	}
}

func (m *mockStore) Create(_ context.Context, p store.CreateParams) (store.Notification, error) {
	n := store.Notification{
		ID:              uuid.New(),
		PlayerID:        p.PlayerID,
		Type:            p.Type,
		Payload:         p.Payload,
		ActionExpiresAt: p.ActionExpiresAt,
		CreatedAt:       time.Now(),
	}
	m.notifications[n.ID] = n
	return n, nil
}

func (m *mockStore) Get(_ context.Context, id uuid.UUID) (store.Notification, error) {
	n, ok := m.notifications[id]
	if !ok {
		return store.Notification{}, errNotificationNotFound
	}
	return n, nil
}

func (m *mockStore) List(_ context.Context, playerID uuid.UUID, includeRead bool, _ time.Time, limit, offset int) ([]store.Notification, error) {
	var result []store.Notification
	for _, n := range m.notifications {
		if n.PlayerID != playerID {
			continue
		}
		if !includeRead && n.ReadAt != nil {
			continue
		}
		result = append(result, n)
	}
	if offset > len(result) {
		return []store.Notification{}, nil
	}
	result = result[offset:]
	if limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockStore) CountNotifications(_ context.Context, playerID uuid.UUID, includeRead bool, _ time.Time) (int, error) {
	count := 0
	for _, n := range m.notifications {
		if n.PlayerID != playerID {
			continue
		}
		if !includeRead && n.ReadAt != nil {
			continue
		}
		count++
	}
	return count, nil
}

func (m *mockStore) MarkRead(_ context.Context, id uuid.UUID) error {
	if n, ok := m.notifications[id]; ok {
		now := time.Now()
		n.ReadAt = &now
		m.notifications[id] = n
	}
	return nil
}

func (m *mockStore) SetAction(_ context.Context, id uuid.UUID, action string) error {
	n, ok := m.notifications[id]
	if !ok {
		return store.ErrActionUnavailable
	}
	if n.ActionTaken != nil {
		return store.ErrActionUnavailable
	}
	if n.ActionExpiresAt != nil && time.Now().After(*n.ActionExpiresAt) {
		return store.ErrActionUnavailable
	}
	n.ActionTaken = &action
	m.notifications[id] = n
	m.actions[id] = action
	return nil
}

// --- stub publisher ----------------------------------------------------------

type stubPublisher struct {
	events []sharedws.Event
}

func (p *stubPublisher) PublishToPlayer(_ context.Context, _ uuid.UUID, event sharedws.Event) {
	p.events = append(p.events, event)
}

// --- helpers -----------------------------------------------------------------

// noopExecutor is a no-op ActionExecutor for tests that don't test side effects.
type noopExecutor struct{}

func (noopExecutor) AcceptFriendRequest(_ context.Context, _, _ uuid.UUID) error { return nil }

func newTestHandler(st NotificationStore, pub Publisher) http.Handler {
	r := chi.NewRouter()
	New(st, pub, noopExecutor{}, slog.Default()).RegisterRoutes(r)
	return r
}

func withAuth(r *http.Request, playerID uuid.UUID, role string) *http.Request {
	ctx := sharedmw.ContextWithPlayer(r.Context(), playerID, "testuser", role)
	return r.WithContext(ctx)
}

func postAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, playerID, sharedmw.RolePlayer)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withAuth(req, playerID, sharedmw.RolePlayer)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// --- tests -------------------------------------------------------------------

func TestListNotifications(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()

	st.notifications[uuid.New()] = store.Notification{
		ID:        uuid.New(),
		PlayerID:  playerID,
		Type:      store.NotificationTypeFriendRequest,
		Payload:   json.RawMessage(`{"from_player_id":"abc"}`),
		CreatedAt: time.Now(),
	}
	// Re-key with correct ID
	for id, n := range st.notifications {
		n.ID = id
		st.notifications[id] = n
	}

	handler := newTestHandler(st, pub)
	rec := getAs(t, handler, "/api/v1/players/"+playerID.String()+"/notifications", playerID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListNotifications_Forbidden(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	otherID := uuid.New()

	handler := newTestHandler(st, pub)
	rec := getAs(t, handler, "/api/v1/players/"+playerID.String()+"/notifications", otherID)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestMarkRead(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()

	st.notifications[notifID] = store.Notification{
		ID:        notifID,
		PlayerID:  playerID,
		Type:      store.NotificationTypeFriendRequest,
		Payload:   json.RawMessage(`{}`),
		CreatedAt: time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/read", playerID, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	n := st.notifications[notifID]
	if n.ReadAt == nil {
		t.Error("expected notification to be marked as read")
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+uuid.New().String()+"/read", playerID, nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAccept(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()
	expires := time.Now().Add(24 * time.Hour)

	st.notifications[notifID] = store.Notification{
		ID:              notifID,
		PlayerID:        playerID,
		Type:            store.NotificationTypeFriendRequest,
		Payload:         json.RawMessage(`{}`),
		ActionExpiresAt: &expires,
		CreatedAt:       time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/accept", playerID, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if st.actions[notifID] != "accepted" {
		t.Errorf("expected action 'accepted', got %q", st.actions[notifID])
	}
}

func TestAccept_BanNotSupported(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()

	st.notifications[notifID] = store.Notification{
		ID:        notifID,
		PlayerID:  playerID,
		Type:      store.NotificationTypeBanIssued,
		Payload:   json.RawMessage(`{}`),
		CreatedAt: time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/accept", playerID, nil)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for ban notification, got %d", rec.Code)
	}
}

func TestAccept_AlreadyTaken(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()
	action := "accepted"

	st.notifications[notifID] = store.Notification{
		ID:          notifID,
		PlayerID:    playerID,
		Type:        store.NotificationTypeFriendRequest,
		Payload:     json.RawMessage(`{}`),
		ActionTaken: &action,
		CreatedAt:   time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/accept", playerID, nil)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestAccept_Expired(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()
	expired := time.Now().Add(-1 * time.Hour)

	st.notifications[notifID] = store.Notification{
		ID:              notifID,
		PlayerID:        playerID,
		Type:            store.NotificationTypeFriendRequest,
		Payload:         json.RawMessage(`{}`),
		ActionExpiresAt: &expired,
		CreatedAt:       time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/accept", playerID, nil)

	// Expired + already-taken are collapsed into a generic 409 so clients
	// don't need to branch on two near-identical failure modes.
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestDecline(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()
	notifID := uuid.New()
	expires := time.Now().Add(24 * time.Hour)

	st.notifications[notifID] = store.Notification{
		ID:              notifID,
		PlayerID:        playerID,
		Type:            store.NotificationTypeRoomInvitation,
		Payload:         json.RawMessage(`{}`),
		ActionExpiresAt: &expires,
		CreatedAt:       time.Now(),
	}

	handler := newTestHandler(st, pub)
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/decline", playerID, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	if st.actions[notifID] != "declined" {
		t.Errorf("expected action 'declined', got %q", st.actions[notifID])
	}
}

func TestCreate_Internal(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()

	handler := newTestHandler(st, pub)

	body := store.CreateParams{
		PlayerID: playerID,
		Type:     store.NotificationTypeFriendRequest,
		Payload:  json.RawMessage(`{"from_player_id":"test"}`),
	}

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest(http.MethodPost, "/internal/notifications", &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(pub.events) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.events))
	}
	if pub.events[0].Type != sharedws.EventNotificationReceived {
		t.Errorf("expected event type %s, got %s", sharedws.EventNotificationReceived, pub.events[0].Type)
	}
}
