package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsDir returns the absolute path to shared/db/migrations/.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "db", "migrations")
}

// Migration identifies a SQL migration file by name (e.g. "001_initial.sql").
type Migration string

const (
	MigrationInitial      Migration = "001_initial.sql"
	MigrationUserService  Migration = "002_user_service.sql"
	MigrationRating       Migration = "003_rating_service.sql"
	MigrationAdmin        Migration = "004_admin.sql"
	MigrationRebrand      Migration = "005_rebrand_rootaccess.sql"
	MigrationAchievements Migration = "006_achievements_update.sql"
	MigrationBotProfile   Migration = "007_bot_profile.sql"
	MigrationRatingUnique Migration = "008_rating_history_unique.sql"
)

// NewTestDB spins up a Postgres container, runs the requested migrations,
// and returns a connection string. The container is terminated when the test ends.
func NewTestDB(t *testing.T, migrations ...Migration) string {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("recess_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("testutil: failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("testutil: failed to terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("testutil: failed to get connection string: %v", err)
	}

	// Run migrations.
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testutil: failed to connect for migrations: %v", err)
	}
	defer pool.Close()

	dir := migrationsDir()
	for _, m := range migrations {
		sql, err := os.ReadFile(filepath.Join(dir, string(m)))
		if err != nil {
			t.Fatalf("testutil: failed to read migration %s: %v", m, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("testutil: failed to run migration %s: %v", m, err)
		}
	}

	return dsn
}
