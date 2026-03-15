package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tableforge/server/internal/platform/store"
)

func putJSON(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGetPlayerSettings_Defaults(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSON(t, router, "/api/v1/players/"+player.ID.String()+"/settings")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result store.PlayerSettings
	json.NewDecoder(w.Body).Decode(&result)

	if result.PlayerID != player.ID {
		t.Errorf("expected player_id %s, got %s", player.ID, result.PlayerID)
	}

	defaults := store.DefaultPlayerSettings()
	if result.Settings.Theme == nil || *result.Settings.Theme != *defaults.Theme {
		t.Errorf("expected default theme %s", *defaults.Theme)
	}
}

func TestGetPlayerSettings_InvalidID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/players/not-a-uuid/settings")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpsertPlayerSettings(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "bob")

	theme := "light"
	w := putJSON(t, router, "/api/v1/players/"+player.ID.String()+"/settings",
		store.PlayerSettingMap{Theme: &theme},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result store.PlayerSettings
	json.NewDecoder(w.Body).Decode(&result)

	if result.Settings.Theme == nil || *result.Settings.Theme != "light" {
		t.Errorf("expected theme light, got %v", result.Settings.Theme)
	}
}

func TestUpsertPlayerSettings_InvalidID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := putJSON(t, router, "/api/v1/players/not-a-uuid/settings",
		store.PlayerSettingMap{},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpsertPlayerSettings_InvalidBody(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "carol")

	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/players/"+player.ID.String()+"/settings",
		bytes.NewReader([]byte("not json")),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpsertPlayerSettings_PersistsAcrossRequests(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "dave")

	// Save settings.
	f := false
	putJSON(t, router, "/api/v1/players/"+player.ID.String()+"/settings",
		store.PlayerSettingMap{NotifyDM: &f},
	)

	// Fetch and verify persistence.
	w := getJSON(t, router, "/api/v1/players/"+player.ID.String()+"/settings")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result store.PlayerSettings
	json.NewDecoder(w.Body).Decode(&result)

	if result.Settings.NotifyDM == nil || *result.Settings.NotifyDM != false {
		t.Errorf("expected notify_dm false, got %v", result.Settings.NotifyDM)
	}
}

func TestUpsertPlayerSettings_MultipleFields(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "eve")

	theme := "light"
	lang := "es"
	reduceMotion := true

	w := putJSON(t, router, "/api/v1/players/"+player.ID.String()+"/settings",
		store.PlayerSettingMap{
			Theme:        &theme,
			Language:     &lang,
			ReduceMotion: &reduceMotion,
		},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result store.PlayerSettings
	json.NewDecoder(w.Body).Decode(&result)

	if result.Settings.Theme == nil || *result.Settings.Theme != "light" {
		t.Errorf("expected theme light")
	}
	if result.Settings.Language == nil || *result.Settings.Language != "es" {
		t.Errorf("expected language es")
	}
	if result.Settings.ReduceMotion == nil || *result.Settings.ReduceMotion != true {
		t.Errorf("expected reduce_motion true")
	}
}
