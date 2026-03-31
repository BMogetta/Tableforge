package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PlayerSettings holds a player's stored preferences merged over defaults.
type PlayerSettings struct {
	PlayerID  uuid.UUID        `json:"player_id"`
	Settings  PlayerSettingMap `json:"settings"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// PlayerSettingMap is the flat key/value map serialized into the JSONB column.
// All fields are pointers — nil means "use the application default".
type PlayerSettingMap struct {
	// Appearance
	Theme        *string `json:"theme,omitempty"`
	Language     *string `json:"language,omitempty"`
	ReduceMotion *bool   `json:"reduce_motion,omitempty"`
	FontSize     *string `json:"font_size,omitempty"`

	// Notifications
	NotifyDM            *bool `json:"notify_dm,omitempty"`
	NotifyGameInvite    *bool `json:"notify_game_invite,omitempty"`
	NotifyFriendRequest *bool `json:"notify_friend_request,omitempty"`
	NotifySound         *bool `json:"notify_sound,omitempty"`

	// Audio
	MuteAll             *bool    `json:"mute_all,omitempty"`
	VolumeMaster        *float64 `json:"volume_master,omitempty"`
	VolumeSFX           *float64 `json:"volume_sfx,omitempty"`
	VolumeUI            *float64 `json:"volume_ui,omitempty"`
	VolumeNotifications *float64 `json:"volume_notifications,omitempty"`
	VolumeMusic         *float64 `json:"volume_music,omitempty"`

	// Gameplay
	ShowMoveHints    *bool `json:"show_move_hints,omitempty"`
	ConfirmMove      *bool `json:"confirm_move,omitempty"`
	ShowTimerWarning *bool `json:"show_timer_warning,omitempty"`

	// Privacy
	ShowOnlineStatus *bool   `json:"show_online_status,omitempty"`
	AllowDMs         *string `json:"allow_dms,omitempty"`
}

func defaultPlayerSettings() PlayerSettingMap {
	t := true
	f := false
	dark := "dark"
	en := "en"
	medium := "medium"
	anyone := "anyone"
	vol1 := 1.0

	return PlayerSettingMap{
		Theme:               &dark,
		Language:            &en,
		ReduceMotion:        &f,
		FontSize:            &medium,
		NotifyDM:            &t,
		NotifyGameInvite:    &t,
		NotifyFriendRequest: &t,
		NotifySound:         &t,
		MuteAll:             &f,
		VolumeMaster:        &vol1,
		VolumeSFX:           &vol1,
		VolumeUI:            &vol1,
		VolumeNotifications: &vol1,
		VolumeMusic:         &vol1,
		ShowMoveHints:       &t,
		ConfirmMove:         &f,
		ShowTimerWarning:    &t,
		ShowOnlineStatus:    &t,
		AllowDMs:            &anyone,
	}
}

func (s *pgStore) GetPlayerSettings(ctx context.Context, playerID uuid.UUID) (PlayerSettings, error) {
	var raw []byte
	var updatedAt time.Time
	err := s.db.QueryRow(ctx,
		`SELECT settings, updated_at FROM player_settings WHERE player_id = $1`,
		playerID,
	).Scan(&raw, &updatedAt)

	defaults := defaultPlayerSettings()
	if err != nil {
		return PlayerSettings{
			PlayerID:  playerID,
			Settings:  defaults,
			UpdatedAt: time.Time{},
		}, nil
	}

	var stored PlayerSettingMap
	if err := json.Unmarshal(raw, &stored); err != nil {
		return PlayerSettings{}, fmt.Errorf("GetPlayerSettings: unmarshal: %w", err)
	}

	return PlayerSettings{
		PlayerID:  playerID,
		Settings:  mergeSettings(defaults, stored),
		UpdatedAt: updatedAt,
	}, nil
}

func (s *pgStore) UpsertPlayerSettings(ctx context.Context, playerID uuid.UUID, settings PlayerSettingMap) (PlayerSettings, error) {
	raw, err := json.Marshal(settings)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("UpsertPlayerSettings: marshal: %w", err)
	}

	var updatedAt time.Time
	err = s.db.QueryRow(ctx,
		`INSERT INTO player_settings (player_id, settings, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (player_id) DO UPDATE
		   SET settings   = EXCLUDED.settings,
		       updated_at = NOW()
		 RETURNING updated_at`,
		playerID, raw,
	).Scan(&updatedAt)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("UpsertPlayerSettings: %w", err)
	}

	return PlayerSettings{
		PlayerID:  playerID,
		Settings:  settings,
		UpdatedAt: updatedAt,
	}, nil
}

func mergeSettings(defaults, stored PlayerSettingMap) PlayerSettingMap {
	m := defaults
	if stored.Theme != nil {
		m.Theme = stored.Theme
	}
	if stored.Language != nil {
		m.Language = stored.Language
	}
	if stored.ReduceMotion != nil {
		m.ReduceMotion = stored.ReduceMotion
	}
	if stored.FontSize != nil {
		m.FontSize = stored.FontSize
	}
	if stored.NotifyDM != nil {
		m.NotifyDM = stored.NotifyDM
	}
	if stored.NotifyGameInvite != nil {
		m.NotifyGameInvite = stored.NotifyGameInvite
	}
	if stored.NotifyFriendRequest != nil {
		m.NotifyFriendRequest = stored.NotifyFriendRequest
	}
	if stored.NotifySound != nil {
		m.NotifySound = stored.NotifySound
	}
	if stored.MuteAll != nil {
		m.MuteAll = stored.MuteAll
	}
	if stored.VolumeMaster != nil {
		m.VolumeMaster = stored.VolumeMaster
	}
	if stored.VolumeSFX != nil {
		m.VolumeSFX = stored.VolumeSFX
	}
	if stored.VolumeUI != nil {
		m.VolumeUI = stored.VolumeUI
	}
	if stored.VolumeNotifications != nil {
		m.VolumeNotifications = stored.VolumeNotifications
	}
	if stored.VolumeMusic != nil {
		m.VolumeMusic = stored.VolumeMusic
	}
	if stored.ShowMoveHints != nil {
		m.ShowMoveHints = stored.ShowMoveHints
	}
	if stored.ConfirmMove != nil {
		m.ConfirmMove = stored.ConfirmMove
	}
	if stored.ShowTimerWarning != nil {
		m.ShowTimerWarning = stored.ShowTimerWarning
	}
	if stored.ShowOnlineStatus != nil {
		m.ShowOnlineStatus = stored.ShowOnlineStatus
	}
	if stored.AllowDMs != nil {
		m.AllowDMs = stored.AllowDMs
	}
	return m
}
