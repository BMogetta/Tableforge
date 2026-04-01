package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/recess/services/chat-service/internal/api"
	"github.com/recess/services/chat-service/internal/store"
	"github.com/recess/shared/config"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	"github.com/recess/shared/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "chat-service")
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

	// --- Database ------------------------------------------------------------
	st, err := store.New(ctx, config.MustEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		panic(err)
	}
	defer st.Close()

	// --- Redis ---------------------------------------------------------------
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer rdb.Close()

	// --- HTTP server ---------------------------------------------------------
	authMW := sharedmw.Require([]byte(jwtSecret))
	pub := api.NewPublisher(rdb)
	router := api.NewRouter(st, pub, authMW, serviceName)

	addr := config.Env("HTTP_ADDR", ":8083")
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		slog.Info("chat-service listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			panic(err)
		}
	}()

	// --- Graceful shutdown ---------------------------------------------------
	<-ctx.Done()
	slog.Info("shutting down chat-service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
}
