package testutil

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/platform/store"
)

var ErrNotFound = errors.New("not found")

// FakeStore is an in-memory implementation of store.Store for use in tests.
type FakeStore struct {
	Players           map[uuid.UUID]store.Player
	Rooms             map[uuid.UUID]store.Room
	RoomPlayers       map[uuid.UUID][]store.RoomPlayer
	RoomSettings      map[uuid.UUID]map[string]string
	Sessions          map[uuid.UUID]store.GameSession
	Moves             map[uuid.UUID][]store.Move
	PauseVoteMap      map[uuid.UUID][]uuid.UUID
	ResumeVoteMap     map[uuid.UUID][]uuid.UUID
	GameResults       map[uuid.UUID]store.GameResult
	RematchVotes      map[uuid.UUID][]store.RematchVote
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		Players:           make(map[uuid.UUID]store.Player),
		Rooms:             make(map[uuid.UUID]store.Room),
		RoomPlayers:       make(map[uuid.UUID][]store.RoomPlayer),
		RoomSettings:      make(map[uuid.UUID]map[string]string),
		Sessions:          make(map[uuid.UUID]store.GameSession),
		Moves:             make(map[uuid.UUID][]store.Move),
		PauseVoteMap:      make(map[uuid.UUID][]uuid.UUID),
		ResumeVoteMap:     make(map[uuid.UUID][]uuid.UUID),
		GameResults:       make(map[uuid.UUID]store.GameResult),
		RematchVotes:      make(map[uuid.UUID][]store.RematchVote),
	}
}

// --- Players -----------------------------------------------------------------

func (f *FakeStore) CreatePlayer(_ context.Context, username string) (store.Player, error) {
	p := store.Player{ID: uuid.New(), Username: username, IsBot: false, CreatedAt: time.Now()}
	f.Players[p.ID] = p
	return p, nil
}

// CreateBotPlayer inserts a player with IsBot = true.
func (f *FakeStore) CreateBotPlayer(_ context.Context, username string) (store.Player, error) {
	p := store.Player{ID: uuid.New(), Username: username, IsBot: true, CreatedAt: time.Now()}
	f.Players[p.ID] = p
	return p, nil
}

func (f *FakeStore) GetPlayer(_ context.Context, id uuid.UUID) (store.Player, error) {
	p, ok := f.Players[id]
	if !ok {
		return store.Player{}, ErrNotFound
	}
	return p, nil
}

func (f *FakeStore) GetPlayerByUsername(_ context.Context, username string) (store.Player, error) {
	for _, p := range f.Players {
		if p.Username == username {
			return p, nil
		}
	}
	return store.Player{}, ErrNotFound
}

func (f *FakeStore) UpdatePlayerAvatar(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (f *FakeStore) SoftDeletePlayer(_ context.Context, _ uuid.UUID) error             { return nil }

// --- Rooms -------------------------------------------------------------------

func (f *FakeStore) CreateRoom(_ context.Context, p store.CreateRoomParams) (store.Room, error) {
	r := store.Room{
		ID:         uuid.New(),
		Code:       p.Code,
		GameID:     p.GameID,
		OwnerID:    p.OwnerID,
		Status:     store.RoomStatusWaiting,
		MaxPlayers: p.MaxPlayers,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	f.Rooms[r.ID] = r

	if len(p.DefaultSettings) > 0 {
		f.RoomSettings[r.ID] = make(map[string]string, len(p.DefaultSettings))
		for k, v := range p.DefaultSettings {
			f.RoomSettings[r.ID][k] = v
		}
	}

	return r, nil
}

func (f *FakeStore) GetRoom(_ context.Context, id uuid.UUID) (store.Room, error) {
	r, ok := f.Rooms[id]
	if !ok {
		return store.Room{}, ErrNotFound
	}
	return r, nil
}

func (f *FakeStore) GetRoomByCode(_ context.Context, code string) (store.Room, error) {
	for _, r := range f.Rooms {
		if r.Code == code {
			return r, nil
		}
	}
	return store.Room{}, ErrNotFound
}

func (f *FakeStore) UpdateRoomStatus(_ context.Context, id uuid.UUID, status store.RoomStatus) error {
	r, ok := f.Rooms[id]
	if !ok {
		return ErrNotFound
	}
	r.Status = status
	f.Rooms[id] = r
	return nil
}

func (f *FakeStore) ListWaitingRooms(_ context.Context) ([]store.Room, error) {
	var rooms []store.Room
	for _, r := range f.Rooms {
		if r.Status == store.RoomStatusWaiting {
			rooms = append(rooms, r)
		}
	}
	return rooms, nil
}

func (f *FakeStore) SoftDeleteRoom(_ context.Context, _ uuid.UUID) error { return nil }

func (f *FakeStore) UpdateRoomOwner(_ context.Context, roomID, newOwnerID uuid.UUID) error {
	r, ok := f.Rooms[roomID]
	if !ok {
		return ErrNotFound
	}
	r.OwnerID = newOwnerID
	f.Rooms[roomID] = r
	return nil
}

func (f *FakeStore) DeleteRoom(_ context.Context, roomID uuid.UUID) error {
	delete(f.Rooms, roomID)
	return nil
}

// --- Room settings -----------------------------------------------------------

func (f *FakeStore) GetRoomSettings(_ context.Context, roomID uuid.UUID) (map[string]string, error) {
	settings, ok := f.RoomSettings[roomID]
	if !ok {
		return map[string]string{}, nil
	}
	result := make(map[string]string, len(settings))
	for k, v := range settings {
		result[k] = v
	}
	return result, nil
}

func (f *FakeStore) SetRoomSetting(_ context.Context, roomID uuid.UUID, key, value string) error {
	if _, ok := f.RoomSettings[roomID]; !ok {
		f.RoomSettings[roomID] = make(map[string]string)
	}
	f.RoomSettings[roomID][key] = value
	return nil
}

// --- Room players ------------------------------------------------------------

func (f *FakeStore) AddPlayerToRoom(_ context.Context, roomID, playerID uuid.UUID, seat int) error {
	f.RoomPlayers[roomID] = append(f.RoomPlayers[roomID], store.RoomPlayer{
		RoomID: roomID, PlayerID: playerID, Seat: seat, JoinedAt: time.Now(),
	})
	return nil
}

func (f *FakeStore) RemovePlayerFromRoom(_ context.Context, roomID, playerID uuid.UUID) error {
	players := f.RoomPlayers[roomID]
	updated := players[:0]
	for _, p := range players {
		if p.PlayerID != playerID {
			updated = append(updated, p)
		}
	}
	f.RoomPlayers[roomID] = updated
	return nil
}

func (f *FakeStore) ListRoomPlayers(_ context.Context, roomID uuid.UUID) ([]store.RoomPlayer, error) {
	return f.RoomPlayers[roomID], nil
}

// --- Game sessions -----------------------------------------------------------

func (f *FakeStore) CreateGameSession(
	_ context.Context,
	roomID uuid.UUID,
	gameID string,
	initialState []byte,
	turnTimeoutSecs *int,
	mode store.SessionMode,
) (store.GameSession, error) {
	gs := store.GameSession{
		ID:              uuid.New(),
		RoomID:          roomID,
		GameID:          gameID,
		State:           initialState,
		Mode:            mode,
		ReadyPlayers:    []string{},
		StartedAt:       time.Now(),
		LastMoveAt:      time.Now(),
		TurnTimeoutSecs: turnTimeoutSecs,
	}
	f.Sessions[gs.ID] = gs
	return gs, nil
}

func (f *FakeStore) GetGameSession(_ context.Context, id uuid.UUID) (store.GameSession, error) {
	gs, ok := f.Sessions[id]
	if !ok {
		return store.GameSession{}, ErrNotFound
	}
	return gs, nil
}

func (f *FakeStore) GetActiveSessionByRoom(_ context.Context, roomID uuid.UUID) (store.GameSession, error) {
	for _, gs := range f.Sessions {
		if gs.RoomID == roomID && gs.FinishedAt == nil {
			return gs, nil
		}
	}
	return store.GameSession{}, ErrNotFound
}

func (f *FakeStore) UpdateSessionState(_ context.Context, id uuid.UUID, state []byte) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	gs.State = state
	gs.MoveCount++
	f.Sessions[id] = gs
	return nil
}

func (f *FakeStore) FinishSession(_ context.Context, id uuid.UUID) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	gs.FinishedAt = &now
	f.Sessions[id] = gs
	return nil
}

func (f *FakeStore) SuspendSession(_ context.Context, id uuid.UUID, reason string) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	gs.SuspendedAt = &now
	gs.SuspendedReason = &reason
	gs.SuspendCount++
	f.Sessions[id] = gs
	return nil
}

func (f *FakeStore) ResumeSession(_ context.Context, id uuid.UUID) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	gs.SuspendedAt = nil
	gs.SuspendedReason = nil
	f.Sessions[id] = gs
	return nil
}

func (f *FakeStore) ListActiveSessions(_ context.Context, playerID uuid.UUID) ([]store.GameSession, error) {
	var sessions []store.GameSession
	for _, gs := range f.Sessions {
		if gs.FinishedAt != nil || gs.DeletedAt != nil {
			continue
		}
		for _, rp := range f.RoomPlayers[gs.RoomID] {
			if rp.PlayerID == playerID {
				sessions = append(sessions, gs)
				break
			}
		}
	}
	return sessions, nil
}

func (f *FakeStore) SoftDeleteSession(_ context.Context, id uuid.UUID) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	gs.DeletedAt = &now
	f.Sessions[id] = gs
	return nil
}

// GetGameConfig returns sensible defaults for any game ID.
// Tests do not populate game configs, so returning defaults avoids
// resolveTimeout failures in lobby.StartGame.
func (f *FakeStore) GetGameConfig(_ context.Context, _ string) (store.GameConfig, error) {
	return store.GameConfig{
		DefaultTimeoutSecs: 60,
		MinTimeoutSecs:     10,
		MaxTimeoutSecs:     300,
	}, nil
}

func (f *FakeStore) TouchLastMoveAt(_ context.Context, id uuid.UUID) error {
	gs, ok := f.Sessions[id]
	if !ok {
		return ErrNotFound
	}
	gs.LastMoveAt = time.Now()
	f.Sessions[id] = gs
	return nil
}

func (f *FakeStore) CountFinishedSessions(_ context.Context, roomID uuid.UUID) (int, error) {
	count := 0
	for _, gs := range f.Sessions {
		if gs.RoomID == roomID && gs.FinishedAt != nil {
			count++
		}
	}
	return count, nil
}

func (f *FakeStore) GetLastFinishedSession(_ context.Context, roomID uuid.UUID) (store.GameSession, error) {
	var last *store.GameSession
	for _, gs := range f.Sessions {
		gs := gs
		if gs.RoomID == roomID && gs.FinishedAt != nil {
			if last == nil || gs.FinishedAt.After(*last.FinishedAt) {
				last = &gs
			}
		}
	}
	if last == nil {
		return store.GameSession{}, ErrNotFound
	}
	return *last, nil
}

// ListSessionsNeedingTimer returns active, non-suspended sessions with a turn
// timeout configured. Used by TurnTimer.ReschedulePending on startup.
func (f *FakeStore) ListSessionsNeedingTimer(_ context.Context) ([]store.GameSession, error) {
	var result []store.GameSession
	for _, gs := range f.Sessions {
		if gs.FinishedAt != nil || gs.DeletedAt != nil || gs.SuspendedAt != nil {
			continue
		}
		if gs.TurnTimeoutSecs == nil || *gs.TurnTimeoutSecs <= 0 {
			continue
		}
		result = append(result, gs)
	}
	return result, nil
}

// --- Moves -------------------------------------------------------------------

func (f *FakeStore) RecordMove(_ context.Context, params store.RecordMoveParams) (store.Move, error) {
	m := store.Move{
		ID:         uuid.New(),
		SessionID:  params.SessionID,
		PlayerID:   params.PlayerID,
		Payload:    params.Payload,
		StateAfter: params.StateAfter,
		MoveNumber: params.MoveNumber,
		AppliedAt:  time.Now(),
	}
	f.Moves[params.SessionID] = append(f.Moves[params.SessionID], m)
	return m, nil
}

func (f *FakeStore) ListSessionMoves(_ context.Context, sessionID uuid.UUID) ([]store.Move, error) {
	return f.Moves[sessionID], nil
}

func (f *FakeStore) GetMoveAt(_ context.Context, sessionID uuid.UUID, moveNumber int) (store.Move, error) {
	for _, m := range f.Moves[sessionID] {
		if m.MoveNumber == moveNumber {
			return m, nil
		}
	}
	return store.Move{}, ErrNotFound
}

// --- Results -----------------------------------------------------------------

func (f *FakeStore) CreateGameResult(_ context.Context, params store.CreateGameResultParams) (store.GameResult, error) {
	r := store.GameResult{
		ID:        uuid.New(),
		SessionID: params.SessionID,
		GameID:    params.GameID,
		WinnerID:  params.WinnerID,
		IsDraw:    params.IsDraw,
		EndedBy:   params.EndedBy,
		CreatedAt: time.Now(),
	}
	f.GameResults[params.SessionID] = r
	return r, nil
}

func (f *FakeStore) GetGameResult(_ context.Context, sessionID uuid.UUID) (store.GameResult, error) {
	r, ok := f.GameResults[sessionID]
	if !ok {
		return store.GameResult{}, ErrNotFound
	}
	return r, nil
}

func (f *FakeStore) GetPlayerStats(_ context.Context, playerID uuid.UUID) (store.PlayerStats, error) {
	return store.PlayerStats{PlayerID: playerID}, nil
}

func (f *FakeStore) ListPlayerHistory(_ context.Context, _ uuid.UUID, _, _ int) ([]store.GameResult, error) {
	return []store.GameResult{}, nil
}

// --- Rematch -----------------------------------------------------------------

func (f *FakeStore) UpsertRematchVote(_ context.Context, sessionID, playerID uuid.UUID) error {
	votes := f.RematchVotes[sessionID]
	for _, v := range votes {
		if v.PlayerID == playerID {
			return nil // already voted
		}
	}
	f.RematchVotes[sessionID] = append(votes, store.RematchVote{
		SessionID: sessionID,
		PlayerID:  playerID,
		VotedAt:   time.Now(),
	})
	return nil
}

func (f *FakeStore) ListRematchVotes(_ context.Context, sessionID uuid.UUID) ([]store.RematchVote, error) {
	return f.RematchVotes[sessionID], nil
}

func (f *FakeStore) DeleteRematchVotes(_ context.Context, sessionID uuid.UUID) error {
	delete(f.RematchVotes, sessionID)
	return nil
}

// --- Session events ----------------------------------------------------------

func (f *FakeStore) BulkCreateSessionEvents(_ context.Context, _ []store.CreateSessionEventParams) error {
	return nil
}

func (f *FakeStore) ListSessionEvents(_ context.Context, _ uuid.UUID) ([]store.SessionEvent, error) {
	return nil, nil
}

// --- Pause / resume votes ----------------------------------------------------

func (f *FakeStore) VotePause(_ context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	if _, ok := f.Sessions[sessionID]; !ok {
		return false, ErrNotFound
	}
	for _, id := range f.PauseVoteMap[sessionID] {
		if id == playerID {
			return f.checkAllVotedByID(sessionID, f.PauseVoteMap[sessionID]), nil
		}
	}
	f.PauseVoteMap[sessionID] = append(f.PauseVoteMap[sessionID], playerID)
	return f.checkAllVotedByID(sessionID, f.PauseVoteMap[sessionID]), nil
}

func (f *FakeStore) ClearPauseVotes(_ context.Context, sessionID uuid.UUID) error {
	f.PauseVoteMap[sessionID] = []uuid.UUID{}
	return nil
}

func (f *FakeStore) VoteResume(_ context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	if _, ok := f.Sessions[sessionID]; !ok {
		return false, ErrNotFound
	}
	for _, id := range f.ResumeVoteMap[sessionID] {
		if id == playerID {
			return f.checkAllVotedByID(sessionID, f.ResumeVoteMap[sessionID]), nil
		}
	}
	f.ResumeVoteMap[sessionID] = append(f.ResumeVoteMap[sessionID], playerID)
	return f.checkAllVotedByID(sessionID, f.ResumeVoteMap[sessionID]), nil
}

func (f *FakeStore) ClearResumeVotes(_ context.Context, sessionID uuid.UUID) error {
	f.ResumeVoteMap[sessionID] = []uuid.UUID{}
	return nil
}

func (f *FakeStore) CountPauseVotes(_ context.Context, sessionID uuid.UUID) (int, error) {
	return len(f.PauseVoteMap[sessionID]), nil
}

func (f *FakeStore) CountResumeVotes(_ context.Context, sessionID uuid.UUID) (int, error) {
	return len(f.ResumeVoteMap[sessionID]), nil
}

// --- Ready handshake ---------------------------------------------------------

func (f *FakeStore) VoteReady(_ context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false, ErrNotFound
	}
	playerStr := playerID.String()
	for _, v := range gs.ReadyPlayers {
		if v == playerStr {
			return f.checkAllVoted(sessionID, gs.ReadyPlayers), nil
		}
	}
	gs.ReadyPlayers = append(gs.ReadyPlayers, playerStr)
	f.Sessions[sessionID] = gs
	return f.checkAllVoted(sessionID, gs.ReadyPlayers), nil
}

func (f *FakeStore) ClearReadyVotes(_ context.Context, sessionID uuid.UUID) error {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	gs.ReadyPlayers = []string{}
	f.Sessions[sessionID] = gs
	return nil
}

func (f *FakeStore) ForceCloseSession(_ context.Context, sessionID uuid.UUID) error {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	gs.FinishedAt = &now
	gs.SuspendedAt = nil
	gs.SuspendedReason = nil
	f.Sessions[sessionID] = gs
	return nil
}

// checkAllVoted es el original — recibe []string, usado para ready players.
func (f *FakeStore) checkAllVoted(sessionID uuid.UUID, votes []string) bool {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false
	}
	return len(votes) >= len(f.RoomPlayers[gs.RoomID])
}

// checkAllVotedByID — recibe []uuid.UUID, usado para pause/resume votes.
func (f *FakeStore) checkAllVotedByID(sessionID uuid.UUID, votes []uuid.UUID) bool {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false
	}
	return len(votes) >= len(f.RoomPlayers[gs.RoomID])
}

// --- Match history -----------------------------------------------------------

func (f *FakeStore) ListPlayerMatches(_ context.Context, _ uuid.UUID, _, _ int) ([]store.MatchHistoryEntry, error) {
	return nil, nil
}

func (f *FakeStore) CountPlayerMatches(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

// --- Player settings ---------------------------------------------------------

// --- Admin stats -------------------------------------------------------------

func (f *FakeStore) CountActiveRooms(_ context.Context) (int, error) {
	count := 0
	for _, r := range f.Rooms {
		if r.DeletedAt == nil && (r.Status == store.RoomStatusWaiting || r.Status == store.RoomStatusInProgress) {
			count++
		}
	}
	return count, nil
}

func (f *FakeStore) CountActiveSessions(_ context.Context) (int, error) {
	count := 0
	for _, gs := range f.Sessions {
		if gs.FinishedAt == nil && gs.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (f *FakeStore) CountTotalPlayers(_ context.Context) (int, error) {
	count := 0
	for _, p := range f.Players {
		if p.DeletedAt == nil && !p.IsBot {
			count++
		}
	}
	return count, nil
}

func (f *FakeStore) CountSessionsToday(_ context.Context) (int, error) {
	count := 0
	today := time.Now().Truncate(24 * time.Hour)
	for _, gs := range f.Sessions {
		if gs.DeletedAt == nil && !gs.StartedAt.Before(today) {
			count++
		}
	}
	return count, nil
}

// --- Cleanup -----------------------------------------------------------------

func (f *FakeStore) CleanupOrphanRooms(_ context.Context, waitingMaxAge, finishedMaxAge time.Duration) (int, error) {
	count := 0
	now := time.Now()
	for id, r := range f.Rooms {
		if r.DeletedAt != nil {
			continue
		}
		if r.Status == store.RoomStatusWaiting && now.Sub(r.UpdatedAt) > waitingMaxAge {
			if len(f.RoomPlayers[id]) == 0 {
				delete(f.Rooms, id)
				count++
			}
		}
		if r.Status == store.RoomStatusFinished && now.Sub(r.UpdatedAt) > finishedMaxAge {
			now := time.Now()
			r.DeletedAt = &now
			f.Rooms[id] = r
			count++
		}
	}
	return count, nil
}

// --- Exec (used by test migrations) ------------------------------------------

func (f *FakeStore) Exec(_ context.Context, _ string) error { return nil }
