package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
	"github.com/recess/notification-service/internal/api"
	"github.com/recess/notification-service/internal/consumer"
	"github.com/recess/notification-service/internal/publisher"
	"github.com/recess/notification-service/internal/store"
	"github.com/recess/shared/config"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	"github.com/recess/shared/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "notification-service")
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

	// ── Store ─────────────────────────────────────────────────────────────────
	st, err := store.New(ctx, config.MustEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		panic(err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer rdb.Close()

	// ── Wire ──────────────────────────────────────────────────────────────────
	log := slog.Default()
	pub := publisher.New(rdb, log)
	handler := api.New(st, pub, log)
	cons := consumer.New(rdb, st, pub, log)

	// ── HTTP ──────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(sharedmw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		r.Use(sharedmw.Require([]byte(config.MustEnv("JWT_SECRET"))))
		handler.RegisterRoutes(r)
	})

	addr := config.Env("HTTP_ADDR", ":8086")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("notification-service listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			panic(err)
		}
	}()

	// ── Consumer ──────────────────────────────────────────────────────────────
	consErr := make(chan error, 1)
	go func() {
		slog.Info("starting event consumer")
		consErr <- cons.Run(ctx)
	}()

	// ── Shutdown ──────────────────────────────────────────────────────────────
	select {
	case <-ctx.Done():
		slog.Info("shutting down...")
	case err := <-consErr:
		if err != nil {
			slog.Error("consumer exited", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown error", "error", err)
	}
}
