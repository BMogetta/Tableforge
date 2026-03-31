package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/tableforge/shared/middleware"
	"github.com/tableforge/services/user-service/internal/store"
)

func TestGetPlayerSettings_Defaults(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/settings", player)

	rec := getJSONAs(t, router, path, player, sharedmw.RolePlayer)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	settings := decodeResponse[store.PlayerSettings](t, rec)
	if settings.PlayerID != player {
		t.Fatalf("expected player_id=%s, got %s", player, settings.PlayerID)
	}
}

func TestUpsertAndGetPlayerSettings(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/settings", player)

	// Upsert custom settings.
	theme := "light"
	rec := putJSONAs(t, router, path, player, sharedmw.RolePlayer, store.PlayerSettingMap{
		Theme: &theme,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on upsert, got %d: %s", rec.Code, rec.Body.String())
	}

	// Read back.
	rec = getJSONAs(t, router, path, player, sharedmw.RolePlayer)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d: %s", rec.Code, rec.Body.String())
	}

	settings := decodeResponse[store.PlayerSettings](t, rec)
	if settings.Settings.Theme == nil || *settings.Settings.Theme != "light" {
		t.Fatalf("expected theme=light, got %v", settings.Settings.Theme)
	}
}
