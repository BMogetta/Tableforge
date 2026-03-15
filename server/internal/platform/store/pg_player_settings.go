package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *PGStore) GetPlayerSettings(ctx context.Context, playerID uuid.UUID) (PlayerSettings, error) {
	var raw []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT settings, updated_at FROM player_settings WHERE player_id = $1`,
		playerID,
	).Scan(&raw, &updatedAt)

	defaults := DefaultPlayerSettings()

	if err != nil {
		// No row — return defaults with zero updated_at.
		return PlayerSettings{
			PlayerID:  playerID,
			Settings:  defaults,
			UpdatedAt: time.Time{},
		}, nil
	}

	// Unmarshal stored values and merge over defaults.
	var stored PlayerSettingMap
	if err := json.Unmarshal(raw, &stored); err != nil {
		return PlayerSettings{}, fmt.Errorf("GetPlayerSettings: unmarshal: %w", err)
	}

	merged := mergeSettings(defaults, stored)
	return PlayerSettings{
		PlayerID:  playerID,
		Settings:  merged,
		UpdatedAt: updatedAt,
	}, nil
}

func (s *PGStore) UpsertPlayerSettings(ctx context.Context, playerID uuid.UUID, settings PlayerSettingMap) (PlayerSettings, error) {
	raw, err := json.Marshal(settings)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("UpsertPlayerSettings: marshal: %w", err)
	}

	var updatedAt time.Time
	err = s.pool.QueryRow(ctx,
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

// mergeSettings applies stored non-nil values over the defaults.
// Only pointer fields that are non-nil in stored override the default.
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
