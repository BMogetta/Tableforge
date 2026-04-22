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

func TestGetProfile(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	bio := "hello world"
	if _, err := st.UpsertProfile(context.Background(),store.UpsertProfileParams{
		PlayerID: playerID,
		Bio:      &bio,
	}); err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf("/api/v1/players/%s/profile", playerID)
	rec := getJSONAs(t, router, path, playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[store.PlayerProfile](t, rec)
	if resp.Bio == nil || *resp.Bio != bio {
		t.Fatalf("expected bio=%q, got %v", bio, resp.Bio)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/profile", playerID)
	rec := getJSONAs(t, router, path, playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpsertProfile(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/profile", playerID)
	rec := putJSONAs(t, router, path, playerID, sharedmw.RolePlayer, map[string]any{
		"bio":     "new bio",
		"country": "AR",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[store.PlayerProfile](t, rec)
	if resp.Bio == nil || *resp.Bio != "new bio" {
		t.Fatalf("expected bio=new bio, got %v", resp.Bio)
	}
	if resp.Country == nil || *resp.Country != "AR" {
		t.Fatalf("expected country=AR, got %v", resp.Country)
	}
}

func TestUpsertProfile_Forbidden(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	target := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/profile", target)
	rec := putJSONAs(t, router, path, caller, sharedmw.RolePlayer, map[string]any{
		"bio": "hacked",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListAchievements(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	if _, err := st.UpsertAchievement(context.Background(),playerID, "first_win", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertAchievement(context.Background(),playerID, "ten_games", 1, 10); err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf("/api/v1/players/%s/achievements", playerID)
	rec := getJSONAs(t, router, path, playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.PlayerAchievement](t, rec)
	if len(resp) != 2 {
		t.Fatalf("expected 2 achievements, got %d", len(resp))
	}
}

func TestListAchievements_Empty(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/achievements", playerID)
	rec := getJSONAs(t, router, path, playerID, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[[]store.PlayerAchievement](t, rec)
	if len(resp) != 0 {
		t.Fatalf("expected 0 achievements, got %d", len(resp))
	}
}

func TestSearchPlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	st.players = append(st.players, store.Player{
		ID:       playerID,
		Username: "alice",
	})

	caller := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/players/search?username=alice", caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeResponse[map[string]any](t, rec)
	if resp["username"] != "alice" {
		t.Fatalf("expected username=alice, got %v", resp["username"])
	}
}

func TestSearchPlayer_NotFound(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/players/search?username=nobody", caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSearchPlayer_MissingUsername(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	caller := uuid.New()
	rec := getJSONAs(t, router, "/api/v1/players/search", caller, sharedmw.RolePlayer)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
