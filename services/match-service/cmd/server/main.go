package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/recess/match-service/internal/api"
	"github.com/recess/match-service/internal/consumer"
	"github.com/recess/match-service/internal/queue"
	"github.com/recess/shared/config"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	gamev1 "github.com/recess/shared/proto/game/v1"
	lobbyv1 "github.com/recess/shared/proto/lobby/v1"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"github.com/recess/shared/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "match-service")
	otlpEndpoint := config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	shutdownTelemetry, err := telemetry.Setup(ctx, serviceName, otlpEndpoint)
	if err != nil {
		slog.Warn("telemetry setup failed, continuing without it", "error", err)
		shutdownTelemetry = func(_ context.Context) error { return nil }
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown error", "error", err)
		}
	}()

	jwtSecret := config.MustEnv("JWT_SECRET")

	// --- Redis ---------------------------------------------------------------
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer rdb.Close()

	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	// --- rating-service gRPC client ------------------------------------------
	ratingAddr := config.Env("RATING_SERVICE_ADDR", "rating-service:9085")
	ratingConn, err := grpc.NewClient(ratingAddr, grpcOpts...)
	if err != nil {
		slog.Error("failed to connect to rating-service", "error", err)
		panic(err)
	}
	defer ratingConn.Close()
	ratingClient := ratingv1.NewRatingServiceClient(ratingConn)
	slog.Info("rating-service gRPC connected", "addr", ratingAddr)

	// --- game-server lobby gRPC client ----------------------------------------
	gameServerAddr := config.Env("GAME_SERVER_ADDR", "game-server:9080")
	gameConn, err := grpc.NewClient(gameServerAddr, grpcOpts...)
	if err != nil {
		slog.Error("failed to connect to game-server", "error", err)
		panic(err)
	}
	defer gameConn.Close()
	lobbyClient := lobbyv1.NewLobbyServiceClient(gameConn)
	gameClient := gamev1.NewGameServiceClient(gameConn)
	slog.Info("game-server gRPC connected", "addr", gameServerAddr)

	// --- Asynq (match expiry tasks) ------------------------------------------
	redisURL := config.MustEnv("REDIS_URL")
	redisAddr, redisPass := parseRedisOpt(redisURL)
	asynqRedis := asynq.RedisClientOpt{Addr: redisAddr, Password: redisPass}

	asynqClient := asynq.NewClient(asynqRedis)
	defer asynqClient.Close()
	asynqInspector := asynq.NewInspector(asynqRedis)
	defer asynqInspector.Close()

	// --- Queue service -------------------------------------------------------
	rankedGameID := config.Env("RANKED_GAME_ID", queue.DefaultRankedGameID)
	var queueOpts []queue.Option
	if v := config.Env("MATCHMAKER_SPREAD_PER_SEC", ""); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			queueOpts = append(queueOpts, queue.WithSpreadPerSecond(f))
		}
	}
	queueSvc := queue.New(rdb, ratingClient, lobbyClient, gameClient, rankedGameID, asynqClient, asynqInspector, queueOpts...)

	// Backfill detector — when enabled, injects idle bots into queue for
	// lonely humans. See queue/backfill.go for the full design and the
	// note on the Asynq-based alternative we did not take.
	queueSvc.SetBackfillConfig(loadBackfillConfig())

	// --- Asynq server (match expiry handler) ---------------------------------
	asynqSrv := asynq.NewServer(asynqRedis, asynq.Config{
		Concurrency: 5,
		Queues:      map[string]int{"match": 1},
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeMatchExpiry, queueSvc.HandleMatchExpiry)
	go func() {
		if err := asynqSrv.Run(mux); err != nil {
			slog.Error("asynq server error", "error", err)
		}
	}()

	// --- Event consumer (player.banned) --------------------------------------
	cons := consumer.New(rdb, queueSvc, slog.Default())
	consErr := make(chan error, 1)
	go func() {
		consErr <- cons.Run(ctx)
	}()

	tickInterval := 5 * time.Second
	if v := config.Env("MATCHMAKER_TICK_INTERVAL", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			tickInterval = d
		}
	}
	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		slog.Info("matchmaker ticker started", "interval", tickInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				queueSvc.FindAndPropose(ctx)
				// Runs on every tick; no-op when BACKFILL_ENABLED is false.
				queueSvc.BackfillScan(ctx)
			}
		}
	}()

	// --- Router --------------------------------------------------------------
	authMW := sharedmw.Require([]byte(jwtSecret))

	r := chi.NewRouter()
	r.Use(sharedmw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Use(sharedmw.MaxBodySize(16 << 10)) // 16 KB
	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	schemaReg, err := sharedmw.NewSchemaRegistry()
	if err != nil {
		slog.Error("failed to compile JSON schemas", "error", err)
		return
	}

	r.Mount("/", api.NewRouter(queueSvc, authMW, schemaReg))

	// --- HTTP server ---------------------------------------------------------
	addr := config.Env("HTTP_ADDR", ":8087")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("match-service listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			panic(err)
		}
	}()

	// --- Graceful shutdown ---------------------------------------------------
	select {
	case <-ctx.Done():
		slog.Info("shutting down match-service...")
	case err := <-consErr:
		if err != nil {
			slog.Error("consumer exited", "error", err)
		}
	}

	asynqSrv.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
}

// loadBackfillConfig reads BACKFILL_* env vars into a BackfillConfig.
// Unknown / malformed values fall back to defaults so a misconfigured env
// never bricks the service — at worst backfill stays disabled.
func loadBackfillConfig() queue.BackfillConfig {
	cfg := queue.DefaultBackfillConfig()
	if v := config.Env("BACKFILL_ENABLED", ""); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Enabled = b
		}
	}
	if v := config.Env("BACKFILL_THRESHOLD_SECS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Threshold = time.Duration(n) * time.Second
		}
	}
	if v := config.Env("BACKFILL_MAX_ACTIVE", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxActive = n
		}
	}
	slog.Info("backfill config",
		"enabled", cfg.Enabled,
		"threshold", cfg.Threshold,
		"max_active", cfg.MaxActive,
	)
	return cfg
}

func parseRedisOpt(rawURL string) (addr, password string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "localhost:6379", ""
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "6379"
	}
	if u.User != nil {
		password, _ = u.User.Password()
	}
	return host + ":" + port, password
}
