// Command bot-runner plays ranked matches as an external API client, reusing
// the MCTS engine and adapters from game-server. It is intended for
// development, benchmarking, and — in later phases — production backfill when
// a human is waiting alone in queue.
//
// Usage (Phase 1, dev):
//
// The --bots flag accepts either username:profile or uuid:profile pairs.
// UUIDs skip the DB lookup, which is what you want when postgres is not
// reachable from the host (compose's data_network is internal by default).
// Grab the UUIDs with:
//
//	docker exec postgres psql -U recess -d recess -tAc \
//	    "SELECT username||':'||id FROM players WHERE is_bot = TRUE ORDER BY username"
//
// Then:
//
//	go run ./cmd/bot-runner \
//	    --base-url http://localhost \
//	    --game-id rootaccess \
//	    --bots <easy-uuid>:easy,<hard-uuid>:hard \
//	    --games 10
//
// Username form (requires host-reachable postgres via DATABASE_URL):
//
//	export DATABASE_URL=postgres://recess:recess@localhost:5432/recess
//	go run ./cmd/bot-runner ... --bots bot_easy_1:easy,bot_hard_1:hard ...
//
// Requirements for Phase 1:
//   - auth-service running with TEST_MODE=true (/auth/test-login enabled)
//   - seed-test executed — the bot accounts must exist with is_bot=TRUE
//   - match-service + game-server + ws-gateway reachable via baseURL
//
// Usage (Phase 3, backfill mode):
//
// In backfill mode the bots do NOT self-queue. They register in Redis
// (bot:known + bot:available), subscribe to the bot.activate channel, and
// wait for match-service to pick them when a lone human has been waiting
// past BACKFILL_THRESHOLD_SECS. match-service must run with
// BACKFILL_ENABLED=true. Redis must be reachable from the host.
//
//	export REDIS_URL=redis://:recess@localhost:6379
//	go run ./cmd/bot-runner \
//	    --mode backfill \
//	    --base-url http://localhost \
//	    --game-id rootaccess \
//	    --bots <easy-uuid>:easy,<medium-uuid>:medium,<hard-uuid>:hard
//
// --games is ignored in backfill mode — bots stay resident until SIGINT.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/recess/game-server/cmd/bot-runner/internal/client"
	"github.com/recess/game-server/cmd/bot-runner/internal/runner"
)

type botSpec struct {
	// Exactly one of username / id is set at parse time. If the identifier
	// in --bots parses as a UUID the id is populated and the DB lookup is
	// skipped; otherwise the username is populated and id is filled later
	// by resolveBotIDs.
	username string
	id       uuid.UUID
	profile  string
}

func main() {
	var (
		baseURL  = flag.String("base-url", "http://localhost", "HTTP origin for API + WS (through Traefik)")
		gameID   = flag.String("game-id", "rootaccess", "game to queue for (rootaccess | tictactoe)")
		botsFlag = flag.String("bots", "", "comma-separated username:profile pairs, e.g. bot_easy_1:easy,bot_hard_1:hard")
		numGames = flag.Int("games", 1, "games per bot (0 = unbounded until SIGINT); ignored in backfill mode")
		mode     = flag.String("mode", "autonomous", "autonomous = self-queue Phase 1 behavior; backfill = wait for match-service activation on Redis channel bot.activate")
		redisURL = flag.String("redis-url", "", "Redis URL (backfill mode only); falls back to $REDIS_URL")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	specs, err := parseBots(*botsFlag)
	if err != nil {
		log.Error("parse --bots", "error", err)
		os.Exit(2)
	}
	if len(specs) == 0 {
		log.Error("at least one bot required, see --bots")
		os.Exit(2)
	}

	if *mode != "autonomous" && *mode != "backfill" {
		log.Error("--mode must be 'autonomous' or 'backfill'", "got", *mode)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Backfill mode needs a shared Redis client so every bot can write to
	// bot:known/bot:available and subscribe to bot.activate.
	var rdb *redis.Client
	if *mode == "backfill" {
		url := *redisURL
		if url == "" {
			url = os.Getenv("REDIS_URL")
		}
		if url == "" {
			log.Error("backfill mode requires --redis-url or $REDIS_URL")
			os.Exit(2)
		}
		opt, err := redis.ParseURL(url)
		if err != nil {
			log.Error("parse redis url", "error", err)
			os.Exit(2)
		}
		rdb = redis.NewClient(opt)
		defer rdb.Close()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Error("redis ping failed", "error", err)
			os.Exit(1)
		}
	}

	// Resolve any specs that carry a username into UUIDs via DB lookup.
	// Specs already carrying a UUID (identifier==uuid parsed successfully)
	// skip the lookup. DATABASE_URL is only required when at least one spec
	// is username-form.
	needLookup := false
	for _, s := range specs {
		if s.id == uuid.Nil {
			needLookup = true
			break
		}
	}
	if needLookup {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			log.Error("DATABASE_URL required to resolve usernames; pass UUIDs in --bots to skip the lookup")
			os.Exit(2)
		}
		resolved, err := resolveBotIDs(ctx, dbURL, specs)
		if err != nil {
			log.Error("resolve bot IDs", "error", err)
			os.Exit(1)
		}
		for i := range specs {
			if specs[i].id == uuid.Nil {
				id, ok := resolved[specs[i].username]
				if !ok {
					log.Error("bot not found in DB (is_bot=TRUE filter)", "username", specs[i].username)
					specs[i].id = uuid.Nil
					continue
				}
				specs[i].id = id
			}
		}
	}

	var wg sync.WaitGroup
	for _, spec := range specs {
		if spec.id == uuid.Nil {
			continue // unresolved, already logged
		}
		id := spec.id
		label := spec.username
		if label == "" {
			label = id.String()[:8]
		}
		c, err := client.New(*baseURL, id, client.Options{
			BotSecret: os.Getenv("BOT_SERVICE_SECRET"),
		})
		if err != nil {
			log.Error("client init", "bot", label, "error", err)
			continue
		}
		r, err := runner.New(log, label, id, *gameID, spec.profile, c)
		if err != nil {
			log.Error("runner init", "bot", label, "error", err)
			continue
		}

		wg.Add(1)
		go func(label string) {
			defer wg.Done()
			var err error
			switch *mode {
			case "backfill":
				err = r.RunBackfill(ctx, rdb)
			default:
				err = r.Run(ctx, *numGames)
			}
			if err != nil && ctx.Err() == nil {
				log.Error("runner exited with error", "bot", label, "error", err)
			}
		}(label)
	}
	wg.Wait()
}

// parseBots parses a comma-separated list of <id>:<profile> pairs where <id>
// is either a UUID (direct) or a bot username (to be resolved via DB).
func parseBots(s string) ([]botSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	specs := make([]botSpec, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid bot spec %q, expected id:profile", p)
		}
		ident := strings.TrimSpace(kv[0])
		profile := strings.TrimSpace(kv[1])
		if id, err := uuid.Parse(ident); err == nil {
			specs = append(specs, botSpec{id: id, profile: profile})
		} else {
			specs = append(specs, botSpec{username: ident, profile: profile})
		}
	}
	return specs, nil
}

// resolveBotIDs fetches UUIDs for the requested bot usernames from the
// players table. Silently skips rows that are not flagged is_bot=TRUE to
// avoid accidentally impersonating a human account whose username happens
// to collide with a bot slot.
func resolveBotIDs(ctx context.Context, dbURL string, specs []botSpec) (map[string]uuid.UUID, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("pg connect: %w", err)
	}
	defer conn.Close(ctx)

	usernames := make([]string, len(specs))
	for i, s := range specs {
		usernames[i] = s.username
	}

	rows, err := conn.Query(ctx,
		`SELECT username, id FROM players WHERE username = ANY($1) AND is_bot = TRUE`,
		usernames,
	)
	if err != nil {
		return nil, fmt.Errorf("query players: %w", err)
	}
	defer rows.Close()

	out := make(map[string]uuid.UUID, len(specs))
	for rows.Next() {
		var username string
		var id uuid.UUID
		if err := rows.Scan(&username, &id); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out[username] = id
	}
	return out, rows.Err()
}
