package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// --- Broadcast ---------------------------------------------------------------

func TestBroadcast(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/broadcast", manager, sharedmw.RoleManager, map[string]any{
		"message": "Server maintenance in 5 minutes",
		"type":    "warning",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBroadcast_DefaultType(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/broadcast", manager, sharedmw.RoleManager, map[string]any{
		"message": "Welcome everyone!",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBroadcast_EmptyMessage(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/broadcast", manager, sharedmw.RoleManager, map[string]any{
		"message": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBroadcast_InvalidType(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/broadcast", manager, sharedmw.RoleManager, map[string]any{
		"message": "hello",
		"type":    "error",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBroadcast_Unauthorized(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/broadcast", player, sharedmw.RolePlayer, map[string]any{
		"message": "hello",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Allowed emails ----------------------------------------------------------

func TestListAllowedEmails(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	if _, err := st.AddAllowedEmail(context.Background(),store.AddAllowedEmailParams{
		Email: "a@test.com",
		Role:  store.RolePlayer,
	}); err != nil {
		t.Fatal(err)
	}

	rec := getJSONAs(t, router, "/api/v1/admin/allowed-emails", manager, sharedmw.RoleManager)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.AllowedEmail](t, rec)
	if len(resp) != 1 {
		t.Fatalf("expected 1 email, got %d", len(resp))
	}
}

func TestListAllowedEmails_Unauthorized(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/admin/allowed-emails", player, sharedmw.RolePlayer)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAddAllowedEmail(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/allowed-emails", manager, sharedmw.RoleManager, map[string]any{
		"email": "new@test.com",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	emails, _ := st.ListAllowedEmails(context.Background())
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
}

func TestAddAllowedEmail_ManagerCannotInviteManager(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/allowed-emails", manager, sharedmw.RoleManager, map[string]any{
		"email": "mgr@test.com",
		"role":  "manager",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAddAllowedEmail_OwnerCanInviteManager(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	owner := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/allowed-emails", owner, sharedmw.RoleOwner, map[string]any{
		"email": "mgr@test.com",
		"role":  "manager",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAddAllowedEmail_EmptyEmail(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := postJSONAs(t, router, "/api/v1/admin/allowed-emails", manager, sharedmw.RoleManager, map[string]any{
		"email": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRemoveAllowedEmail(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	if _, err := st.AddAllowedEmail(context.Background(),store.AddAllowedEmailParams{
		Email: "rm@test.com",
		Role:  store.RolePlayer,
	}); err != nil {
		t.Fatal(err)
	}

	rec := deleteJSONAs(t, router, "/api/v1/admin/allowed-emails/rm@test.com", manager, sharedmw.RoleManager)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	emails, _ := st.ListAllowedEmails(context.Background())
	if len(emails) != 0 {
		t.Fatalf("expected 0 emails, got %d", len(emails))
	}
}

func TestRemoveAllowedEmail_NotFound(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	rec := deleteJSONAs(t, router, "/api/v1/admin/allowed-emails/nope@test.com", manager, sharedmw.RoleManager)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- List players ------------------------------------------------------------

func TestListPlayers(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	st.players = []store.Player{
		{ID: uuid.New(), Username: "alice", Role: store.RolePlayer},
		{ID: uuid.New(), Username: "bob", Role: store.RoleManager},
	}

	manager := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/admin/players", manager, sharedmw.RoleManager)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.Player](t, rec)
	if len(resp) != 2 {
		t.Fatalf("expected 2 players, got %d", len(resp))
	}
}

func TestListPlayers_Unauthorized(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/admin/players", player, sharedmw.RolePlayer)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Set player role ---------------------------------------------------------

func TestSetPlayerRole(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	target := uuid.New()
	st.players = []store.Player{
		{ID: target, Username: "alice", Role: store.RolePlayer},
	}

	owner := uuid.New()
	path := fmt.Sprintf("/api/v1/admin/players/%s/role", target)
	rec := putJSONAs(t, router, path, owner, sharedmw.RoleOwner, map[string]any{
		"role": "manager",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	for _, p := range st.players {
		if p.ID == target && p.Role != store.RoleManager {
			t.Fatalf("expected role=manager, got %s", p.Role)
		}
	}
}

func TestSetPlayerRole_ManagerForbidden(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	target := uuid.New()
	st.players = []store.Player{
		{ID: target, Username: "alice", Role: store.RolePlayer},
	}

	manager := uuid.New()
	path := fmt.Sprintf("/api/v1/admin/players/%s/role", target)
	rec := putJSONAs(t, router, path, manager, sharedmw.RoleManager, map[string]any{
		"role": "manager",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetPlayerRole_CannotDemoteSelf(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	owner := uuid.New()
	st.players = []store.Player{
		{ID: owner, Username: "admin", Role: store.RoleOwner},
	}

	path := fmt.Sprintf("/api/v1/admin/players/%s/role", owner)
	rec := putJSONAs(t, router, path, owner, sharedmw.RoleOwner, map[string]any{
		"role": "player",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetPlayerRole_MissingRole(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	target := uuid.New()
	st.players = []store.Player{
		{ID: target, Username: "alice", Role: store.RolePlayer},
	}

	owner := uuid.New()
	path := fmt.Sprintf("/api/v1/admin/players/%s/role", target)
	rec := putJSONAs(t, router, path, owner, sharedmw.RoleOwner, map[string]any{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
