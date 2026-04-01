// seed-test creates test players and seed data for Playwright e2e tests.
//
// Usage:
//
//	DATABASE_URL=... go run .
//
// Output: JSON with player1_id, player2_id, player3_id.
//
// Players:
//   - player1: room owner / host in most tests
//   - player2: second participant
//   - player3: used for spectator tests
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

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

	p1 := createPlayer(ctx, conn, "test_player_1")
	p2 := createPlayer(ctx, conn, "test_player_2")
	p3 := createPlayer(ctx, conn, "test_player_3")

	// Seed ranked ratings for leaderboard tests.
	seedRating(ctx, conn, p1, 1536.0, 1, 1, 0)
	seedRating(ctx, conn, p2, 1464.0, 1, 0, 1)

	// Add emails to allowed_emails for test-login.
	for _, email := range []string{"test1@recess.test", "test2@recess.test", "test3@recess.test"} {
		_, err := conn.Exec(ctx,
			`INSERT INTO allowed_emails (email, role) VALUES ($1, 'player') ON CONFLICT (email) DO NOTHING`,
			email,
		)
		if err != nil {
			log.Printf("warn: add allowed email %s: %v", email, err)
		}
	}

	out, _ := json.MarshalIndent(map[string]string{
		"player1_id": p1,
		"player2_id": p2,
		"player3_id": p3,
	}, "", "  ")
	fmt.Println(string(out))
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

func seedRating(ctx context.Context, conn *pgx.Conn, playerID string, rating float64, games, wins, losses int) {
	_, err := conn.Exec(ctx,
		`INSERT INTO ratings (player_id, game_id, mmr, display_rating, games_played, win_streak, loss_streak)
		 VALUES ($1, 'tictactoe', $2, $2, $3, $4, $5)
		 ON CONFLICT (player_id, game_id) DO UPDATE
		   SET mmr = EXCLUDED.mmr, display_rating = EXCLUDED.display_rating,
		       games_played = EXCLUDED.games_played, win_streak = EXCLUDED.win_streak,
		       loss_streak = EXCLUDED.loss_streak`,
		playerID, rating, games, wins, losses,
	)
	if err != nil {
		log.Printf("warn: upsert rating %s: %v", playerID, err)
	}
}
