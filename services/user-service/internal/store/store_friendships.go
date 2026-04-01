package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *pgStore) GetFriendship(ctx context.Context, playerA, playerB uuid.UUID) (Friendship, error) {
	row := s.db.QueryRow(ctx, `
		SELECT requester_id, addressee_id, status, note, created_at, updated_at
		FROM users.friendships
		WHERE (requester_id = $1 AND addressee_id = $2)
		   OR (requester_id = $2 AND addressee_id = $1)
		LIMIT 1
	`, playerA, playerB)

	var f Friendship
	err := row.Scan(&f.RequesterID, &f.AddresseeID, &f.Status, &f.Note, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// No relationship — return zero value with empty status.
		return Friendship{}, nil
	}
	return f, err
}

func (s *pgStore) ListFriends(ctx context.Context, playerID uuid.UUID) ([]FriendshipView, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END AS friend_id,
			p.username AS friend_username,
			p.avatar_url AS friend_avatar_url,
			f.status,
			f.created_at
		FROM users.friendships f
		JOIN public.players p ON p.id = CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
		WHERE (f.requester_id = $1 OR f.addressee_id = $1) AND f.status = 'accepted'
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFriendshipViews(rows)
}

func (s *pgStore) ListPendingFriendRequests(ctx context.Context, playerID uuid.UUID) ([]FriendshipView, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			f.requester_id AS friend_id,
			p.username AS friend_username,
			p.avatar_url AS friend_avatar_url,
			f.status,
			f.created_at
		FROM users.friendships f
		JOIN public.players p ON p.id = f.requester_id
		WHERE f.addressee_id = $1 AND f.status = 'pending'
		ORDER BY f.created_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFriendshipViews(rows)
}

func (s *pgStore) SendFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO users.friendships (requester_id, addressee_id, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (requester_id, addressee_id) DO NOTHING
		RETURNING requester_id, addressee_id, status, note, created_at, updated_at
	`, requesterID, addresseeID)

	var f Friendship
	err := row.Scan(&f.RequesterID, &f.AddresseeID, &f.Status, &f.Note, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (s *pgStore) AcceptFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE users.friendships
		SET status = 'accepted', updated_at = NOW()
		WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'
		RETURNING requester_id, addressee_id, status, note, created_at, updated_at
	`, requesterID, addresseeID)

	var f Friendship
	err := row.Scan(&f.RequesterID, &f.AddresseeID, &f.Status, &f.Note, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Friendship{}, ErrNotFound
	}
	return f, err
}

func (s *pgStore) DeclineFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM users.friendships
		WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'
	`, requesterID, addresseeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) BlockPlayer(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error) {
	// Upsert: if a friendship row exists in any direction, replace it with a block.
	row := s.db.QueryRow(ctx, `
		INSERT INTO users.friendships (requester_id, addressee_id, status)
		VALUES ($1, $2, 'blocked')
		ON CONFLICT (requester_id, addressee_id)
		DO UPDATE SET status = 'blocked', updated_at = NOW()
		RETURNING requester_id, addressee_id, status, note, created_at, updated_at
	`, requesterID, addresseeID)

	var f Friendship
	err := row.Scan(&f.RequesterID, &f.AddresseeID, &f.Status, &f.Note, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (s *pgStore) UnblockPlayer(ctx context.Context, requesterID, addresseeID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM users.friendships
		WHERE requester_id = $1 AND addressee_id = $2 AND status = 'blocked'
	`, requesterID, addresseeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) RemoveFriend(ctx context.Context, playerA, playerB uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM users.friendships
		WHERE (requester_id = $1 AND addressee_id = $2)
		   OR (requester_id = $2 AND addressee_id = $1)
		  AND status = 'accepted'
	`, playerA, playerB)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- helpers -----------------------------------------------------------------

func scanFriendships(rows pgx.Rows) ([]Friendship, error) {
	var out []Friendship
	for rows.Next() {
		var f Friendship
		if err := rows.Scan(&f.RequesterID, &f.AddresseeID, &f.Status, &f.Note, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func scanFriendshipViews(rows pgx.Rows) ([]FriendshipView, error) {
	var out []FriendshipView
	for rows.Next() {
		var fv FriendshipView
		if err := rows.Scan(&fv.FriendID, &fv.FriendUsername, &fv.FriendAvatarURL, &fv.Status, &fv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fv)
	}
	return out, rows.Err()
}
