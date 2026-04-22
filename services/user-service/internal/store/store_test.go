package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/testutil"
)

type testEnv struct {
	store store.Store
	pool  *pgxpool.Pool
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial, testutil.MigrationUserService, testutil.MigrationAchievements, testutil.MigrationBotProfile)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return testEnv{store: s, pool: pool}
}

func (e testEnv) seedPlayer(t *testing.T, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO players (id, username) VALUES ($1, $2)`, id, username)
	if err != nil {
		t.Fatalf("seedPlayer: %v", err)
	}
	return id
}

// --- Friendships -------------------------------------------------------------

func TestFriendRequestFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	// Send request.
	f, err := env.store.SendFriendRequest(ctx, alice, bob)
	if err != nil {
		t.Fatalf("SendFriendRequest: %v", err)
	}
	if f.Status != store.FriendshipStatusPending {
		t.Errorf("expected pending, got %s", f.Status)
	}

	// Pending requests visible to bob.
	pending, err := env.store.ListPendingFriendRequests(ctx, bob)
	if err != nil {
		t.Fatalf("ListPendingFriendRequests: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].FriendUsername != "alice" {
		t.Errorf("expected alice, got %s", pending[0].FriendUsername)
	}

	// Accept.
	accepted, err := env.store.AcceptFriendRequest(ctx, alice, bob)
	if err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	if accepted.Status != store.FriendshipStatusAccepted {
		t.Errorf("expected accepted, got %s", accepted.Status)
	}

	// Both see each other as friends.
	friends, _ := env.store.ListFriends(ctx, alice)
	if len(friends) != 1 || friends[0].FriendUsername != "bob" {
		t.Error("alice should see bob as friend")
	}

	friends, _ = env.store.ListFriends(ctx, bob)
	if len(friends) != 1 || friends[0].FriendUsername != "alice" {
		t.Error("bob should see alice as friend")
	}
}

func TestDeclineFriendRequest(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	if _, err := env.store.SendFriendRequest(ctx, alice, bob); err != nil {
		t.Fatal(err)
	}

	if err := env.store.DeclineFriendRequest(ctx, alice, bob); err != nil {
		t.Fatalf("DeclineFriendRequest: %v", err)
	}

	f, _ := env.store.GetFriendship(ctx, alice, bob)
	if f.Status != "" {
		t.Errorf("expected empty status after decline, got %s", f.Status)
	}
}

func TestRemoveFriend(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	if _, err := env.store.SendFriendRequest(ctx, alice, bob); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.AcceptFriendRequest(ctx, alice, bob); err != nil {
		t.Fatal(err)
	}

	if err := env.store.RemoveFriend(ctx, bob, alice); err != nil {
		t.Fatalf("RemoveFriend: %v", err)
	}

	friends, _ := env.store.ListFriends(ctx, alice)
	if len(friends) != 0 {
		t.Error("expected no friends after removal")
	}
}

func TestBlockAndUnblock(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	blocked, err := env.store.BlockPlayer(ctx, alice, bob)
	if err != nil {
		t.Fatalf("BlockPlayer: %v", err)
	}
	if blocked.Status != store.FriendshipStatusBlocked {
		t.Errorf("expected blocked, got %s", blocked.Status)
	}

	if err := env.store.UnblockPlayer(ctx, alice, bob); err != nil {
		t.Fatalf("UnblockPlayer: %v", err)
	}

	f, _ := env.store.GetFriendship(ctx, alice, bob)
	if f.Status != "" {
		t.Errorf("expected empty status after unblock, got %s", f.Status)
	}
}

// --- Mutes -------------------------------------------------------------------

func TestMuteAndUnmute(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	alice := env.seedPlayer(t, "alice")
	bob := env.seedPlayer(t, "bob")

	if err := env.store.MutePlayer(ctx, alice, bob); err != nil {
		t.Fatalf("MutePlayer: %v", err)
	}

	muted, err := env.store.GetMutedPlayers(ctx, alice)
	if err != nil {
		t.Fatalf("GetMutedPlayers: %v", err)
	}
	if len(muted) != 1 || muted[0].MutedID != bob {
		t.Error("expected bob in muted list")
	}

	// Mute again is idempotent (ON CONFLICT DO NOTHING).
	if err := env.store.MutePlayer(ctx, alice, bob); err != nil {
		t.Fatalf("MutePlayer idempotent: %v", err)
	}

	if err := env.store.UnmutePlayer(ctx, alice, bob); err != nil {
		t.Fatalf("UnmutePlayer: %v", err)
	}

	muted, _ = env.store.GetMutedPlayers(ctx, alice)
	if len(muted) != 0 {
		t.Error("expected no muted players after unmute")
	}
}

// --- Bans --------------------------------------------------------------------

func TestBanFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	admin := env.seedPlayer(t, "admin")
	player := env.seedPlayer(t, "player")

	reason := "cheating"
	expiry := time.Now().Add(24 * time.Hour)
	ban, err := env.store.IssueBan(ctx, store.IssueBanParams{
		PlayerID:  player,
		BannedBy:  admin,
		Reason:    &reason,
		ExpiresAt: &expiry,
	})
	if err != nil {
		t.Fatalf("IssueBan: %v", err)
	}

	// Check active ban.
	active, err := env.store.CheckActiveBan(ctx, player)
	if err != nil {
		t.Fatalf("CheckActiveBan: %v", err)
	}
	if active == nil {
		t.Fatal("expected active ban")
	}
	if active.ID != ban.ID {
		t.Error("active ban ID mismatch")
	}

	// Get ban by ID.
	fetched, err := env.store.GetBan(ctx, ban.ID)
	if err != nil {
		t.Fatalf("GetBan: %v", err)
	}
	if *fetched.Reason != "cheating" {
		t.Errorf("expected reason cheating, got %s", *fetched.Reason)
	}

	// Lift ban.
	if err := env.store.LiftBan(ctx, ban.ID, admin); err != nil {
		t.Fatalf("LiftBan: %v", err)
	}

	active, _ = env.store.CheckActiveBan(ctx, player)
	if active != nil {
		t.Error("expected no active ban after lift")
	}

	// List bans history.
	bans, err := env.store.ListBans(ctx, player)
	if err != nil {
		t.Fatalf("ListBans: %v", err)
	}
	if len(bans) != 1 {
		t.Errorf("expected 1 ban in history, got %d", len(bans))
	}
	if bans[0].LiftedAt == nil {
		t.Error("expected lifted_at to be set")
	}
}

func TestCheckActiveBanNoBan(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	player := env.seedPlayer(t, "player")

	active, err := env.store.CheckActiveBan(ctx, player)
	if err != nil {
		t.Fatalf("CheckActiveBan: %v", err)
	}
	if active != nil {
		t.Error("expected nil for unbanned player")
	}
}

// --- Reports -----------------------------------------------------------------

func TestReportFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	reporter := env.seedPlayer(t, "reporter")
	reported := env.seedPlayer(t, "reported")
	reviewer := env.seedPlayer(t, "reviewer")

	ctxData, _ := json.Marshal(map[string]string{"game": "tictactoe"})
	report, err := env.store.CreateReport(ctx, store.CreateReportParams{
		ReporterID: reporter,
		ReportedID: reported,
		Reason:     "harassment",
		Context:    ctxData,
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}
	if report.Status != store.ReportStatusPending {
		t.Errorf("expected pending, got %s", report.Status)
	}

	// List pending.
	pending, err := env.store.ListPendingReports(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListPendingReports: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending report, got %d", len(pending))
	}

	// Review.
	resolution := "warning issued"
	err = env.store.ReviewReport(ctx, store.ReviewReportParams{
		ReportID:   report.ID,
		ReviewedBy: reviewer,
		Resolution: &resolution,
	})
	if err != nil {
		t.Fatalf("ReviewReport: %v", err)
	}

	// No more pending.
	pending, _ = env.store.ListPendingReports(ctx, 50, 0)
	if len(pending) != 0 {
		t.Error("expected 0 pending after review")
	}

	// List by player.
	reports, _ := env.store.ListReportsByPlayer(ctx, reported)
	if len(reports) != 1 {
		t.Errorf("expected 1 report for player, got %d", len(reports))
	}
	if reports[0].Status != store.ReportStatusReviewed {
		t.Errorf("expected reviewed, got %s", reports[0].Status)
	}
}

// --- Profiles ----------------------------------------------------------------

func TestProfileUpsertAndGet(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	player := env.seedPlayer(t, "alice")

	// Get default (no profile row yet).
	profile, err := env.store.GetProfile(ctx, player)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Bio != nil {
		t.Error("expected nil bio for new player")
	}

	// Upsert.
	bio := "I love board games"
	country := "AR"
	updated, err := env.store.UpsertProfile(ctx, store.UpsertProfileParams{
		PlayerID: player,
		Bio:      &bio,
		Country:  &country,
	})
	if err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}
	if *updated.Bio != bio {
		t.Errorf("expected bio %q, got %q", bio, *updated.Bio)
	}
	if *updated.Country != "AR" {
		t.Errorf("expected country AR, got %s", *updated.Country)
	}

	// Update bio only.
	newBio := "Updated bio"
	updated2, _ := env.store.UpsertProfile(ctx, store.UpsertProfileParams{
		PlayerID: player,
		Bio:      &newBio,
		Country:  &country,
	})
	if *updated2.Bio != newBio {
		t.Errorf("expected updated bio")
	}
}

// --- Achievements ------------------------------------------------------------

func TestAchievements(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	player := env.seedPlayer(t, "alice")

	// First upsert creates the row.
	a, err := env.store.UpsertAchievement(ctx, player, "games_played", 1, 1)
	if err != nil {
		t.Fatalf("UpsertAchievement: %v", err)
	}
	if a.AchievementKey != "games_played" {
		t.Errorf("expected games_played, got %s", a.AchievementKey)
	}
	if a.Tier != 1 || a.Progress != 1 {
		t.Errorf("expected tier=1, progress=1, got tier=%d, progress=%d", a.Tier, a.Progress)
	}

	// Upserting again with higher tier updates the row.
	a2, err := env.store.UpsertAchievement(ctx, player, "games_played", 2, 10)
	if err != nil {
		t.Fatalf("UpsertAchievement tier-up: %v", err)
	}
	if a2.Tier != 2 || a2.Progress != 10 {
		t.Errorf("expected tier=2, progress=10, got tier=%d, progress=%d", a2.Tier, a2.Progress)
	}

	// Upserting with lower tier keeps the higher tier.
	a3, err := env.store.UpsertAchievement(ctx, player, "games_played", 1, 10)
	if err != nil {
		t.Fatalf("UpsertAchievement lower tier: %v", err)
	}
	if a3.Tier != 2 {
		t.Errorf("expected tier=2 (kept higher), got tier=%d", a3.Tier)
	}

	achievements, err := env.store.ListAchievements(ctx, player)
	if err != nil {
		t.Fatalf("ListAchievements: %v", err)
	}
	if len(achievements) != 1 {
		t.Errorf("expected 1 achievement, got %d", len(achievements))
	}
}

// --- Search ------------------------------------------------------------------

func TestFindPlayerByUsername(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedPlayer(t, "Alice")

	// Case insensitive search.
	p, err := env.store.FindPlayerByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("FindPlayerByUsername: %v", err)
	}
	if p.Username != "Alice" {
		t.Errorf("expected Alice, got %s", p.Username)
	}

	_, err = env.store.FindPlayerByUsername(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent player")
	}
}

// --- Admin -------------------------------------------------------------------

func TestListPlayers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedPlayer(t, "alice")
	env.seedPlayer(t, "bob")

	players, err := env.store.ListPlayers(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListPlayers: %v", err)
	}
	if len(players) != 2 {
		t.Errorf("expected 2 players, got %d", len(players))
	}
}

func TestSetPlayerRole(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	player := env.seedPlayer(t, "alice")

	if err := env.store.SetPlayerRole(ctx, player, store.RoleManager); err != nil {
		t.Fatalf("SetPlayerRole to manager: %v", err)
	}

	p, _ := env.store.FindPlayerByUsername(ctx, "alice")
	if p.Role != store.RoleManager {
		t.Errorf("expected manager, got %s", p.Role)
	}
}

func TestSetPlayerRoleCannotDemoteOwner(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	owner := env.seedPlayer(t, "owner")
	if _, err := env.pool.Exec(ctx, `UPDATE players SET role = 'owner' WHERE id = $1`, owner); err != nil {
		t.Fatal(err)
	}

	err := env.store.SetPlayerRole(ctx, owner, store.RolePlayer)
	if err == nil {
		t.Error("expected error when demoting owner")
	}
}

func TestAllowedEmailsCRUD(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	inviter := env.seedPlayer(t, "admin")

	added, err := env.store.AddAllowedEmail(ctx, store.AddAllowedEmailParams{
		Email:     "new@example.com",
		Role:      store.RolePlayer,
		InvitedBy: &inviter,
	})
	if err != nil {
		t.Fatalf("AddAllowedEmail: %v", err)
	}
	if added.Email != "new@example.com" {
		t.Errorf("expected new@example.com, got %s", added.Email)
	}

	emails, err := env.store.ListAllowedEmails(ctx)
	if err != nil {
		t.Fatalf("ListAllowedEmails: %v", err)
	}
	if len(emails) != 1 {
		t.Errorf("expected 1 email, got %d", len(emails))
	}

	if err := env.store.RemoveAllowedEmail(ctx, "new@example.com"); err != nil {
		t.Fatalf("RemoveAllowedEmail: %v", err)
	}

	emails, _ = env.store.ListAllowedEmails(ctx)
	if len(emails) != 0 {
		t.Error("expected 0 emails after removal")
	}
}

func TestRemoveAllowedEmailNotFound(t *testing.T) {
	env := newTestEnv(t)
	err := env.store.RemoveAllowedEmail(context.Background(), "nonexistent@example.com")
	if err == nil {
		t.Error("expected error for non-existent email")
	}
}

// --- Settings ----------------------------------------------------------------

func TestSettingsDefaultAndUpsert(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	player := env.seedPlayer(t, "alice")

	// Get defaults (no row yet).
	settings, err := env.store.GetPlayerSettings(ctx, player)
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}
	if *settings.Settings.Theme != "dark" {
		t.Errorf("expected default theme dark, got %s", *settings.Settings.Theme)
	}

	// Upsert custom settings.
	light := "light"
	vol := 0.5
	updated, err := env.store.UpsertPlayerSettings(ctx, player, store.PlayerSettingMap{
		Theme:        &light,
		VolumeMaster: &vol,
	})
	if err != nil {
		t.Fatalf("UpsertPlayerSettings: %v", err)
	}
	if *updated.Settings.Theme != "light" {
		t.Errorf("expected light theme, got %s", *updated.Settings.Theme)
	}

	// Re-fetch — custom values merged with defaults.
	fetched, _ := env.store.GetPlayerSettings(ctx, player)
	if *fetched.Settings.Theme != "light" {
		t.Errorf("expected light after re-fetch, got %s", *fetched.Settings.Theme)
	}
	if *fetched.Settings.VolumeMaster != 0.5 {
		t.Errorf("expected volume 0.5, got %f", *fetched.Settings.VolumeMaster)
	}
	// Non-overridden defaults should still be present.
	if *fetched.Settings.Language != "en" {
		t.Errorf("expected default language en, got %s", *fetched.Settings.Language)
	}
}
