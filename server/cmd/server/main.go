// @title           Tableforge API
// @version         1.0
// @host            localhost
// @BasePath        /api/v1
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tableforge/server/games"
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/domain/notification"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/api"
	"github.com/tableforge/server/internal/platform/events"
	"github.com/tableforge/server/internal/platform/queue"
	"github.com/tableforge/server/internal/platform/ratelimit"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/userclient"

	"github.com/tableforge/server/internal/platform/ws"
	"github.com/tableforge/shared/config"
	sharedredis "github.com/tableforge/shared/redis"
	"github.com/tableforge/shared/telemetry"

	_ "github.com/tableforge/server/games/loveletter"
	_ "github.com/tableforge/server/games/tictactoe"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "game-server")
	otlpEndpoint := config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	userServiceAddr := config.Env("USER_SERVICE_ADDR", "user-service:9082")

	shutdownTelemetry, err := telemetry.Setup(ctx, serviceName, otlpEndpoint)
	if err != nil {
		slog.Warn("telemetry setup failed, continuing without it", "error", err)
		shutdownTelemetry = func(_ context.Context) error { return nil }
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Warn("telemetry shutdown error", "error", err)
		}
	}()

	st, err := store.New(ctx, config.Env("DATABASE_URL", "postgres://tableforge:tableforge@localhost:5432/tableforge?sslmode=disable"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	rdb := sharedredis.Connect(ctx, config.Env("REDIS_URL", "redis://localhost:6379"))
	defer rdb.Close()

	userClient, err := userclient.New(userServiceAddr)
	if err != nil {
		slog.Error("failed to connect to user-service", "error", err)
		os.Exit(1)
	}
	defer userClient.Close()
	slog.Info("user-service gRPC connected", "addr", userServiceAddr)

	eventStore := events.New(rdb, st)

	reg := games.DefaultRegistry()
	lobbyService := lobby.New(st, reg)
	runtimeService := runtime.New(st, reg, eventStore, rdb, slog.Default())
	hub := ws.NewHubWithRedis(rdb)

	turnTimer := runtime.NewTurnTimer(runtimeService, hub, st, rdb, eventStore)
	runtimeService.SetTimer(turnTimer)

	queueSvc := queue.New(rdb, st, lobbyService, runtimeService, hub)
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

	turnTimer.ReschedulePending(ctx)
	go turnTimer.Start(ctx)

	testMode := config.Env("TEST_MODE", "false") == "true"

	var limiter *ratelimit.Limiter
	if !testMode {
		limiter = ratelimit.New(rdb, 100, time.Minute)
	}

	var jwtSecret []byte
	if !testMode {
		jwtSecret = []byte(config.MustEnv("JWT_SECRET"))
	}

	notificationSvc := notification.New(st, hub)

	router := api.NewRouter(
		lobbyService,
		runtimeService,
		st,
		hub,
		jwtSecret,
		limiter,
		eventStore,
		queueSvc,
		notificationSvc,
		userClient,
	)

	var handler http.Handler = router
	if limiter != nil {
		handler = limiter.Middleware(router)
	}

	addr := config.Env("ADDR", ":8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		slog.Info("game-server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("http shutdown error", "error", err)
	}
}
