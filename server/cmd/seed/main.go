package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tableforge/server/internal/platform/store"
)

const (
	waitTimeout  = 10 * time.Minute
	waitInterval = 5 * time.Second
)

func main() {
	ctx := context.Background()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ownerEmail := os.Getenv("OWNER_EMAIL")
	if ownerEmail == "" {
		log.Fatal("OWNER_EMAIL is required")
	}

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	// Add owner email to whitelist with owner role.
	_, err = st.AddAllowedEmail(ctx, store.AddAllowedEmailParams{
		Email: ownerEmail,
		Role:  store.RoleOwner,
	})
	if err != nil {
		log.Fatalf("add allowed email: %v", err)
	}
	fmt.Printf("seeder: added %s to allowed_emails as owner\n", ownerEmail)

	// Wait for the owner to log in and their player record to be created,
	// then promote them. Exits automatically once done.
	fmt.Printf("seeder: waiting for %s to log in (timeout: %s)...\n", ownerEmail, waitTimeout)

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		identity, err := st.GetOAuthIdentityByEmail(ctx, ownerEmail)
		if err == nil {
			// Player exists — promote to owner.
			if err := st.SetPlayerRole(ctx, identity.PlayerID, store.RoleOwner); err != nil {
				log.Fatalf("seeder: set owner role: %v", err)
			}
			fmt.Printf("seeder: promoted player %s to owner\n", identity.PlayerID)
			fmt.Println("seeder: done")
			return
		}

		fmt.Printf("seeder: player not found yet, retrying in %s...\n", waitInterval)
		time.Sleep(waitInterval)
	}

	log.Fatalf("seeder: timed out after %s waiting for %s to log in", waitTimeout, ownerEmail)
}
