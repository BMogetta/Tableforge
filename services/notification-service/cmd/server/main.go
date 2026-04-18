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
	"github.com/recess/shared/featureflags"
	sharedmw "github.com/recess/shared/middleware"
	userv1 "github.com/recess/shared/proto/user/v1"
	sharedredis "github.com/recess/shared/redis"
	"github.com/recess/shared/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// ── user-service gRPC client ──────────────────────────────────────────────
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

	// ── Wire ──────────────────────────────────────────────────────────────────
	log := slog.Default()
	pub := publisher.New(rdb, log)
	executor := api.NewGRPCExecutor(userClient)
	handler := api.New(st, pub, executor, log)
	cons := consumer.New(rdb, st, pub, log)

	// ── Feature flags ─────────────────────────────────────────────────────────
	flags, err := featureflags.Init(config.LoadUnleash(serviceName))
	if err != nil {
		slog.Warn("feature flags init failed, using defaults", "error", err)
	}
	defer func() { _ = flags.Close() }()

	// ── HTTP ──────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(sharedmw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Use(sharedmw.MaxBodySize(16 << 10)) // 16 KB
	r.Use(sharedmw.Maintenance(flags))

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
