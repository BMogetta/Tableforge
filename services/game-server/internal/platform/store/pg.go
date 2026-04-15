package store

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the PostgreSQL implementation of Store.
type PGStore struct {
	pool *pgxpool.Pool
}

// New creates a new PGStore and verifies the connection.
func New(ctx context.Context, databaseURL string) (*PGStore, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, config)
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
         RETURNING id, username, avatar_url, role, is_bot, bot_profile, created_at, deleted_at`,
		username,
	)
	return scanPlayer(row)
}

func (s *PGStore) GetPlayer(ctx context.Context, id uuid.UUID) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, avatar_url, role, is_bot, bot_profile, created_at, deleted_at
		 FROM players WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanPlayer(row)
}

func (s *PGStore) GetPlayerByUsername(ctx context.Context, username string) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, username, avatar_url, role, is_bot, bot_profile, created_at, deleted_at
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

func (s *PGStore) CreateBotPlayer(ctx context.Context, username string) (Player, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO players (username, is_bot)
         VALUES ($1, TRUE)
         RETURNING id, username, avatar_url, role, is_bot, bot_profile, created_at, deleted_at`,
		username,
	)
	return scanPlayer(row)
}

func (s *PGStore) UpdateRoomOwner(ctx context.Context, roomID, newOwnerID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE rooms SET owner_id = $1, updated_at = NOW() WHERE id = $2`,
		newOwnerID, roomID,
	)
	if err != nil {
		return fmt.Errorf("UpdateRoomOwner: %w", err)
	}
	return nil
}

func (s *PGStore) DeleteRoom(ctx context.Context, roomID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM room_players WHERE room_id = $1`,
		roomID,
	)
	if err != nil {
		return fmt.Errorf("DeleteRoom: remove players: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE rooms SET status = 'finished', updated_at = NOW() WHERE id = $1`,
		roomID,
	)
	if err != nil {
		return fmt.Errorf("DeleteRoom: close room: %w", err)
	}
	return nil
}

// --- Rooms -------------------------------------------------------------------

// CreateRoom inserts a new room and its default settings in a single transaction.
func (s *PGStore) CreateRoom(ctx context.Context, params CreateRoomParams) (Room, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Room{}, fmt.Errorf("CreateRoom: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx,
		`INSERT INTO rooms (code, game_id, owner_id, max_players, turn_timeout_secs)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, code, game_id, owner_id, status, max_players,
		           turn_timeout_secs, created_at, updated_at, deleted_at`,
		params.Code, params.GameID, params.OwnerID, params.MaxPlayers,
		params.TurnTimeoutSecs,
	)
	room, err := scanRoom(row)
	if err != nil {
		return Room{}, fmt.Errorf("CreateRoom: insert room: %w", err)
	}

	for key, value := range params.DefaultSettings {
		_, err := tx.Exec(ctx,
			`INSERT INTO room_settings (room_id, key, value)
			 VALUES ($1, $2, $3)`,
			room.ID, key, value,
		)
		if err != nil {
			return Room{}, fmt.Errorf("CreateRoom: insert setting %q: %w", key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Room{}, fmt.Errorf("CreateRoom: commit: %w", err)
	}

	return room, nil
}

func (s *PGStore) GetRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, created_at, updated_at, deleted_at
		 FROM rooms WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanRoom(row)
}

func (s *PGStore) GetRoomByCode(ctx context.Context, code string) (Room, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, created_at, updated_at, deleted_at
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

func (s *PGStore) ListWaitingRooms(ctx context.Context, limit, offset int) ([]Room, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, code, game_id, owner_id, status, max_players,
		        turn_timeout_secs, created_at, updated_at, deleted_at
		 FROM rooms WHERE status = 'waiting' AND deleted_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ListWaitingRooms: %w", err)
	}
	defer rows.Close()

	rooms := []Room{}
	for rows.Next() {
		r, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (s *PGStore) CountWaitingRooms(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rooms WHERE status = 'waiting' AND deleted_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountWaitingRooms: %w", err)
	}
	return count, nil
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

// --- Room settings -----------------------------------------------------------

func (s *PGStore) GetRoomSettings(ctx context.Context, roomID uuid.UUID) (map[string]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT key, value FROM room_settings WHERE room_id = $1`,
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetRoomSettings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("GetRoomSettings scan: %w", err)
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

func (s *PGStore) SetRoomSetting(ctx context.Context, roomID uuid.UUID, key, value string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO room_settings (room_id, key, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (room_id, key) DO UPDATE
		   SET value      = EXCLUDED.value,
		       updated_at = NOW()`,
		roomID, key, value,
	)
	if err != nil {
		return fmt.Errorf("SetRoomSetting: %w", err)
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

func (s *PGStore) IsPlayerInActiveRoom(ctx context.Context, playerID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM room_players rp
			JOIN rooms r ON r.id = rp.room_id
			WHERE rp.player_id = $1
			AND r.status IN ('waiting', 'in_progress')
			AND r.deleted_at IS NULL
		)`, playerID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("IsPlayerInActiveRoom: %w", err)
	}
	return exists, nil
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

	players := []RoomPlayer{}
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

func (s *PGStore) CreateGameSession(ctx context.Context, roomID uuid.UUID, gameID string, initialState []byte, turnTimeoutSecs *int, mode SessionMode) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO game_sessions (room_id, game_id, state, turn_timeout_secs, mode)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id, room_id, game_id, name, state, mode, move_count, suspend_count,
                   suspended_at, suspended_reason, ready_players,
                   turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at`,
		roomID, gameID, initialState, turnTimeoutSecs, mode,
	)
	return scanSession(row)
}

func (s *PGStore) GetGameSession(ctx context.Context, id uuid.UUID) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, room_id, game_id, name, state, mode, move_count,
		        suspend_count, suspended_at, suspended_reason, ready_players,
		        turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
		 FROM game_sessions WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return scanSession(row)
}

func (s *PGStore) GetGameResult(ctx context.Context, sessionID uuid.UUID) (GameResult, error) {
	var r GameResult
	err := s.pool.QueryRow(ctx,
		`SELECT id, session_id, game_id, winner_id, is_draw, ended_by, created_at
         FROM game_results WHERE session_id = $1`,
		sessionID,
	).Scan(&r.ID, &r.SessionID, &r.GameID, &r.WinnerID, &r.IsDraw, &r.EndedBy, &r.CreatedAt)
	if err != nil {
		return GameResult{}, err
	}
	return r, nil
}

func (s *PGStore) GetActiveSessionByRoom(ctx context.Context, roomID uuid.UUID) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, room_id, game_id, name, state, mode, move_count,
		        suspend_count, suspended_at, suspended_reason, ready_players,
		        turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
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
	// After resume, players get 40% of the turn timeout (pause is not free time).
	// Set last_move_at = NOW() - 60% * turn_timeout_secs so the frontend
	// calculates remaining = timeout - elapsed = timeout - 60% = 40%.
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions
		 SET last_move_at = NOW() - (COALESCE(turn_timeout_secs, 30) * 0.6) * INTERVAL '1 second',
		     suspended_at = NULL,
		     suspended_reason = NULL
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
		`SELECT gs.id, gs.room_id, gs.game_id, gs.name, gs.state, gs.mode, gs.move_count,
		        gs.suspend_count, gs.suspended_at, gs.suspended_reason,
		        gs.ready_players,
		        gs.turn_timeout_secs, gs.last_move_at,
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

// GetGameConfig loads the configuration for a game from game_configs.
// Returns a default config if the game has no entry.
func (s *PGStore) GetGameConfig(ctx context.Context, gameID string) (GameConfig, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT game_id, default_timeout_secs, min_timeout_secs, max_timeout_secs, timeout_penalty
		 FROM game_configs WHERE game_id = $1`,
		gameID,
	)
	var c GameConfig
	if err := row.Scan(&c.GameID, &c.DefaultTimeoutSecs, &c.MinTimeoutSecs, &c.MaxTimeoutSecs, &c.TimeoutPenalty); err != nil {
		return GameConfig{
			GameID:             gameID,
			DefaultTimeoutSecs: 60,
			MinTimeoutSecs:     30,
			MaxTimeoutSecs:     600,
			TimeoutPenalty:     PenaltyLoseTurn,
		}, nil
	}
	return c, nil
}

// TouchLastMoveAt updates last_move_at to now for the given session.
func (s *PGStore) TouchLastMoveAt(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE game_sessions SET last_move_at = NOW() WHERE id = $1`,
		sessionID,
	)
	return err
}

// CountFinishedSessions returns the number of completed sessions for a room.
func (s *PGStore) CountFinishedSessions(ctx context.Context, roomID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_sessions
		 WHERE room_id = $1 AND finished_at IS NOT NULL AND deleted_at IS NULL`,
		roomID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountFinishedSessions: %w", err)
	}
	return count, nil
}

// GetLastFinishedSession returns the most recently finished session for a room.
func (s *PGStore) GetLastFinishedSession(ctx context.Context, roomID uuid.UUID) (GameSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, room_id, game_id, name, state, mode, move_count,
		        suspend_count, suspended_at, suspended_reason, ready_players,
		        turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
		 FROM game_sessions
		 WHERE room_id = $1 AND finished_at IS NOT NULL AND deleted_at IS NULL
		 ORDER BY finished_at DESC
		 LIMIT 1`,
		roomID,
	)
	return scanSession(row)
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

func (s *PGStore) ListSessionMoves(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]Move, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, player_id, payload, state_after, move_number, applied_at
		 FROM moves WHERE session_id = $1 ORDER BY move_number
		 LIMIT $2 OFFSET $3`,
		sessionID, limit, offset,
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

// --- Results -----------------------------------------------------------------

func (s *PGStore) CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GameResult{}, fmt.Errorf("CreateGameResult begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var durationSecs *int
	row := tx.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (NOW() - started_at))::INT FROM game_sessions WHERE id = $1`,
		params.SessionID,
	)
	var d int
	if err := row.Scan(&d); err == nil {
		durationSecs = &d
	}

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

func (s *PGStore) ListPlayerMatches(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]MatchHistoryEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT gr.id, gr.session_id, gr.game_id, grp.outcome,
		        gr.ended_by, gr.duration_secs, gr.created_at,
		        op.id, op.username, op.is_bot, op.bot_profile
		 FROM game_results gr
		 JOIN game_result_players grp ON grp.result_id = gr.id
		 LEFT JOIN LATERAL (
		   SELECT p.id, p.username, p.is_bot, p.bot_profile
		   FROM game_result_players g2
		   JOIN players p ON p.id = g2.player_id
		   WHERE g2.result_id = gr.id AND g2.player_id <> $1
		   ORDER BY g2.seat
		   LIMIT 1
		 ) op ON TRUE
		 WHERE grp.player_id = $1
		 ORDER BY gr.created_at DESC
		 LIMIT $2 OFFSET $3`,
		playerID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ListPlayerMatches: %w", err)
	}
	defer rows.Close()

	entries := []MatchHistoryEntry{}
	for rows.Next() {
		var e MatchHistoryEntry
		if err := rows.Scan(
			&e.ID, &e.SessionID, &e.GameID, &e.Outcome, &e.EndedBy, &e.DurationSecs, &e.CreatedAt,
			&e.OpponentID, &e.OpponentUsername, &e.OpponentIsBot, &e.OpponentBotProfile,
		); err != nil {
			return nil, fmt.Errorf("ListPlayerMatches scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *PGStore) CountPlayerMatches(ctx context.Context, playerID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_result_players WHERE player_id = $1`,
		playerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountPlayerMatches: %w", err)
	}
	return count, nil
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

// --- Admin stats -------------------------------------------------------------

func (s *PGStore) CountActiveRooms(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rooms
		 WHERE status IN ('waiting', 'in_progress') AND deleted_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountActiveRooms: %w", err)
	}
	return count, nil
}

func (s *PGStore) CountActiveSessions(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_sessions
		 WHERE finished_at IS NULL AND deleted_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountActiveSessions: %w", err)
	}
	return count, nil
}

func (s *PGStore) CountTotalPlayers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM players WHERE deleted_at IS NULL AND is_bot = FALSE`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountTotalPlayers: %w", err)
	}
	return count, nil
}

func (s *PGStore) CountSessionsToday(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_sessions
		 WHERE started_at >= CURRENT_DATE AND deleted_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountSessionsToday: %w", err)
	}
	return count, nil
}

// --- Cleanup -----------------------------------------------------------------

// CleanupOrphanRooms hard-deletes waiting rooms with 0 players that haven't
// been updated in waitingMaxAge, and soft-deletes finished rooms older than
// finishedMaxAge. Returns total rows affected.
func (s *PGStore) CleanupOrphanRooms(ctx context.Context, waitingMaxAge, finishedMaxAge time.Duration) (int, error) {
	// 1. Hard-delete empty waiting rooms older than threshold.
	res1, err := s.pool.Exec(ctx,
		`DELETE FROM rooms
		 WHERE status = 'waiting'
		   AND deleted_at IS NULL
		   AND updated_at < NOW() - $1::interval
		   AND id NOT IN (SELECT DISTINCT room_id FROM room_players)`,
		waitingMaxAge,
	)
	if err != nil {
		return 0, fmt.Errorf("CleanupOrphanRooms (waiting): %w", err)
	}

	// 2. Soft-delete finished rooms older than threshold.
	res2, err := s.pool.Exec(ctx,
		`UPDATE rooms SET deleted_at = NOW()
		 WHERE status = 'finished'
		   AND deleted_at IS NULL
		   AND updated_at < NOW() - $1::interval`,
		finishedMaxAge,
	)
	if err != nil {
		return int(res1.RowsAffected()), fmt.Errorf("CleanupOrphanRooms (finished): %w", err)
	}

	return int(res1.RowsAffected() + res2.RowsAffected()), nil
}

// --- Helpers -----------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanPlayer(row scanner) (Player, error) {
	var p Player
	if err := row.Scan(&p.ID, &p.Username, &p.AvatarURL, &p.Role, &p.IsBot, &p.BotProfile, &p.CreatedAt, &p.DeletedAt); err != nil {
		return Player{}, fmt.Errorf("scanPlayer: %w", err)
	}
	return p, nil
}

func scanRoom(row scanner) (Room, error) {
	var r Room
	if err := row.Scan(
		&r.ID, &r.Code, &r.GameID, &r.OwnerID, &r.Status, &r.MaxPlayers,
		&r.TurnTimeoutSecs, &r.CreatedAt, &r.UpdatedAt, &r.DeletedAt,
	); err != nil {
		return Room{}, fmt.Errorf("scanRoom: %w", err)
	}
	return r, nil
}

// scanSession scans all columns of game_sessions into a GameSession.
// All queries against game_sessions must select columns in this exact order:
//
//	id, room_id, game_id, name, state, mode, move_count,
//	suspend_count, suspended_at, suspended_reason,   ready_players,
//	turn_timeout_secs, last_move_at, started_at, finished_at, deleted_at
func scanSession(row scanner) (GameSession, error) {
	var gs GameSession
	if err := row.Scan(
		&gs.ID, &gs.RoomID, &gs.GameID, &gs.Name, &gs.State, &gs.Mode,
		&gs.MoveCount, &gs.SuspendCount, &gs.SuspendedAt, &gs.SuspendedReason,
		&gs.ReadyPlayers,
		&gs.TurnTimeoutSecs, &gs.LastMoveAt, &gs.StartedAt, &gs.FinishedAt, &gs.DeletedAt,
	); err != nil {
		return GameSession{}, fmt.Errorf("scanSession: %w", err)
	}
	return gs, nil
}
