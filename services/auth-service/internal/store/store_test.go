package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/auth-service/internal/handler"
	"github.com/recess/auth-service/internal/store"
	"github.com/recess/shared/testutil"
)

type testEnv struct {
	store *store.Store
	pool  *pgxpool.Pool
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(s.Close)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return testEnv{store: s, pool: pool}
}

func (e testEnv) seedAllowedEmail(t *testing.T, email, role string) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO allowed_emails (email, role) VALUES ($1, $2::player_role)`,
		email, role,
	)
	if err != nil {
		t.Fatalf("seedAllowedEmail: %v", err)
	}
}

func TestIsEmailAllowed(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedAllowedEmail(t, "alice@example.com", "player")

	allowed, err := env.store.IsEmailAllowed(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("IsEmailAllowed: %v", err)
	}
	if !allowed {
		t.Error("expected alice to be allowed")
	}

	allowed, err = env.store.IsEmailAllowed(ctx, "unknown@example.com")
	if err != nil {
		t.Fatalf("IsEmailAllowed: %v", err)
	}
	if allowed {
		t.Error("expected unknown email to not be allowed")
	}
}

func TestIsEmailAllowedExpired(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// Insert an expired entry.
	past := time.Now().Add(-1 * time.Hour)
	_, err := env.pool.Exec(ctx,
		`INSERT INTO allowed_emails (email, role, expires_at) VALUES ($1, 'player', $2)`,
		"expired@example.com", past,
	)
	if err != nil {
		t.Fatalf("insert expired email: %v", err)
	}

	allowed, err := env.store.IsEmailAllowed(ctx, "expired@example.com")
	if err != nil {
		t.Fatalf("IsEmailAllowed: %v", err)
	}
	if allowed {
		t.Error("expected expired email to not be allowed")
	}
}

func TestUpsertOAuthIdentityFirstLogin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedAllowedEmail(t, "bob@example.com", "player")

	avatar := "https://example.com/avatar.png"
	identity, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-123",
		Email:      "bob@example.com",
		AvatarURL:  &avatar,
		Username:   "bob",
	})
	if err != nil {
		t.Fatalf("UpsertOAuthIdentity: %v", err)
	}

	if identity.PlayerID == uuid.Nil {
		t.Error("expected non-nil player ID")
	}

	// Verify the player was created.
	player, err := env.store.GetPlayer(ctx, identity.PlayerID)
	if err != nil {
		t.Fatalf("GetPlayer: %v", err)
	}
	if player.Username != "bob" {
		t.Errorf("expected username bob, got %s", player.Username)
	}
	if player.Role != "player" {
		t.Errorf("expected role player, got %s", player.Role)
	}
}

func TestUpsertOAuthIdentitySubsequentLogin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedAllowedEmail(t, "carol@example.com", "player")

	avatar1 := "https://example.com/old.png"
	first, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-456",
		Email:      "carol@example.com",
		AvatarURL:  &avatar1,
		Username:   "carol",
	})
	if err != nil {
		t.Fatalf("first login: %v", err)
	}

	// Second login with updated avatar.
	avatar2 := "https://example.com/new.png"
	second, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-456",
		Email:      "carol@example.com",
		AvatarURL:  &avatar2,
		Username:   "carol",
	})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	if second.PlayerID != first.PlayerID {
		t.Error("expected same player ID on subsequent login")
	}
}

func TestUpsertOAuthIdentityOwnerRole(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedAllowedEmail(t, "admin@example.com", "owner")

	identity, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-admin",
		Email:      "admin@example.com",
		Username:   "admin",
	})
	if err != nil {
		t.Fatalf("UpsertOAuthIdentity: %v", err)
	}

	player, _ := env.store.GetPlayer(ctx, identity.PlayerID)
	if player.Role != "owner" {
		t.Errorf("expected role owner, got %s", player.Role)
	}
}

func TestCreateSession(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.seedAllowedEmail(t, "dave@example.com", "player")

	identity, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-789",
		Email:      "dave@example.com",
		Username:   "dave",
	})
	if err != nil {
		t.Fatalf("setup player: %v", err)
	}

	_, err = env.store.CreateSession(ctx, handler.CreateSessionParams{
		PlayerID:       identity.PlayerID,
		UserAgent:      "TestAgent/1.0",
		AcceptLanguage: "en-US",
		IPAddress:      "127.0.0.1",
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Verify session exists.
	var count int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM player_sessions WHERE player_id = $1`,
		identity.PlayerID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestGetPlayerDeletedReturnsError(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// Create a player then soft-delete.
	env.seedAllowedEmail(t, "ghost@example.com", "player")

	identity, err := env.store.UpsertOAuthIdentity(ctx, handler.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: "gh-ghost",
		Email:      "ghost@example.com",
		Username:   "ghost",
	})
	if err != nil {
		t.Fatalf("setup player: %v", err)
	}

	_, err = env.pool.Exec(ctx,
		`UPDATE players SET deleted_at = NOW() WHERE id = $1`,
		identity.PlayerID,
	)
	if err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	_, err = env.store.GetPlayer(ctx, identity.PlayerID)
	if err == nil {
		t.Error("expected error for deleted player")
	}
}
