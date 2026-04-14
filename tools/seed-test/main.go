// seed-test creates a pool of test players for Playwright e2e tests.
//
// Usage:
//
//	DATABASE_URL=... go run .
//
// Output: JSON with player IDs for all test players.
//
// Players are allocated dynamically at runtime — each test acquires players
// from the pool and releases them when done, enabling full parallelism.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

const (
	playerCount      = 30
	adminPlayerIndex = 28 // 1-based; reserved for admin e2e tests
)

// botSlot describes a bot account seeded at a specific 1-based pool index.
// profile must match the enum values allowed by the players_bot_profile_check
// CHECK constraint (see migration 007).
type botSlot struct {
	username string
	profile  string
}

// botSlots maps 1-based pool indices to bot metadata. These slots are marked
// is_bot=true and bot_profile=<profile> in the players table and used by
// tools/bot-runner. Indices must match BOT_RESERVED_INDICES in
// frontend/tests/e2e/player-pool.ts.
var botSlots = map[int]botSlot{
	24: {"bot_easy_1", "easy"},
	25: {"bot_medium_1", "medium"},
	26: {"bot_hard_1", "hard"},
	27: {"bot_aggressive_1", "aggressive"},
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	ids := make([]string, playerCount)
	for i := range playerCount {
		username := fmt.Sprintf("test_player_%d", i+1)
		if bot, ok := botSlots[i+1]; ok {
			username = bot.username
		}
		ids[i] = createPlayer(ctx, conn, username)
	}

	// Flag bot slots so game-server / user-service / bot-runner can identify
	// them, and stamp the difficulty profile so UI badges and future param
	// lookups can read it without parsing the username.
	for idx, bot := range botSlots {
		if idx < 1 || idx > playerCount {
			continue
		}
		if _, err := conn.Exec(ctx,
			`UPDATE players SET is_bot = TRUE, bot_profile = $2 WHERE id = $1`,
			ids[idx-1], bot.profile); err != nil {
			log.Printf("warn: flag bot slot %d: %v", idx, err)
		}
	}

	// Add emails to allowed_emails for test-login.
	for i := range playerCount {
		email := fmt.Sprintf("test%d@recess.test", i+1)
		_, err := conn.Exec(ctx,
			`INSERT INTO allowed_emails (email, role) VALUES ($1, 'player') ON CONFLICT (email) DO NOTHING`,
			email,
		)
		if err != nil {
			log.Printf("warn: add allowed email %s: %v", email, err)
		}
	}

	// Clean up stale rooms and sessions from previous test runs so players
	// don't get stuck in "active game" state.
	for _, id := range ids {
		// Mark active sessions as finished so the player is free.
		_, _ = conn.Exec(ctx,
			`UPDATE game_sessions SET finished_at = NOW(), deleted_at = NOW()
			 WHERE room_id IN (SELECT room_id FROM room_players WHERE player_id = $1)
			   AND finished_at IS NULL`, id)
		// Remove player from all rooms.
		_, _ = conn.Exec(ctx,
			`DELETE FROM room_players WHERE player_id = $1`, id)
	}
	// Delete rooms with no players left.
	_, _ = conn.Exec(ctx,
		`DELETE FROM rooms WHERE id NOT IN (SELECT DISTINCT room_id FROM room_players)`)

	// Lower turn timeouts so e2e tests don't wait 30s for a timeout.
	_, err = conn.Exec(ctx,
		`UPDATE game_configs SET default_timeout_secs = 5, min_timeout_secs = 5`)
	if err != nil {
		log.Printf("warn: update game_configs timeouts: %v", err)
	}

	// Promote the dedicated admin slot so admin e2e tests have a caller with
	// the manager role. All other players keep the default 'player' role.
	if adminPlayerIndex >= 1 && adminPlayerIndex <= playerCount {
		if _, err := conn.Exec(ctx,
			`UPDATE players SET role = 'manager' WHERE id = $1`, ids[adminPlayerIndex-1]); err != nil {
			log.Printf("warn: promote admin player: %v", err)
		}
	}

	out := make(map[string]string, playerCount)
	for i, id := range ids {
		out[fmt.Sprintf("player%d_id", i+1)] = id
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

func createPlayer(ctx context.Context, conn *pgx.Conn, username string) string {
	var id string
	err := conn.QueryRow(ctx,
		`INSERT INTO players (username) VALUES ($1)
		 ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
		 RETURNING id`,
		username,
	).Scan(&id)
	if err != nil {
		log.Fatalf("create player %s: %v", username, err)
	}
	return id
}
