package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
)

// mockStore is an in-memory implementation of store.Store for testing.
type mockStore struct {
	mu            sync.Mutex
	friendships   map[string]store.Friendship   // key: "requesterID|addresseeID"
	mutes         map[string]store.PlayerMute   // key: "muterID|mutedID"
	bans          map[uuid.UUID]store.Ban
	reports       map[uuid.UUID]store.PlayerReport
	profiles      map[uuid.UUID]store.PlayerProfile
	achievements  map[uuid.UUID][]store.PlayerAchievement
	players       []store.Player
	allowedEmails map[string]store.AllowedEmail
	settings      map[uuid.UUID]store.PlayerSettings
	auditLogs     []store.AuditLog
}

func newMockStore() *mockStore {
	return &mockStore{
		friendships:   make(map[string]store.Friendship),
		mutes:         make(map[string]store.PlayerMute),
		bans:          make(map[uuid.UUID]store.Ban),
		reports:       make(map[uuid.UUID]store.PlayerReport),
		profiles:      make(map[uuid.UUID]store.PlayerProfile),
		achievements:  make(map[uuid.UUID][]store.PlayerAchievement),
		players:       []store.Player{},
		allowedEmails: make(map[string]store.AllowedEmail),
		settings:      make(map[uuid.UUID]store.PlayerSettings),
	}
}

func friendKey(a, b uuid.UUID) string { return a.String() + "|" + b.String() }
func muteKey(a, b uuid.UUID) string   { return a.String() + "|" + b.String() }

// --- Friendships -------------------------------------------------------------

func (m *mockStore) GetFriendship(_ context.Context, playerA, playerB uuid.UUID) (store.Friendship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.friendships[friendKey(playerA, playerB)]; ok {
		return f, nil
	}
	if f, ok := m.friendships[friendKey(playerB, playerA)]; ok {
		return f, nil
	}
	return store.Friendship{}, store.ErrNotFound
}

func (m *mockStore) ListFriends(_ context.Context, playerID uuid.UUID) ([]store.FriendshipView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.FriendshipView
	for _, f := range m.friendships {
		if f.Status == store.FriendshipStatusAccepted && (f.RequesterID == playerID || f.AddresseeID == playerID) {
			friendID := f.AddresseeID
			if f.AddresseeID == playerID {
				friendID = f.RequesterID
			}
			out = append(out, store.FriendshipView{
				FriendID:       friendID,
				FriendUsername: "mock-user",
				Status:         f.Status,
				CreatedAt:      f.CreatedAt,
			})
		}
	}
	if out == nil {
		out = []store.FriendshipView{}
	}
	return out, nil
}

func (m *mockStore) ListPendingFriendRequests(_ context.Context, playerID uuid.UUID) ([]store.FriendshipView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.FriendshipView
	for _, f := range m.friendships {
		if f.Status == store.FriendshipStatusPending && f.AddresseeID == playerID {
			out = append(out, store.FriendshipView{
				FriendID:       f.RequesterID,
				FriendUsername: "mock-user",
				Status:         f.Status,
				CreatedAt:      f.CreatedAt,
			})
		}
	}
	if out == nil {
		out = []store.FriendshipView{}
	}
	return out, nil
}

func (m *mockStore) SendFriendRequest(_ context.Context, requesterID, addresseeID uuid.UUID) (store.Friendship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := friendKey(requesterID, addresseeID)
	if _, ok := m.friendships[key]; ok {
		return store.Friendship{}, store.ErrConflict
	}
	f := store.Friendship{
		RequesterID: requesterID,
		AddresseeID: addresseeID,
		Status:      store.FriendshipStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.friendships[key] = f
	return f, nil
}

func (m *mockStore) AcceptFriendRequest(_ context.Context, requesterID, addresseeID uuid.UUID) (store.Friendship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := friendKey(requesterID, addresseeID)
	f, ok := m.friendships[key]
	if !ok || f.Status != store.FriendshipStatusPending {
		return store.Friendship{}, store.ErrNotFound
	}
	f.Status = store.FriendshipStatusAccepted
	f.UpdatedAt = time.Now()
	m.friendships[key] = f
	return f, nil
}

func (m *mockStore) DeclineFriendRequest(_ context.Context, requesterID, addresseeID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := friendKey(requesterID, addresseeID)
	f, ok := m.friendships[key]
	if !ok || f.Status != store.FriendshipStatusPending {
		return store.ErrNotFound
	}
	delete(m.friendships, key)
	return nil
}

func (m *mockStore) BlockPlayer(_ context.Context, requesterID, addresseeID uuid.UUID) (store.Friendship, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Remove any existing friendship in either direction.
	delete(m.friendships, friendKey(requesterID, addresseeID))
	delete(m.friendships, friendKey(addresseeID, requesterID))

	key := friendKey(requesterID, addresseeID)
	f := store.Friendship{
		RequesterID: requesterID,
		AddresseeID: addresseeID,
		Status:      store.FriendshipStatusBlocked,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.friendships[key] = f
	return f, nil
}

func (m *mockStore) UnblockPlayer(_ context.Context, requesterID, addresseeID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := friendKey(requesterID, addresseeID)
	f, ok := m.friendships[key]
	if !ok || f.Status != store.FriendshipStatusBlocked {
		return store.ErrNotFound
	}
	delete(m.friendships, key)
	return nil
}

func (m *mockStore) RemoveFriend(_ context.Context, playerA, playerB uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	keyAB := friendKey(playerA, playerB)
	keyBA := friendKey(playerB, playerA)
	if f, ok := m.friendships[keyAB]; ok && f.Status == store.FriendshipStatusAccepted {
		delete(m.friendships, keyAB)
		return nil
	}
	if f, ok := m.friendships[keyBA]; ok && f.Status == store.FriendshipStatusAccepted {
		delete(m.friendships, keyBA)
		return nil
	}
	return store.ErrNotFound
}

// --- Mutes -------------------------------------------------------------------

func (m *mockStore) MutePlayer(_ context.Context, muterID, mutedID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mutes[muteKey(muterID, mutedID)] = store.PlayerMute{
		MuterID:   muterID,
		MutedID:   mutedID,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *mockStore) UnmutePlayer(_ context.Context, muterID, mutedID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := muteKey(muterID, mutedID)
	if _, ok := m.mutes[key]; !ok {
		return store.ErrNotFound
	}
	delete(m.mutes, key)
	return nil
}

func (m *mockStore) GetMutedPlayers(_ context.Context, playerID uuid.UUID) ([]store.PlayerMute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.PlayerMute
	for _, mt := range m.mutes {
		if mt.MuterID == playerID {
			out = append(out, mt)
		}
	}
	if out == nil {
		out = []store.PlayerMute{}
	}
	return out, nil
}

// --- Bans --------------------------------------------------------------------

func (m *mockStore) IssueBan(_ context.Context, params store.IssueBanParams) (store.Ban, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ban := store.Ban{
		ID:        uuid.New(),
		PlayerID:  params.PlayerID,
		BannedBy:  params.BannedBy,
		Reason:    params.Reason,
		ExpiresAt: params.ExpiresAt,
		CreatedAt: time.Now(),
	}
	m.bans[ban.ID] = ban
	return ban, nil
}

func (m *mockStore) GetBan(_ context.Context, banID uuid.UUID) (store.Ban, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ban, ok := m.bans[banID]
	if !ok {
		return store.Ban{}, store.ErrNotFound
	}
	return ban, nil
}

func (m *mockStore) LiftBan(_ context.Context, banID, liftedBy uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ban, ok := m.bans[banID]
	if !ok || ban.LiftedAt != nil {
		return store.ErrNotFound
	}
	now := time.Now()
	ban.LiftedAt = &now
	ban.LiftedBy = &liftedBy
	m.bans[banID] = ban
	return nil
}

func (m *mockStore) CheckActiveBan(_ context.Context, playerID uuid.UUID) (*store.Ban, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.bans {
		if b.PlayerID == playerID && b.LiftedAt == nil {
			return &b, nil
		}
	}
	return nil, nil
}

func (m *mockStore) ListBans(_ context.Context, playerID uuid.UUID) ([]store.Ban, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.Ban
	for _, b := range m.bans {
		if b.PlayerID == playerID {
			out = append(out, b)
		}
	}
	if out == nil {
		out = []store.Ban{}
	}
	return out, nil
}

// --- Reports -----------------------------------------------------------------

func (m *mockStore) CreateReport(_ context.Context, params store.CreateReportParams) (store.PlayerReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rpt := store.PlayerReport{
		ID:         uuid.New(),
		ReporterID: params.ReporterID,
		ReportedID: params.ReportedID,
		Reason:     params.Reason,
		Context:    params.Context,
		Status:     store.ReportStatusPending,
		CreatedAt:  time.Now(),
	}
	m.reports[rpt.ID] = rpt
	return rpt, nil
}

func (m *mockStore) ReviewReport(_ context.Context, params store.ReviewReportParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rpt, ok := m.reports[params.ReportID]
	if !ok || rpt.Status != store.ReportStatusPending {
		return store.ErrNotFound
	}
	now := time.Now()
	rpt.Status = store.ReportStatusReviewed
	rpt.ReviewedBy = &params.ReviewedBy
	rpt.ReviewedAt = &now
	rpt.Resolution = params.Resolution
	rpt.BanID = params.BanID
	m.reports[params.ReportID] = rpt
	return nil
}

func (m *mockStore) ListPendingReports(_ context.Context) ([]store.PlayerReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.PlayerReport
	for _, rpt := range m.reports {
		if rpt.Status == store.ReportStatusPending {
			out = append(out, rpt)
		}
	}
	if out == nil {
		out = []store.PlayerReport{}
	}
	return out, nil
}

func (m *mockStore) ListReportsByPlayer(_ context.Context, reportedID uuid.UUID) ([]store.PlayerReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.PlayerReport
	for _, rpt := range m.reports {
		if rpt.ReportedID == reportedID {
			out = append(out, rpt)
		}
	}
	if out == nil {
		out = []store.PlayerReport{}
	}
	return out, nil
}

// --- Profiles ----------------------------------------------------------------

func (m *mockStore) GetProfile(_ context.Context, playerID uuid.UUID) (store.PlayerProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[playerID]
	if !ok {
		return store.PlayerProfile{}, store.ErrNotFound
	}
	return p, nil
}

func (m *mockStore) UpsertProfile(_ context.Context, params store.UpsertProfileParams) (store.PlayerProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := store.PlayerProfile{
		PlayerID:  params.PlayerID,
		Bio:       params.Bio,
		Country:   params.Country,
		UpdatedAt: time.Now(),
	}
	m.profiles[params.PlayerID] = p
	return p, nil
}

// --- Achievements ------------------------------------------------------------

func (m *mockStore) UpsertAchievement(_ context.Context, playerID uuid.UUID, key string, tier, progress int) (store.PlayerAchievement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check if it already exists — update in place.
	for i, existing := range m.achievements[playerID] {
		if existing.AchievementKey == key {
			if tier > existing.Tier {
				m.achievements[playerID][i].Tier = tier
			}
			m.achievements[playerID][i].Progress = progress
			return m.achievements[playerID][i], nil
		}
	}
	a := store.PlayerAchievement{
		ID:             uuid.New(),
		PlayerID:       playerID,
		AchievementKey: key,
		Tier:           tier,
		Progress:       progress,
		UnlockedAt:     time.Now(),
	}
	m.achievements[playerID] = append(m.achievements[playerID], a)
	return a, nil
}

func (m *mockStore) ListAchievements(_ context.Context, playerID uuid.UUID) ([]store.PlayerAchievement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.achievements[playerID]
	if out == nil {
		out = []store.PlayerAchievement{}
	}
	return out, nil
}

// --- Admin: players ----------------------------------------------------------

func (m *mockStore) ListPlayers(_ context.Context, limit, offset int) ([]store.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.players, nil
}

func (m *mockStore) SetPlayerRole(_ context.Context, playerID uuid.UUID, role store.PlayerRole) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.players {
		if p.ID == playerID {
			m.players[i].Role = role
			return nil
		}
	}
	return store.ErrNotFound
}

// --- Admin: allowed emails ---------------------------------------------------

func (m *mockStore) ListAllowedEmails(_ context.Context) ([]store.AllowedEmail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.AllowedEmail
	for _, e := range m.allowedEmails {
		out = append(out, e)
	}
	if out == nil {
		out = []store.AllowedEmail{}
	}
	return out, nil
}

func (m *mockStore) AddAllowedEmail(_ context.Context, params store.AddAllowedEmailParams) (store.AllowedEmail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := store.AllowedEmail{
		Email:     params.Email,
		Role:      params.Role,
		InvitedBy: params.InvitedBy,
		CreatedAt: time.Now(),
	}
	m.allowedEmails[params.Email] = e
	return e, nil
}

func (m *mockStore) RemoveAllowedEmail(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.allowedEmails[email]; !ok {
		return store.ErrNotFound
	}
	delete(m.allowedEmails, email)
	return nil
}

// --- Player settings ---------------------------------------------------------

func (m *mockStore) GetPlayerSettings(_ context.Context, playerID uuid.UUID) (store.PlayerSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.settings[playerID]
	if !ok {
		// Return empty settings (handler returns defaults on the DB side).
		return store.PlayerSettings{
			PlayerID: playerID,
			Settings: store.PlayerSettingMap{},
		}, nil
	}
	return s, nil
}

func (m *mockStore) FindPlayerByUsername(_ context.Context, username string) (store.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.players {
		if p.Username == username {
			return p, nil
		}
	}
	return store.Player{}, fmt.Errorf("player not found")
}

func (m *mockStore) UpsertPlayerSettings(_ context.Context, playerID uuid.UUID, settings store.PlayerSettingMap) (store.PlayerSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := store.PlayerSettings{
		PlayerID:  playerID,
		Settings:  settings,
		UpdatedAt: time.Now(),
	}
	m.settings[playerID] = s
	return s, nil
}

// --- Audit logs --------------------------------------------------------------

func (m *mockStore) LogAction(_ context.Context, actorID uuid.UUID, action, targetType, targetID string, details map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var detailsJSON []byte
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}
	m.auditLogs = append(m.auditLogs, store.AuditLog{
		ID:        uuid.New(),
		ActorID:   actorID,
		Action:    action,
		TargetType: targetType,
		TargetID:  targetID,
		Details:   detailsJSON,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *mockStore) ListAuditLogs(_ context.Context, filter store.AuditFilter) ([]store.AuditLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.AuditLog
	for _, l := range m.auditLogs {
		if filter.ActorID != nil && l.ActorID != *filter.ActorID {
			continue
		}
		if filter.Action != nil && l.Action != *filter.Action {
			continue
		}
		if filter.TargetType != nil && l.TargetType != *filter.TargetType {
			continue
		}
		if filter.From != nil && l.CreatedAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && l.CreatedAt.After(*filter.To) {
			continue
		}
		out = append(out, l)
	}
	if out == nil {
		out = []store.AuditLog{}
	}
	// Apply limit/offset
	if filter.Offset > 0 && filter.Offset < len(out) {
		out = out[filter.Offset:]
	} else if filter.Offset >= len(out) {
		return []store.AuditLog{}, nil
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}
