package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/tableforge/services/ws-gateway/internal/consumer"
	"github.com/tableforge/services/ws-gateway/internal/handler"
	"github.com/tableforge/services/ws-gateway/internal/hub"
	"github.com/tableforge/services/ws-gateway/internal/presence"
	"github.com/tableforge/shared/config"
	sharedmw "github.com/tableforge/shared/middleware"
	gamev1 "github.com/tableforge/shared/proto/game/v1"
	userv1 "github.com/tableforge/shared/proto/user/v1"
	sharedredis "github.com/tableforge/shared/redis"
	"github.com/tableforge/shared/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "ws-gateway")
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

	// --- user-service gRPC client --------------------------------------------
	userServiceAddr := config.Env("USER_SERVICE_ADDR", "user-service:9082")
	userConn, err := grpc.NewClient(userServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("failed to connect to user-service", "error", err)
		panic(err)
	}
	defer userConn.Close()
	userClient := userv1.NewUserServiceClient(userConn)
	slog.Info("user-service gRPC connected", "addr", userServiceAddr)

	// --- game-server gRPC client (IsParticipant) -----------------------------
	gameServerAddr := config.Env("GAME_SERVER_ADDR", "game-server:9080")
	gameConn, err := grpc.NewClient(gameServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("failed to connect to game-server", "error", err)
		panic(err)
	}
	defer gameConn.Close()
	gameClient := gamev1.NewGameServiceClient(gameConn)
	slog.Info("game-server gRPC connected", "addr", gameServerAddr)

	// --- Hub + presence ------------------------------------------------------
	h := hub.New(rdb)
	ps := presence.New(rdb)

	// --- Event consumer (player.session.revoked) ------------------------------
	cons := consumer.New(rdb, h, slog.Default())
	consErr := make(chan error, 1)
	go func() {
		consErr <- cons.Run(ctx)
	}()

	// --- Router --------------------------------------------------------------
	authMW := sharedmw.Require([]byte(jwtSecret))

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	r.With(authMW).Get("/ws/rooms/{roomID}", handler.RoomHandler(h, ps, userClient, gameClient))
	r.With(authMW).Get("/ws/players/{playerID}", handler.PlayerHandler(h, userClient))

	// --- HTTP server ---------------------------------------------------------
	addr := config.Env("HTTP_ADDR", ":8084")
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("ws-gateway listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			panic(err)
		}
	}()

	// --- Graceful shutdown ---------------------------------------------------
	select {
	case <-ctx.Done():
		slog.Info("shutting down ws-gateway...")
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
