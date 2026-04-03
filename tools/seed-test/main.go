// seed-test creates test players and seed data for Playwright e2e tests.
//
// Usage:
//
//	DATABASE_URL=... go run .
//
// Output: JSON with player IDs for all test players.
//
// Players are created in pairs — each Playwright project gets its own pair
// so tests can run in parallel without sharing game state.
//
//	Pair 1 (players 1–2):  game-tests, session-history-tests, auth
//	Pair 2 (players 3–4):  chat-tests
//	Pair 3 (players 5–6):  settings-tests
//	Pair 4 (players 7–8):  spectator-tests (+ player 9 as spectator)
//	Pair 5 (players 10–11): presence-tests
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

const playerCount = 11

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
		ids[i] = createPlayer(ctx, conn, fmt.Sprintf("test_player_%d", i+1))
	}

	// Seed ranked ratings for pair 1 (used by game-tests).
	seedRating(ctx, conn, ids[0], 1536.0, 1, 1, 0)
	seedRating(ctx, conn, ids[1], 1464.0, 1, 0, 1)

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
