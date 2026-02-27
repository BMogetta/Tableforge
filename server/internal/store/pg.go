package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the PostgreSQL implementation of Store.
type PGStore struct {
	pool *pgxpool.Pool
}

// New creates a new PGStore and verifies the connection.
func New(ctx context.Context, databaseURL string) (*PGStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &PGStore{pool: pool}, nil
}

// Close releases the connection pool.
func (s *PGStore) Close() {
	s.pool.Close()
}

// Exec runs a raw SQL statement. Used for applying migrations in tests.
func (s *PGStore) Exec(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}

// --- Players -----------------------------------------------------------------

func (s *PGStore) CreatePlayer(ctx context.Context, username string) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO players (username)
		 VALUES ($1)
		 RETURNING id, username, avatar_url, created_at, deleted_at`,
		username,
	)
	return scanPlayer(row)
}

func (s *PGStore) GetPlayer(ctx context.Context, id uuid.UUID) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, avatar_url, created_at, deleted_at
		 FROM players WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanPlayer(row)
}

func (s *PGStore) GetPlayerByUsername(ctx context.Context, username string) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, avatar_url, created_at, deleted_at
		 FROM players WHERE username = $1 AND deleted_at IS NULL`,
		username,
	)
	return scanPlayer(row)
}

func (s *PGStore) UpdatePlayerAvatar(ctx context.Context, id uuid.UUID, avatarURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE players SET avatar_url = $1 WHERE id = $2`,
		avatarURL, id,
	)
	return fmt.Errorf("UpdatePlayerAvatar: %w", err)
}

func (s *PGStore) SoftDeletePlayer(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE players SET deleted_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("SoftDeletePlayer: %w", err)
	}
	return nil
}

// --- Rooms -------------------------------------------------------------------

func (s *PGStore) CreateRoom(ctx context.Context, params CreateRoomParams) (Room, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO rooms (code, game_id, owner_id, max_players, turn_timeout_secs, allow_spectators)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, code, game_id, owner_id, status, max_players,
		           turn_timeout_secs, allow_spectators, created_at, updated_at, deleted_at`,
		params.Code, params.GameID, params.OwnerID, params.MaxPlayers,
		params.TurnTimeoutSecs, params.AllowSpectators,
	)
	return scanRoom(row)
}

func (s *PGStore) GetRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, allow_spectators, created_at, updated_at, deleted_at
		 FROM rooms WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanRoom(row)
}

func (s *PGStore) GetRoomByCode(ctx context.Context, code string) (Room, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, allow_spectators, created_at, updated_at, deleted_at
		 FROM rooms WHERE code = $1 AND deleted_at IS NULL`,
		code,
	)
	return scanRoom(row)
}

func (s *PGStore) UpdateRoomStatus(ctx context.Context, id uuid.UUID, status RoomStatus) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE rooms SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateRoomStatus: %w", err)
	}
	return nil
}

func (s *PGStore) ListWaitingRooms(ctx context.Context) ([]Room, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, allow_spectators, created_at, updated_at, deleted_at
		 FROM rooms WHERE status = 'waiting' AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListWaitingRooms: %w", err)
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		r, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (s *PGStore) SoftDeleteRoom(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE rooms SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("SoftDeleteRoom: %w", err)
	}
	return nil
}

// --- Room players ------------------------------------------------------------

func (s *PGStore) AddPlayerToRoom(ctx context.Context, roomID, playerID uuid.UUID, seat int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO room_players (room_id, player_id, seat) VALUES ($1, $2, $3)`,
		roomID, playerID, seat,
	)
	if err != nil {
		return fmt.Errorf("AddPlayerToRoom: %w", err)
	}
	return nil
}

func (s *PGStore) RemovePlayerFromRoom(ctx context.Context, roomID, playerID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM room_players WHERE room_id = $1 AND player_id = $2`,
		roomID, playerID,
	)
	if err != nil {
		return fmt.Errorf("RemovePlayerFromRoom: %w", err)
	}
	return nil
}

func (s *PGStore) ListRoomPlayers(ctx context.Context, roomID uuid.UUID) ([]RoomPlayer, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT room_id, player_id, seat, joined_at
		 FROM room_players WHERE room_id = $1 ORDER BY seat`,
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListRoomPlayers: %w", err)
	}
	defer rows.Close()

	var players []RoomPlayer
	for rows.Next() {
		var rp RoomPlayer
		if err := rows.Scan(&rp.RoomID, &rp.PlayerID, &rp.Seat, &rp.JoinedAt); err != nil {
			return nil, fmt.Errorf("ListRoomPlayers scan: %w", err)
		}
		players = append(players, rp)
	}
	return players, rows.Err()
}

// --- Game sessions -----------------------------------------------------------

func (s *PGStore) CreateGameSession(ctx context.Context, roomID uuid.UUID, gameID string, initialState []byte) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO game_sessions (room_id, game_id, state)
		 VALUES ($1, $2, $3)
		 RETURNING id, room_id, game_id, name, state, move_count,
		           suspend_count, suspended_at, suspended_reason,
		           started_at, finished_at, deleted_at`,
		roomID, gameID, initialState,
	)
	return scanSession(row)
}

func (s *PGStore) GetGameSession(ctx context.Context, id uuid.UUID) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, room_id, game_id, name, state, move_count,
		        suspend_count, suspended_at, suspended_reason,
		        started_at, finished_at, deleted_at
		 FROM game_sessions WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanSession(row)
}

func (s *PGStore) GetActiveSessionByRoom(ctx context.Context, roomID uuid.UUID) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, room_id, game_id, name, state, move_count,
		        suspend_count, suspended_at, suspended_reason,
		        started_at, finished_at, deleted_at
		 FROM game_sessions
		 WHERE room_id = $1 AND finished_at IS NULL AND deleted_at IS NULL
		 ORDER BY started_at DESC LIMIT 1`,
		roomID,
	)
	return scanSession(row)
}

func (s *PGStore) UpdateSessionState(ctx context.Context, id uuid.UUID, state []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET state = $1, move_count = move_count + 1 WHERE id = $2`,
		state, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateSessionState: %w", err)
	}
	return nil
}

func (s *PGStore) FinishSession(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET finished_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("FinishSession: %w", err)
	}
	return nil
}

func (s *PGStore) SuspendSession(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET suspended_at = NOW(),
		     suspended_reason = $1,
		     suspend_count = suspend_count + 1
		 WHERE id = $2`,
		reason, id,
	)
	if err != nil {
		return fmt.Errorf("SuspendSession: %w", err)
	}
	return nil
}

func (s *PGStore) ResumeSession(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET suspended_at = NULL, suspended_reason = NULL
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("ResumeSession: %w", err)
	}
	return nil
}

func (s *PGStore) ListActiveSessions(ctx context.Context, playerID uuid.UUID) ([]GameSession, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT gs.id, gs.room_id, gs.game_id, gs.name, gs.state, gs.move_count,
		        gs.suspend_count, gs.suspended_at, gs.suspended_reason,
		        gs.started_at, gs.finished_at, gs.deleted_at
		 FROM game_sessions gs
		 JOIN room_players rp ON rp.room_id = gs.room_id
		 WHERE rp.player_id = $1
		   AND gs.finished_at IS NULL
		   AND gs.deleted_at IS NULL
		 ORDER BY gs.started_at DESC`,
		playerID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListActiveSessions: %w", err)
	}
	defer rows.Close()

	sessions := []GameSession{}
	for rows.Next() {
		gs, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, gs)
	}
	return sessions, rows.Err()
}

func (s *PGStore) SoftDeleteSession(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET deleted_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("SoftDeleteSession: %w", err)
	}
	return nil
}

// --- Moves -------------------------------------------------------------------

func (s *PGStore) RecordMove(ctx context.Context, params RecordMoveParams) (Move, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO moves (session_id, player_id, payload, state_after, move_number)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, session_id, player_id, payload, state_after, move_number, applied_at`,
		params.SessionID, params.PlayerID, params.Payload, params.StateAfter, params.MoveNumber,
	)
	var m Move
	if err := row.Scan(&m.ID, &m.SessionID, &m.PlayerID, &m.Payload, &m.StateAfter, &m.MoveNumber, &m.AppliedAt); err != nil {
		return Move{}, fmt.Errorf("RecordMove: %w", err)
	}
	return m, nil
}

func (s *PGStore) ListSessionMoves(ctx context.Context, sessionID uuid.UUID) ([]Move, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, player_id, payload, state_after, move_number, applied_at
		 FROM moves WHERE session_id = $1 ORDER BY move_number`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListSessionMoves: %w", err)
	}
	defer rows.Close()

	moves := []Move{}
	for rows.Next() {
		var m Move
		if err := rows.Scan(&m.ID, &m.SessionID, &m.PlayerID, &m.Payload, &m.StateAfter, &m.MoveNumber, &m.AppliedAt); err != nil {
			return nil, fmt.Errorf("ListSessionMoves scan: %w", err)
		}
		moves = append(moves, m)
	}
	return moves, rows.Err()
}

func (s *PGStore) GetMoveAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (Move, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, session_id, player_id, payload, state_after, move_number, applied_at
		 FROM moves WHERE session_id = $1 AND move_number = $2`,
		sessionID, moveNumber,
	)
	var m Move
	if err := row.Scan(&m.ID, &m.SessionID, &m.PlayerID, &m.Payload, &m.StateAfter, &m.MoveNumber, &m.AppliedAt); err != nil {
		return Move{}, fmt.Errorf("GetMoveAt: %w", err)
	}
	return m, nil
}

// --- OAuth -------------------------------------------------------------------

func (s *PGStore) UpsertOAuthIdentity(ctx context.Context, params UpsertOAuthParams) (OAuthIdentity, error) {
	// Upsert player first
	row := s.pool.QueryRow(ctx,
		`INSERT INTO players (username, avatar_url)
		 VALUES ($1, $2)
		 ON CONFLICT (username) DO UPDATE
		   SET avatar_url = EXCLUDED.avatar_url
		 RETURNING id, username, avatar_url, created_at, deleted_at`,
		params.Username, params.AvatarURL,
	)
	player, err := scanPlayer(row)
	if err != nil {
		return OAuthIdentity{}, fmt.Errorf("UpsertOAuthIdentity upsert player: %w", err)
	}

	// Upsert identity
	row = s.pool.QueryRow(ctx,
		`INSERT INTO oauth_identities (player_id, provider, provider_id, email, avatar_url)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (provider, provider_id) DO UPDATE
		   SET email      = EXCLUDED.email,
		       avatar_url = EXCLUDED.avatar_url
		 RETURNING id, player_id, provider, provider_id, email, avatar_url, created_at`,
		player.ID, params.Provider, params.ProviderID, params.Email, params.AvatarURL,
	)
	var oi OAuthIdentity
	if err := row.Scan(&oi.ID, &oi.PlayerID, &oi.Provider, &oi.ProviderID, &oi.Email, &oi.AvatarURL, &oi.CreatedAt); err != nil {
		return OAuthIdentity{}, fmt.Errorf("UpsertOAuthIdentity upsert identity: %w", err)
	}
	return oi, nil
}

func (s *PGStore) GetOAuthIdentity(ctx context.Context, provider, providerID string) (OAuthIdentity, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, player_id, provider, provider_id, email, avatar_url, created_at
		 FROM oauth_identities WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	)
	var oi OAuthIdentity
	if err := row.Scan(&oi.ID, &oi.PlayerID, &oi.Provider, &oi.ProviderID, &oi.Email, &oi.AvatarURL, &oi.CreatedAt); err != nil {
		return OAuthIdentity{}, fmt.Errorf("GetOAuthIdentity: %w", err)
	}
	return oi, nil
}

func (s *PGStore) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM allowed_emails
		 WHERE email = $1
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		email,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("IsEmailAllowed: %w", err)
	}
	return count > 0, nil
}

// --- Results & leaderboard ---------------------------------------------------

func (s *PGStore) CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GameResult{}, fmt.Errorf("CreateGameResult begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Calculate duration
	var durationSecs *int
	row := tx.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (NOW() - started_at))::INT FROM game_sessions WHERE id = $1`,
		params.SessionID,
	)
	var d int
	if err := row.Scan(&d); err == nil {
		durationSecs = &d
	}

	// Insert result
	var gr GameResult
	row = tx.QueryRow(ctx,
		`INSERT INTO game_results (session_id, game_id, winner_id, is_draw, ended_by, duration_secs)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, session_id, game_id, winner_id, is_draw, ended_by, duration_secs, created_at`,
		params.SessionID, params.GameID, params.WinnerID, params.IsDraw, params.EndedBy, durationSecs,
	)
	if err := row.Scan(&gr.ID, &gr.SessionID, &gr.GameID, &gr.WinnerID, &gr.IsDraw, &gr.EndedBy, &gr.DurationSecs, &gr.CreatedAt); err != nil {
		return GameResult{}, fmt.Errorf("CreateGameResult insert result: %w", err)
	}

	// Insert per-player outcomes
	for _, p := range params.Players {
		_, err := tx.Exec(ctx,
			`INSERT INTO game_result_players (result_id, player_id, seat, outcome)
			 VALUES ($1, $2, $3, $4)`,
			gr.ID, p.PlayerID, p.Seat, p.Outcome,
		)
		if err != nil {
			return GameResult{}, fmt.Errorf("CreateGameResult insert player outcome: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return GameResult{}, fmt.Errorf("CreateGameResult commit: %w", err)
	}
	return gr, nil
}

func (s *PGStore) GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT
		     COUNT(*)                                          AS total,
		     COUNT(*) FILTER (WHERE outcome = 'win')          AS wins,
		     COUNT(*) FILTER (WHERE outcome = 'loss')         AS losses,
		     COUNT(*) FILTER (WHERE outcome = 'draw')         AS draws,
		     COUNT(*) FILTER (WHERE outcome = 'forfeit')      AS forfeits
		 FROM game_result_players
		 WHERE player_id = $1`,
		playerID,
	)
	var ps PlayerStats
	ps.PlayerID = playerID
	if err := row.Scan(&ps.TotalGames, &ps.Wins, &ps.Losses, &ps.Draws, &ps.Forfeits); err != nil {
		return PlayerStats{}, fmt.Errorf("GetPlayerStats: %w", err)
	}
	return ps, nil
}

func (s *PGStore) GetLeaderboard(ctx context.Context, gameID string, limit int) ([]LeaderboardEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.username, p.avatar_url,
		        COUNT(*) FILTER (WHERE grp.outcome = 'win')   AS wins,
		        COUNT(*) FILTER (WHERE grp.outcome = 'loss')  AS losses,
		        COUNT(*) FILTER (WHERE grp.outcome = 'draw')  AS draws
		 FROM game_result_players grp
		 JOIN game_results gr ON gr.id = grp.result_id
		 JOIN players p ON p.id = grp.player_id
		 WHERE ($1 = '' OR gr.game_id = $1)
		   AND p.deleted_at IS NULL
		 GROUP BY p.id, p.username, p.avatar_url
		 ORDER BY wins DESC, draws DESC
		 LIMIT $2`,
		gameID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("GetLeaderboard: %w", err)
	}
	defer rows.Close()

	entries := []LeaderboardEntry{}
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.PlayerID, &e.Username, &e.AvatarURL, &e.Wins, &e.Losses, &e.Draws); err != nil {
			return nil, fmt.Errorf("GetLeaderboard scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *PGStore) ListPlayerHistory(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]GameResult, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT gr.id, gr.session_id, gr.game_id, gr.winner_id, gr.is_draw,
		        gr.ended_by, gr.duration_secs, gr.created_at
		 FROM game_results gr
		 JOIN game_result_players grp ON grp.result_id = gr.id
		 WHERE grp.player_id = $1
		 ORDER BY gr.created_at DESC
		 LIMIT $2 OFFSET $3`,
		playerID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPlayerHistory: %w", err)
	}
	defer rows.Close()

	results := []GameResult{}
	for rows.Next() {
		var gr GameResult
		if err := rows.Scan(&gr.ID, &gr.SessionID, &gr.GameID, &gr.WinnerID, &gr.IsDraw, &gr.EndedBy, &gr.DurationSecs, &gr.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListPlayerHistory scan: %w", err)
		}
		results = append(results, gr)
	}
	return results, rows.Err()
}

// --- Spectators --------------------------------------------------------------

func (s *PGStore) CreateSpectatorLink(ctx context.Context, roomID, createdBy uuid.UUID) (SpectatorLink, error) {
	token, err := generateToken(32)
	if err != nil {
		return SpectatorLink{}, fmt.Errorf("CreateSpectatorLink generate token: %w", err)
	}

	row := s.pool.QueryRow(ctx,
		`INSERT INTO spectator_links (token, room_id, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING token, room_id, created_by, created_at`,
		token, roomID, createdBy,
	)
	var sl SpectatorLink
	if err := row.Scan(&sl.Token, &sl.RoomID, &sl.CreatedBy, &sl.CreatedAt); err != nil {
		return SpectatorLink{}, fmt.Errorf("CreateSpectatorLink: %w", err)
	}
	return sl, nil
}

func (s *PGStore) GetSpectatorLink(ctx context.Context, token string) (SpectatorLink, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT sl.token, sl.room_id, sl.created_by, sl.created_at
		 FROM spectator_links sl
		 JOIN game_sessions gs ON gs.room_id = sl.room_id
		 WHERE sl.token = $1 AND gs.finished_at IS NULL
		 LIMIT 1`,
		token,
	)
	var sl SpectatorLink
	if err := row.Scan(&sl.Token, &sl.RoomID, &sl.CreatedBy, &sl.CreatedAt); err != nil {
		return SpectatorLink{}, fmt.Errorf("GetSpectatorLink: %w", err)
	}
	return sl, nil
}

// --- Rematch -----------------------------------------------------------------

func (s *PGStore) UpsertRematchVote(ctx context.Context, sessionID, playerID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO rematch_votes (session_id, player_id)
		 VALUES ($1, $2)
		 ON CONFLICT (session_id, player_id) DO NOTHING`,
		sessionID, playerID,
	)
	if err != nil {
		return fmt.Errorf("UpsertRematchVote: %w", err)
	}
	return nil
}

func (s *PGStore) ListRematchVotes(ctx context.Context, sessionID uuid.UUID) ([]RematchVote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT session_id, player_id, voted_at FROM rematch_votes WHERE session_id = $1`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListRematchVotes: %w", err)
	}
	defer rows.Close()

	votes := []RematchVote{}
	for rows.Next() {
		var v RematchVote
		if err := rows.Scan(&v.SessionID, &v.PlayerID, &v.VotedAt); err != nil {
			return nil, fmt.Errorf("ListRematchVotes scan: %w", err)
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}

func (s *PGStore) DeleteRematchVotes(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM rematch_votes WHERE session_id = $1`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("DeleteRematchVotes: %w", err)
	}
	return nil
}

// --- Helpers -----------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanPlayer(row scanner) (Player, error) {
	var p Player
	if err := row.Scan(&p.ID, &p.Username, &p.AvatarURL, &p.CreatedAt, &p.DeletedAt); err != nil {
		return Player{}, fmt.Errorf("scanPlayer: %w", err)
	}
	return p, nil
}

func scanRoom(row scanner) (Room, error) {
	var r Room
	if err := row.Scan(
		&r.ID, &r.Code, &r.GameID, &r.OwnerID, &r.Status, &r.MaxPlayers,
		&r.TurnTimeoutSecs, &r.AllowSpectators, &r.CreatedAt, &r.UpdatedAt, &r.DeletedAt,
	); err != nil {
		return Room{}, fmt.Errorf("scanRoom: %w", err)
	}
	return r, nil
}

func scanSession(row scanner) (GameSession, error) {
	var gs GameSession
	if err := row.Scan(
		&gs.ID, &gs.RoomID, &gs.GameID, &gs.Name, &gs.State, &gs.MoveCount,
		&gs.SuspendCount, &gs.SuspendedAt, &gs.SuspendedReason,
		&gs.StartedAt, &gs.FinishedAt, &gs.DeletedAt,
	); err != nil {
		return GameSession{}, fmt.Errorf("scanSession: %w", err)
	}
	return gs, nil
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
