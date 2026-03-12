package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tableforge/server/internal/platform/store"
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

	// Add all emails to allowed_emails so they can log in via test-login.
	allowedEmails := []struct {
		email string
		role  store.PlayerRole
	}{
		{"test1@tableforge.test", store.RolePlayer},
		{"test2@tableforge.test", store.RolePlayer},
		{"test3@tableforge.test", store.RolePlayer},
	}
	for _, e := range allowedEmails {
		if _, err := st.AddAllowedEmail(ctx, store.AddAllowedEmailParams{
			Email: e.email,
			Role:  e.role,
		}); err != nil {
			log.Printf("warn: add allowed email %s: %v", e.email, err)
		}
	}

	out, _ := json.MarshalIndent(map[string]string{
		"player1_id": p1.ID.String(),
		"player2_id": p2.ID.String(),
		"player3_id": p3.ID.String(),
	}, "", "  ")
	fmt.Println(string(out))
}
