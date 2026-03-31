package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tableforge/game-server/internal/platform/store"
)

// seed-test creates test players for Playwright and prints their IDs.
// Run with TEST_MODE=true before running Playwright tests.
//
// Usage:
//
//	DATABASE_URL=... go run ./cmd/seed-test
//
// Output is a JSON object with player1_id, player2_id, and player3_id.
// Pass these as TEST_PLAYER1_ID, TEST_PLAYER2_ID, TEST_PLAYER3_ID env vars
// to Playwright.
//
// Players:
//   - player1: room owner / host in most tests
//   - player2: second participant
//   - player3: used for spectator tests — joins rooms without a seat
func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	st, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	p1, err := st.CreatePlayer(ctx, "test_player_1")
	if err != nil {
		log.Fatalf("create player 1: %v", err)
	}

	p2, err := st.CreatePlayer(ctx, "test_player_2")
	if err != nil {
		log.Fatalf("create player 2: %v", err)
	}

	p3, err := st.CreatePlayer(ctx, "test_player_3")
	if err != nil {
		log.Fatalf("create player 3: %v", err)
	}

	// Seed ranked ratings so the leaderboard test has data without requiring
	// a full ranked game flow.
	ratingSQL := `INSERT INTO ratings (player_id, game_id, mmr, display_rating, games_played, win_streak, loss_streak)
		 VALUES ('%s', 'tictactoe', %f, %f, %d, %d, %d)
		 ON CONFLICT (player_id, game_id) DO UPDATE
		   SET mmr = EXCLUDED.mmr, display_rating = EXCLUDED.display_rating,
		       games_played = EXCLUDED.games_played, win_streak = EXCLUDED.win_streak,
		       loss_streak = EXCLUDED.loss_streak`
	if err := st.Exec(ctx, fmt.Sprintf(ratingSQL, p1.ID, 1536.0, 1536.0, 1, 1, 0)); err != nil {
		log.Printf("warn: upsert rating p1: %v", err)
	}
	if err := st.Exec(ctx, fmt.Sprintf(ratingSQL, p2.ID, 1464.0, 1464.0, 1, 0, 1)); err != nil {
		log.Printf("warn: upsert rating p2: %v", err)
	}

	// Add all emails to allowed_emails so they can log in via test-login.
	for _, email := range []string{"test1@tableforge.test", "test2@tableforge.test", "test3@tableforge.test"} {
		sql := fmt.Sprintf(
			`INSERT INTO allowed_emails (email, role) VALUES ('%s', 'player') ON CONFLICT (email) DO NOTHING`,
			email,
		)
		if err := st.Exec(ctx, sql); err != nil {
			log.Printf("warn: add allowed email %s: %v", email, err)
		}
	}

	out, _ := json.MarshalIndent(map[string]string{
		"player1_id": p1.ID.String(),
		"player2_id": p2.ID.String(),
		"player3_id": p3.ID.String(),
	}, "", "  ")
	fmt.Println(string(out))
}
