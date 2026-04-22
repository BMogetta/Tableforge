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

func TestMutePlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/mute", caller)
	rec := postJSONAs(t, router, path, caller, sharedmw.RolePlayer, map[string]any{
		"muted_id": target.String(),
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	mutes, _ := st.GetMutedPlayers(context.Background(),caller)
	if len(mutes) != 1 {
		t.Fatalf("expected 1 mute, got %d", len(mutes))
	}
	if mutes[0].MutedID != target {
		t.Fatalf("expected muted_id=%s, got %s", target, mutes[0].MutedID)
	}
}

func TestMutePlayer_Self(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/mute", caller)
	rec := postJSONAs(t, router, path, caller, sharedmw.RolePlayer, map[string]any{
		"muted_id": caller.String(),
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMutePlayer_Forbidden(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	other := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/mute", other)
	rec := postJSONAs(t, router, path, caller, sharedmw.RolePlayer, map[string]any{
		"muted_id": target.String(),
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnmutePlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	target := uuid.New()
	if err := st.MutePlayer(context.Background(),caller, target); err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf("/api/v1/players/%s/mute/%s", caller, target)
	rec := deleteJSONAs(t, router, path, caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	mutes, _ := st.GetMutedPlayers(context.Background(),caller)
	if len(mutes) != 0 {
		t.Fatalf("expected 0 mutes, got %d", len(mutes))
	}
}

func TestUnmutePlayer_Forbidden(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	other := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/mute/%s", other, target)
	rec := deleteJSONAs(t, router, path, caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMutes(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	t1 := uuid.New()
	t2 := uuid.New()
	if err := st.MutePlayer(context.Background(),caller, t1); err != nil {
		t.Fatal(err)
	}
	if err := st.MutePlayer(context.Background(),caller, t2); err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf("/api/v1/players/%s/mutes", caller)
	rec := getJSONAs(t, router, path, caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.PlayerMute](t, rec)
	if len(resp) != 2 {
		t.Fatalf("expected 2 mutes, got %d", len(resp))
	}
}

func TestGetMutes_ManagerOverride(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	target := uuid.New()
	if err := st.MutePlayer(context.Background(),player, target); err != nil {
		t.Fatal(err)
	}

	manager := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/mutes", player)
	rec := getJSONAs(t, router, path, manager, sharedmw.RoleManager)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.PlayerMute](t, rec)
	if len(resp) != 1 {
		t.Fatalf("expected 1 mute, got %d", len(resp))
	}
}

func TestGetMutes_ForbiddenOtherPlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	other := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/mutes", player)
	rec := getJSONAs(t, router, path, other, sharedmw.RolePlayer)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
