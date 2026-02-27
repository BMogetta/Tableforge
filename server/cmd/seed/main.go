// cmd/seed/main.go
// Runs once: if allowed_emails is empty, inserts OWNER_EMAIL.
// Safe to run on every deploy — no-ops if the table already has rows.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ownerEmail := os.Getenv("OWNER_EMAIL")
	if ownerEmail == "" {
		log.Fatal("OWNER_EMAIL is not set")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Check if the table already has any rows.
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM allowed_emails`).Scan(&count)
	if err != nil {
		log.Fatalf("query: %v", err)
	}

	if count > 0 {
		log.Printf("allowed_emails already has %d row(s), skipping seed", count)
		return
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO allowed_emails (email, note) VALUES ($1, $2)
		 ON CONFLICT (email) DO NOTHING`,
		ownerEmail, "owner — seeded automatically",
	)
	if err != nil {
		log.Fatalf("insert: %v", err)
	}

	log.Printf("seeded owner email: %s", ownerEmail)
}
