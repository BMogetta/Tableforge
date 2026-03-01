package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tableforge/server/games"
	"github.com/tableforge/server/internal/api"
	"github.com/tableforge/server/internal/auth"
	"github.com/tableforge/server/internal/lobby"
	"github.com/tableforge/server/internal/ratelimit"
	"github.com/tableforge/server/internal/runtime"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/telemetry"
	"github.com/tableforge/server/internal/ws"

	_ "github.com/tableforge/server/games/tictactoe"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Telemetry
	otlpEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	serviceName := getEnv("OTEL_SERVICE_NAME", "game-server")

	shutdownTelemetry, err := telemetry.Setup(ctx, serviceName, otlpEndpoint)
	if err != nil {
		log.Printf("telemetry setup failed, continuing without it: %v", err)
		shutdownTelemetry = func(_ context.Context) error { return nil }
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Printf("telemetry shutdown error: %v", err)
		}
	}()

	// Database
	dbURL := getEnv("DATABASE_URL", "postgres://tableforge:tableforge@localhost:5432/tableforge?sslmode=disable")
	st, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer st.Close()

	// Redis
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	rdb, err := connectRedis(ctx, redisURL)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	// Services
	reg := games.DefaultRegistry()
	lobbyService := lobby.New(st, reg)
	runtimeService := runtime.New(st, reg)
	hub := ws.NewHubWithRedis(rdb)

	turnTimer := runtime.NewTurnTimer(runtimeService, hub, st)
	runtimeService.SetTimer(turnTimer)

	// Rate limiter: 100 req/min per IP
	limiter := ratelimit.New(rdb, 100, time.Minute)

	// Auth
	authHandler := auth.NewHandler(
		st,
		getEnv("GITHUB_CLIENT_ID", ""),
		getEnv("GITHUB_CLIENT_SECRET", ""),
		getEnv("JWT_SECRET", "change-me-in-production"),
		getEnv("ENV", "development") == "production",
	)

	// HTTP server
	router := api.NewRouter(lobbyService, runtimeService, st, hub, authHandler, limiter)
	addr := getEnv("ADDR", ":8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: limiter.Middleware(router),
	}

	go func() {
		log.Printf("server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
}

func connectRedis(ctx context.Context, rawURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	log.Println("redis connected")
	return rdb, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
