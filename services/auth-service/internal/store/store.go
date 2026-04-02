package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/auth-service/internal/handler"
)

// Store implements handler.Store using pgx.
// Only accesses: players, oauth_identities, allowed_emails.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and returns a Store.
func New(ctx context.Context, connStr string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// IsEmailAllowed checks whether the given email is in the allowed_emails table.
func (s *Store) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM allowed_emails
			WHERE email = $1
			AND (expires_at IS NULL OR expires_at > NOW())
		)`, email,
	).Scan(&exists)
	return exists, err
}

// UpsertOAuthIdentity creates or updates an OAuth identity and its linked player.
// On first login: creates the player, then the identity.
// On subsequent logins: updates avatar_url if changed.
func (s *Store) UpsertOAuthIdentity(ctx context.Context, params handler.UpsertOAuthParams) (handler.OAuthIdentity, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return handler.OAuthIdentity{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Try to find existing identity.
	var playerID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT player_id FROM oauth_identities
		 WHERE provider = $1 AND provider_id = $2`,
		params.Provider, params.ProviderID,
	).Scan(&playerID)

	if err != nil {
		// First login — create player then identity.
		var initialRole string
		err = tx.QueryRow(ctx,
			`SELECT role FROM allowed_emails WHERE email = $1`,
			params.Email,
		).Scan(&initialRole)
		if err != nil {
			return handler.OAuthIdentity{}, fmt.Errorf("get initial role: %w", err)
		}

		err = tx.QueryRow(ctx,
			`INSERT INTO players (username, role, avatar_url)
			 VALUES ($1, $2::player_role, $3)
			 ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
			 RETURNING id`,
			params.Username, initialRole, params.AvatarURL,
		).Scan(&playerID)
		if err != nil {
			return handler.OAuthIdentity{}, fmt.Errorf("insert player: %w", err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO oauth_identities (player_id, provider, provider_id, email, avatar_url)
			 VALUES ($1, $2, $3, $4, $5)`,
			playerID, params.Provider, params.ProviderID, params.Email, params.AvatarURL,
		)
		if err != nil {
			return handler.OAuthIdentity{}, fmt.Errorf("insert identity: %w", err)
		}
	} else {
		// Existing identity — update avatar if changed.
		_, err = tx.Exec(ctx,
			`UPDATE oauth_identities SET avatar_url = $1
			 WHERE provider = $2 AND provider_id = $3`,
			params.AvatarURL, params.Provider, params.ProviderID,
		)
		if err != nil {
			return handler.OAuthIdentity{}, fmt.Errorf("update identity: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return handler.OAuthIdentity{}, fmt.Errorf("commit: %w", err)
	}

	return handler.OAuthIdentity{PlayerID: playerID}, nil
}

// CreateSession inserts a new row into player_sessions with device info
// captured from the HTTP request.
func (s *Store) CreateSession(ctx context.Context, params handler.CreateSessionParams) error {
	deviceInfo := map[string]string{
		"user_agent":      params.UserAgent,
		"accept_language": params.AcceptLanguage,
		"ip_address":      params.IPAddress,
	}
	info, err := json.Marshal(deviceInfo)
	if err != nil {
		return fmt.Errorf("marshal device_info: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO player_sessions (player_id, device_info, expires_at)
		 VALUES ($1, $2, $3)`,
		params.PlayerID, info, params.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetPlayer fetches a player by ID.
func (s *Store) GetPlayer(ctx context.Context, id uuid.UUID) (handler.Player, error) {
	var p handler.Player
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, role FROM players WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&p.ID, &p.Username, &role)
	if err != nil {
		return p, fmt.Errorf("get player: %w", err)
	}
	p.Role = role
	return p, nil
}
