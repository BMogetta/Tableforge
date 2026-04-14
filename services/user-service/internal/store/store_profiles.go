package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *pgStore) FindPlayerByUsername(ctx context.Context, username string) (Player, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, username, avatar_url, role, is_bot, created_at, deleted_at
		 FROM players
		 WHERE LOWER(username) = LOWER($1) AND deleted_at IS NULL AND is_bot = FALSE`,
		username,
	)
	var p Player
	if err := row.Scan(&p.ID, &p.Username, &p.AvatarURL, &p.Role, &p.IsBot, &p.CreatedAt, &p.DeletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Player{}, fmt.Errorf("player not found")
		}
		return Player{}, fmt.Errorf("FindPlayerByUsername: %w", err)
	}
	return p, nil
}

func (s *pgStore) GetProfile(ctx context.Context, playerID uuid.UUID) (PlayerProfile, error) {
	// LEFT JOIN on user_profiles so rows that never upserted a profile still
	// return identity fields (username, is_bot, bot_profile). This lets the
	// profile page render bots even though they will never fill in bio/country.
	row := s.db.QueryRow(ctx, `
		SELECT p.id, p.username, p.avatar_url, p.is_bot, p.bot_profile,
		       pp.bio, pp.country, COALESCE(pp.updated_at, p.created_at)
		FROM players p
		LEFT JOIN users.player_profiles pp ON pp.player_id = p.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`, playerID)

	var p PlayerProfile
	err := row.Scan(&p.PlayerID, &p.Username, &p.AvatarURL, &p.IsBot, &p.BotProfile, &p.Bio, &p.Country, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Caller navigated to a player that does not exist (or is soft-deleted).
		return PlayerProfile{}, fmt.Errorf("player not found")
	}
	return p, err
}

func (s *pgStore) UpsertProfile(ctx context.Context, params UpsertProfileParams) (PlayerProfile, error) {
	if _, err := s.db.Exec(ctx, `
		INSERT INTO users.player_profiles (player_id, bio, country)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_id) DO UPDATE
		SET bio        = EXCLUDED.bio,
		    country    = EXCLUDED.country,
		    updated_at = NOW()
	`, params.PlayerID, params.Bio, params.Country); err != nil {
		return PlayerProfile{}, fmt.Errorf("upsert profile: %w", err)
	}
	// Re-read via GetProfile so identity fields (username, avatar, is_bot,
	// bot_profile) come back in the same shape every caller expects.
	return s.GetProfile(ctx, params.PlayerID)
}

func (s *pgStore) UpsertAchievement(ctx context.Context, playerID uuid.UUID, key string, tier, progress int) (PlayerAchievement, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO users.player_achievements (player_id, achievement_key, tier, progress)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (player_id, achievement_key) DO UPDATE
		SET tier     = GREATEST(users.player_achievements.tier, EXCLUDED.tier),
		    progress = EXCLUDED.progress
		RETURNING id, player_id, achievement_key, tier, progress, unlocked_at
	`, playerID, key, tier, progress)

	var a PlayerAchievement
	err := row.Scan(&a.ID, &a.PlayerID, &a.AchievementKey, &a.Tier, &a.Progress, &a.UnlockedAt)
	return a, err
}

func (s *pgStore) ListAchievements(ctx context.Context, playerID uuid.UUID) ([]PlayerAchievement, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, player_id, achievement_key, tier, progress, unlocked_at
		FROM users.player_achievements
		WHERE player_id = $1
		ORDER BY unlocked_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PlayerAchievement
	for rows.Next() {
		var a PlayerAchievement
		if err := rows.Scan(&a.ID, &a.PlayerID, &a.AchievementKey, &a.Tier, &a.Progress, &a.UnlockedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
