package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Player is a minimal player record for admin listing purposes.
type Player struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	Role      PlayerRole `json:"role"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	IsBot     bool       `json:"is_bot"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// PlayerRole identifies the access level of a player.
type PlayerRole string

const (
	RolePlayer  PlayerRole = "player"
	RoleManager PlayerRole = "manager"
	RoleOwner   PlayerRole = "owner"
)

// AllowedEmail is a whitelisted email entry for sign-up.
type AllowedEmail struct {
	Email     string     `json:"email"`
	Role      PlayerRole `json:"role"`
	Note      *string    `json:"note,omitempty"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// AddAllowedEmailParams holds the fields for adding an allowed email.
type AddAllowedEmailParams struct {
	Email     string
	Role      PlayerRole
	InvitedBy *uuid.UUID
}

func (s *pgStore) ListPlayers(ctx context.Context) ([]Player, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, username, avatar_url, role, is_bot, created_at, deleted_at
		 FROM players
		 WHERE deleted_at IS NULL
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPlayers: %w", err)
	}
	defer rows.Close()

	players := []Player{}
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.Username, &p.AvatarURL, &p.Role, &p.IsBot, &p.CreatedAt, &p.DeletedAt); err != nil {
			return nil, fmt.Errorf("ListPlayers scan: %w", err)
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// SetPlayerRole updates the role of a player.
// Returns an error if the target is an owner (owners can only be changed directly in the DB).
func (s *pgStore) SetPlayerRole(ctx context.Context, playerID uuid.UUID, role PlayerRole) error {
	var currentRole PlayerRole
	err := s.db.QueryRow(ctx,
		`SELECT role FROM players WHERE id = $1 AND deleted_at IS NULL`,
		playerID,
	).Scan(&currentRole)
	if err != nil {
		return fmt.Errorf("SetPlayerRole get current: %w", err)
	}
	if currentRole == RoleOwner && role != RoleOwner {
		return fmt.Errorf("SetPlayerRole: cannot demote an owner")
	}

	tag, err := s.db.Exec(ctx,
		`UPDATE players SET role = $1 WHERE id = $2 AND deleted_at IS NULL`,
		role, playerID,
	)
	if err != nil {
		return fmt.Errorf("SetPlayerRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SetPlayerRole: player not found")
	}
	return nil
}

func (s *pgStore) ListAllowedEmails(ctx context.Context) ([]AllowedEmail, error) {
	rows, err := s.db.Query(ctx,
		`SELECT email, role, note, invited_by, expires_at, created_at
		 FROM allowed_emails
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListAllowedEmails: %w", err)
	}
	defer rows.Close()

	entries := []AllowedEmail{}
	for rows.Next() {
		var e AllowedEmail
		if err := rows.Scan(&e.Email, &e.Role, &e.Note, &e.InvitedBy, &e.ExpiresAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListAllowedEmails scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *pgStore) AddAllowedEmail(ctx context.Context, params AddAllowedEmailParams) (AllowedEmail, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO allowed_emails (email, role, invited_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO UPDATE
		   SET role       = EXCLUDED.role,
		       invited_by = EXCLUDED.invited_by
		 RETURNING email, role, note, invited_by, expires_at, created_at`,
		params.Email, params.Role, params.InvitedBy,
	)
	var e AllowedEmail
	if err := row.Scan(&e.Email, &e.Role, &e.Note, &e.InvitedBy, &e.ExpiresAt, &e.CreatedAt); err != nil {
		return AllowedEmail{}, fmt.Errorf("AddAllowedEmail: %w", err)
	}
	return e, nil
}

func (s *pgStore) RemoveAllowedEmail(ctx context.Context, email string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM allowed_emails WHERE email = $1`,
		email,
	)
	if err != nil {
		return fmt.Errorf("RemoveAllowedEmail: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("RemoveAllowedEmail: email not found")
	}
	return nil
}
