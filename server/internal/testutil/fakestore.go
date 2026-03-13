package testutil

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
)

var ErrNotFound = errors.New("not found")

// FakeStore is an in-memory implementation of store.Store for use in tests.
type FakeStore struct {
	Players        map[uuid.UUID]store.Player
	Rooms          map[uuid.UUID]store.Room
	RoomPlayers    map[uuid.UUID][]store.RoomPlayer
	RoomSettings   map[uuid.UUID]map[string]string
	Sessions       map[uuid.UUID]store.GameSession
	Moves          map[uuid.UUID][]store.Move
	AllowedEmails  map[string]store.AllowedEmail
	GameResults    map[uuid.UUID]store.GameResult
	RematchVotes   map[uuid.UUID][]store.RematchVote
	RoomMessages   map[uuid.UUID][]store.RoomMessage
	DirectMessages map[uuid.UUID]store.DirectMessage
	PlayerMutes    map[uuid.UUID][]store.PlayerMute
	Ratings        map[string]store.Rating
	Notifications  map[uuid.UUID]store.Notification
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		Players:        make(map[uuid.UUID]store.Player),
		Rooms:          make(map[uuid.UUID]store.Room),
		RoomPlayers:    make(map[uuid.UUID][]store.RoomPlayer),
		RoomSettings:   make(map[uuid.UUID]map[string]string),
		Sessions:       make(map[uuid.UUID]store.GameSession),
		Moves:          make(map[uuid.UUID][]store.Move),
		AllowedEmails:  make(map[string]store.AllowedEmail),
		GameResults:    make(map[uuid.UUID]store.GameResult),
		RematchVotes:   make(map[uuid.UUID][]store.RematchVote),
		RoomMessages:   make(map[uuid.UUID][]store.RoomMessage),
		DirectMessages: make(map[uuid.UUID]store.DirectMessage),
		PlayerMutes:    make(map[uuid.UUID][]store.PlayerMute),
		Ratings:        make(map[string]store.Rating),
		Notifications:  make(map[uuid.UUID]store.Notification),
	}
}

// --- Players -----------------------------------------------------------------

func (f *FakeStore) CreatePlayer(_ context.Context, username string) (store.Player, error) {
	p := store.Player{ID: uuid.New(), Username: username, CreatedAt: time.Now()}
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

// --- Admin: allowed emails ---------------------------------------------------

func (f *FakeStore) ListAllowedEmails(_ context.Context) ([]store.AllowedEmail, error) {
	var result []store.AllowedEmail
	for _, e := range f.AllowedEmails {
		result = append(result, e)
	}
	return result, nil
}

func (f *FakeStore) AddAllowedEmail(_ context.Context, params store.AddAllowedEmailParams) (store.AllowedEmail, error) {
	e := store.AllowedEmail{Email: params.Email, Role: params.Role, CreatedAt: time.Now()}
	f.AllowedEmails[params.Email] = e
	return e, nil
}

func (f *FakeStore) RemoveAllowedEmail(_ context.Context, email string) error {
	delete(f.AllowedEmails, email)
	return nil
}

// --- Admin: players ----------------------------------------------------------

func (f *FakeStore) ListPlayers(_ context.Context) ([]store.Player, error) {
	var result []store.Player
	for _, p := range f.Players {
		result = append(result, p)
	}
	return result, nil
}

func (f *FakeStore) SetPlayerRole(_ context.Context, playerID uuid.UUID, role store.PlayerRole) error {
	p, ok := f.Players[playerID]
	if !ok {
		return ErrNotFound
	}
	p.Role = role
	f.Players[playerID] = p
	return nil
}

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
		PauseVotes:      []string{},
		ResumeVotes:     []string{},
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

// --- OAuth -------------------------------------------------------------------

func (f *FakeStore) UpsertOAuthIdentity(_ context.Context, _ store.UpsertOAuthParams) (store.OAuthIdentity, error) {
	return store.OAuthIdentity{}, nil
}

func (f *FakeStore) GetOAuthIdentity(_ context.Context, _, _ string) (store.OAuthIdentity, error) {
	return store.OAuthIdentity{}, ErrNotFound
}

func (f *FakeStore) GetOAuthIdentityByEmail(_ context.Context, _ string) (store.OAuthIdentity, error) {
	return store.OAuthIdentity{}, ErrNotFound
}

func (f *FakeStore) IsEmailAllowed(_ context.Context, _ string) (bool, error) {
	return true, nil
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

// --- Room chat ---------------------------------------------------------------

func (f *FakeStore) SaveRoomMessage(_ context.Context, roomID, playerID uuid.UUID, content string) (store.RoomMessage, error) {
	m := store.RoomMessage{
		ID:        uuid.New(),
		RoomID:    roomID,
		PlayerID:  playerID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	f.RoomMessages[roomID] = append(f.RoomMessages[roomID], m)
	return m, nil
}

func (f *FakeStore) GetRoomMessages(_ context.Context, roomID uuid.UUID) ([]store.RoomMessage, error) {
	return f.RoomMessages[roomID], nil
}

func (f *FakeStore) HideRoomMessage(_ context.Context, messageID uuid.UUID) error {
	for roomID, msgs := range f.RoomMessages {
		for i, m := range msgs {
			if m.ID == messageID {
				f.RoomMessages[roomID][i].Hidden = true
				return nil
			}
		}
	}
	return ErrNotFound
}

func (f *FakeStore) ReportRoomMessage(_ context.Context, messageID uuid.UUID) error {
	for roomID, msgs := range f.RoomMessages {
		for i, m := range msgs {
			if m.ID == messageID {
				f.RoomMessages[roomID][i].Reported = true
				return nil
			}
		}
	}
	return ErrNotFound
}

// --- Direct messages ---------------------------------------------------------

func (f *FakeStore) SaveDM(_ context.Context, senderID, receiverID uuid.UUID, content string) (store.DirectMessage, error) {
	m := store.DirectMessage{
		ID:         uuid.New(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		CreatedAt:  time.Now(),
	}
	f.DirectMessages[m.ID] = m
	return m, nil
}

func (f *FakeStore) GetDMHistory(_ context.Context, playerA, playerB uuid.UUID) ([]store.DirectMessage, error) {
	var result []store.DirectMessage
	for _, m := range f.DirectMessages {
		if (m.SenderID == playerA && m.ReceiverID == playerB) ||
			(m.SenderID == playerB && m.ReceiverID == playerA) {
			result = append(result, m)
		}
	}
	return result, nil
}

func (f *FakeStore) MarkDMRead(_ context.Context, messageID uuid.UUID) error {
	m, ok := f.DirectMessages[messageID]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	m.ReadAt = &now
	f.DirectMessages[messageID] = m
	return nil
}

func (f *FakeStore) GetUnreadDMCount(_ context.Context, playerID uuid.UUID) (int, error) {
	count := 0
	for _, m := range f.DirectMessages {
		if m.ReceiverID == playerID && m.ReadAt == nil && !m.Hidden {
			count++
		}
	}
	return count, nil
}

func (f *FakeStore) ReportDM(_ context.Context, messageID uuid.UUID) error {
	m, ok := f.DirectMessages[messageID]
	if !ok {
		return ErrNotFound
	}
	m.Reported = true
	f.DirectMessages[messageID] = m
	return nil
}

// --- Mutes -------------------------------------------------------------------

func (f *FakeStore) MutePlayer(_ context.Context, muterID, mutedID uuid.UUID) error {
	for _, m := range f.PlayerMutes[muterID] {
		if m.MutedID == mutedID {
			return nil // already muted
		}
	}
	f.PlayerMutes[muterID] = append(f.PlayerMutes[muterID], store.PlayerMute{
		MuterID:   muterID,
		MutedID:   mutedID,
		CreatedAt: time.Now(),
	})
	return nil
}

func (f *FakeStore) UnmutePlayer(_ context.Context, muterID, mutedID uuid.UUID) error {
	mutes := f.PlayerMutes[muterID]
	updated := mutes[:0]
	for _, m := range mutes {
		if m.MutedID != mutedID {
			updated = append(updated, m)
		}
	}
	f.PlayerMutes[muterID] = updated
	return nil
}

func (f *FakeStore) GetMutedPlayers(_ context.Context, playerID uuid.UUID) ([]store.PlayerMute, error) {
	return f.PlayerMutes[playerID], nil
}

// --- Ratings -----------------------------------------------------------------

func (f *FakeStore) GetRating(_ context.Context, playerID uuid.UUID, gameID string) (store.Rating, error) {
	r, ok := f.Ratings[playerID.String()+":"+gameID]
	if !ok {
		return store.Rating{}, ErrNotFound
	}
	return r, nil
}

func (f *FakeStore) UpsertRating(_ context.Context, r store.Rating) error {
	r.UpdatedAt = time.Now()
	f.Ratings[r.PlayerID.String()+":"+r.GameID] = r
	return nil
}

func (f *FakeStore) GetRatingLeaderboard(_ context.Context, gameID string, limit int) ([]store.RatingLeaderboardEntry, error) {
	result := []store.RatingLeaderboardEntry{}
	for _, r := range f.Ratings {
		if r.GameID != gameID {
			continue
		}
		p := f.Players[r.PlayerID]
		avatarURL := ""
		if p.AvatarURL != nil {
			avatarURL = *p.AvatarURL
		}
		result = append(result, store.RatingLeaderboardEntry{
			PlayerID:      r.PlayerID,
			GameID:        r.GameID,
			Username:      p.Username,
			AvatarURL:     avatarURL,
			DisplayRating: r.DisplayRating,
			GamesPlayed:   r.GamesPlayed,
			WinStreak:     r.WinStreak,
			LossStreak:    r.LossStreak,
			UpdatedAt:     r.UpdatedAt,
		})
	}
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].DisplayRating > result[j-1].DisplayRating; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// --- Pause / resume votes ----------------------------------------------------

func (f *FakeStore) VotePause(_ context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false, ErrNotFound
	}
	playerStr := playerID.String()
	for _, v := range gs.PauseVotes {
		if v == playerStr {
			return f.checkAllVoted(sessionID, gs.PauseVotes), nil
		}
	}
	gs.PauseVotes = append(gs.PauseVotes, playerStr)
	f.Sessions[sessionID] = gs
	return f.checkAllVoted(sessionID, gs.PauseVotes), nil
}

func (f *FakeStore) ClearPauseVotes(_ context.Context, sessionID uuid.UUID) error {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	gs.PauseVotes = []string{}
	f.Sessions[sessionID] = gs
	return nil
}

func (f *FakeStore) VoteResume(_ context.Context, sessionID uuid.UUID, playerID uuid.UUID) (bool, error) {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false, ErrNotFound
	}
	playerStr := playerID.String()
	for _, v := range gs.ResumeVotes {
		if v == playerStr {
			return f.checkAllVoted(sessionID, gs.ResumeVotes), nil
		}
	}
	gs.ResumeVotes = append(gs.ResumeVotes, playerStr)
	f.Sessions[sessionID] = gs
	return f.checkAllVoted(sessionID, gs.ResumeVotes), nil
}

func (f *FakeStore) ClearResumeVotes(_ context.Context, sessionID uuid.UUID) error {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	gs.ResumeVotes = []string{}
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

// checkAllVoted returns true when the number of votes matches the number of
// players in the session's room.
func (f *FakeStore) checkAllVoted(sessionID uuid.UUID, votes []string) bool {
	gs, ok := f.Sessions[sessionID]
	if !ok {
		return false
	}
	return len(votes) >= len(f.RoomPlayers[gs.RoomID])
}

// --- Notifications -----------------------------------------------------------

func (f *FakeStore) CreateNotification(_ context.Context, params store.CreateNotificationParams) (store.Notification, error) {
	n := store.Notification{
		ID:              uuid.New(),
		PlayerID:        params.PlayerID,
		Type:            params.Type,
		Payload:         params.Payload,
		ActionExpiresAt: params.ActionExpiresAt,
		CreatedAt:       time.Now(),
	}
	f.Notifications[n.ID] = n
	return n, nil
}

func (f *FakeStore) GetNotification(_ context.Context, id uuid.UUID) (store.Notification, error) {
	n, ok := f.Notifications[id]
	if !ok {
		return store.Notification{}, ErrNotFound
	}
	return n, nil
}

func (f *FakeStore) ListNotifications(_ context.Context, playerID uuid.UUID, includeRead bool, readCutoff time.Time) ([]store.Notification, error) {
	var result []store.Notification
	for _, n := range f.Notifications {
		if n.PlayerID != playerID {
			continue
		}
		if !includeRead && n.ReadAt != nil {
			continue
		}
		if includeRead && n.ReadAt != nil && n.CreatedAt.Before(readCutoff) {
			continue
		}
		result = append(result, n)
	}
	if result == nil {
		result = []store.Notification{}
	}
	return result, nil
}

func (f *FakeStore) MarkNotificationRead(_ context.Context, id uuid.UUID) error {
	n, ok := f.Notifications[id]
	if !ok {
		return ErrNotFound
	}
	if n.ReadAt != nil {
		return nil // already read, no-op
	}
	now := time.Now()
	n.ReadAt = &now
	f.Notifications[id] = n
	return nil
}

func (f *FakeStore) SetNotificationAction(_ context.Context, id uuid.UUID, action string) error {
	n, ok := f.Notifications[id]
	if !ok {
		return ErrNotFound
	}
	if n.ActionTaken != nil {
		return store.ErrNotificationActionExpired
	}
	if n.ActionExpiresAt != nil && time.Now().After(*n.ActionExpiresAt) {
		return store.ErrNotificationActionExpired
	}
	n.ActionTaken = &action
	f.Notifications[id] = n
	return nil
}

// --- Exec (used by test migrations) ------------------------------------------

func (f *FakeStore) Exec(_ context.Context, _ string) error { return nil }
