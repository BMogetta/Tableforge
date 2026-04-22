package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/recess/shared/middleware"
)

func TestIssueBan(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	target := uuid.New()
	reason := "cheating"

	path := fmt.Sprintf("/api/v1/admin/players/%s/ban", target)
	rec := postJSONAs(t, router, path, manager, sharedmw.RoleManager, map[string]any{
		"reason": reason,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	bans, _ := st.ListBans(context.Background(), target)
	if len(bans) != 1 {
		t.Fatalf("expected 1 ban, got %d", len(bans))
	}
	if bans[0].BannedBy != manager {
		t.Fatalf("expected banned_by=%s, got %s", manager, bans[0].BannedBy)
	}
}

func TestIssueBan_Unauthorized(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/admin/players/%s/ban", target)
	rec := postJSONAs(t, router, path, player, sharedmw.RolePlayer, map[string]any{
		"reason": "trolling",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLiftBan(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	target := uuid.New()

	// Issue a ban first.
	issuePath := fmt.Sprintf("/api/v1/admin/players/%s/ban", target)
	postJSONAs(t, router, issuePath, manager, sharedmw.RoleManager, map[string]any{
		"reason": "testing",
	})

	bans, _ := st.ListBans(context.Background(), target)
	if len(bans) == 0 {
		t.Fatal("expected ban to exist")
	}

	liftPath := fmt.Sprintf("/api/v1/admin/bans/%s", bans[0].ID)
	rec := deleteJSONAs(t, router, liftPath, manager, sharedmw.RoleManager)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify ban was lifted.
	bans, _ = st.ListBans(context.Background(), target)
	if bans[0].LiftedAt == nil {
		t.Fatal("expected ban to be lifted")
	}
	if *bans[0].LiftedBy != manager {
		t.Fatalf("expected lifted_by=%s, got %s", manager, *bans[0].LiftedBy)
	}
}

func TestLiftBan_NotFound(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()
	fakeBanID := uuid.New()

	liftPath := fmt.Sprintf("/api/v1/admin/bans/%s", fakeBanID)
	rec := deleteJSONAs(t, router, liftPath, manager, sharedmw.RoleManager)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
