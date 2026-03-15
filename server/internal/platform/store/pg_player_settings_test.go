package store_test

import (
	"context"
	"testing"

	"github.com/tableforge/server/internal/platform/store"
)

func TestGetPlayerSettings_Defaults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "alice")

	// No row exists yet — must return defaults without error.
	settings, err := s.GetPlayerSettings(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}

	if settings.PlayerID != player.ID {
		t.Errorf("expected player_id %s, got %s", player.ID, settings.PlayerID)
	}

	defaults := store.DefaultPlayerSettings()

	if *settings.Settings.Theme != *defaults.Theme {
		t.Errorf("expected theme %s, got %s", *defaults.Theme, *settings.Settings.Theme)
	}
	if *settings.Settings.Language != *defaults.Language {
		t.Errorf("expected language %s, got %s", *defaults.Language, *settings.Settings.Language)
	}
	if *settings.Settings.NotifyDM != *defaults.NotifyDM {
		t.Errorf("expected notify_dm %v, got %v", *defaults.NotifyDM, *settings.Settings.NotifyDM)
	}
}

func TestUpsertPlayerSettings_CreateAndFetch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "bob")

	theme := "light"
	patch := store.PlayerSettingMap{
		Theme: &theme,
	}

	saved, err := s.UpsertPlayerSettings(ctx, player.ID, patch)
	if err != nil {
		t.Fatalf("UpsertPlayerSettings: %v", err)
	}
	if *saved.Settings.Theme != "light" {
		t.Errorf("expected theme light, got %s", *saved.Settings.Theme)
	}

	// Fetch and verify the stored value is merged over defaults.
	fetched, err := s.GetPlayerSettings(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetPlayerSettings after upsert: %v", err)
	}
	if *fetched.Settings.Theme != "light" {
		t.Errorf("expected theme light after fetch, got %s", *fetched.Settings.Theme)
	}

	// Keys not in patch should fall back to defaults.
	defaults := store.DefaultPlayerSettings()
	if *fetched.Settings.Language != *defaults.Language {
		t.Errorf("expected default language %s, got %s", *defaults.Language, *fetched.Settings.Language)
	}
}

func TestUpsertPlayerSettings_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "carol")

	theme := "dark"
	s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{Theme: &theme})

	// Update with a different value.
	light := "light"
	_, err := s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{Theme: &light})
	if err != nil {
		t.Fatalf("UpsertPlayerSettings update: %v", err)
	}

	fetched, _ := s.GetPlayerSettings(ctx, player.ID)
	if *fetched.Settings.Theme != "light" {
		t.Errorf("expected updated theme light, got %s", *fetched.Settings.Theme)
	}
}

func TestUpsertPlayerSettings_PartialPatch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "dave")

	// First upsert — set theme and language.
	theme := "light"
	lang := "es"
	s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{
		Theme:    &theme,
		Language: &lang,
	})

	// Second upsert — only change theme, language should persist.
	dark := "dark"
	s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{
		Theme:    &dark,
		Language: &lang,
	})

	fetched, _ := s.GetPlayerSettings(ctx, player.ID)
	if *fetched.Settings.Theme != "dark" {
		t.Errorf("expected theme dark, got %s", *fetched.Settings.Theme)
	}
	if *fetched.Settings.Language != "es" {
		t.Errorf("expected language es, got %s", *fetched.Settings.Language)
	}
}

func TestUpsertPlayerSettings_BooleanFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "eve")

	f := false
	t2 := true
	_, err := s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{
		NotifyDM:      &f,
		ShowMoveHints: &t2,
		MuteAll:       &t2,
	})
	if err != nil {
		t.Fatalf("UpsertPlayerSettings booleans: %v", err)
	}

	fetched, _ := s.GetPlayerSettings(ctx, player.ID)
	if *fetched.Settings.NotifyDM != false {
		t.Errorf("expected notify_dm false, got %v", *fetched.Settings.NotifyDM)
	}
	if *fetched.Settings.ShowMoveHints != true {
		t.Errorf("expected show_move_hints true, got %v", *fetched.Settings.ShowMoveHints)
	}
	if *fetched.Settings.MuteAll != true {
		t.Errorf("expected mute_all true, got %v", *fetched.Settings.MuteAll)
	}
}

func TestUpsertPlayerSettings_VolumeFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "frank")

	vol := 0.5
	_, err := s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{
		VolumeMaster: &vol,
		VolumeSFX:    &vol,
	})
	if err != nil {
		t.Fatalf("UpsertPlayerSettings volumes: %v", err)
	}

	fetched, _ := s.GetPlayerSettings(ctx, player.ID)
	if *fetched.Settings.VolumeMaster != 0.5 {
		t.Errorf("expected volume_master 0.5, got %v", *fetched.Settings.VolumeMaster)
	}
	if *fetched.Settings.VolumeSFX != 0.5 {
		t.Errorf("expected volume_sfx 0.5, got %v", *fetched.Settings.VolumeSFX)
	}
	// Unset volume should fall back to default (1.0).
	defaults := store.DefaultPlayerSettings()
	if *fetched.Settings.VolumeMusic != *defaults.VolumeMusic {
		t.Errorf("expected default volume_music %v, got %v", *defaults.VolumeMusic, *fetched.Settings.VolumeMusic)
	}
}

func TestGetPlayerSettings_UpdatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	player, _ := s.CreatePlayer(ctx, "grace")

	// No row — updated_at should be zero.
	before, _ := s.GetPlayerSettings(ctx, player.ID)
	if !before.UpdatedAt.IsZero() {
		t.Errorf("expected zero updated_at for missing row, got %v", before.UpdatedAt)
	}

	theme := "dark"
	saved, _ := s.UpsertPlayerSettings(ctx, player.ID, store.PlayerSettingMap{Theme: &theme})
	if saved.UpdatedAt.IsZero() {
		t.Error("expected non-zero updated_at after upsert")
	}
}
