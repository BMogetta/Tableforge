package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/tableforge/match-service/internal/api"
	"github.com/tableforge/match-service/internal/consumer"
	"github.com/tableforge/match-service/internal/queue"
	"github.com/tableforge/shared/config"
	sharedmw "github.com/tableforge/shared/middleware"
	sharedredis "github.com/tableforge/shared/redis"
	gamev1 "github.com/tableforge/shared/proto/game/v1"
	lobbyv1 "github.com/tableforge/shared/proto/lobby/v1"
	ratingv1 "github.com/tableforge/shared/proto/rating/v1"
	"github.com/tableforge/shared/telemetry"
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

	// --- Queue service -------------------------------------------------------
	rankedGameID := config.Env("RANKED_GAME_ID", queue.DefaultRankedGameID)
	queueSvc := queue.New(rdb, ratingClient, lobbyClient, gameClient, rankedGameID)

	// --- Event consumer (player.banned) --------------------------------------
	cons := consumer.New(rdb, queueSvc, slog.Default())
	consErr := make(chan error, 1)
	go func() {
		consErr <- cons.Run(ctx)
	}()

	go queueSvc.ListenExpiry(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				queueSvc.FindAndPropose(ctx)
			}
		}
	}()

	// --- Router --------------------------------------------------------------
	authMW := sharedmw.Require([]byte(jwtSecret))

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	r.Mount("/", api.NewRouter(queueSvc, authMW))

	// --- HTTP server ---------------------------------------------------------
	addr := config.Env("HTTP_ADDR", ":8087")
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
}
