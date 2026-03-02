package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tableforge/server/internal/store"
)

// seed-test creates two test players for Playwright and prints their IDs.
// Run with TEST_MODE=true before running Playwright tests.
//
// Usage:
//   DATABASE_URL=... go run ./cmd/seed-test
//
// Output is a JSON object with player1_id and player2_id.
// Pass these as TEST_PLAYER1_ID and TEST_PLAYER2_ID env vars to Playwright.

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

	// TODO add more users to check other integrations and roles
	// We nee to test spectators, managers, admin, friends, chats, etc.

	// Also add both emails to allowed_emails so they can log in via test-login.
	for _, email := range []string{"test1@tableforge.test", "test2@tableforge.test"} {
		if _, err := st.AddAllowedEmail(ctx, store.AddAllowedEmailParams{
			Email: email,
			Role:  store.RolePlayer,
		}); err != nil {
			log.Printf("warn: add allowed email %s: %v", email, err)
		}
	}

	out, _ := json.MarshalIndent(map[string]string{
		"player1_id": p1.ID.String(),
		"player2_id": p2.ID.String(),
	}, "", "  ")
	fmt.Println(string(out))
}
