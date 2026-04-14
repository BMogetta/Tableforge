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
	"github.com/recess/auth-service/internal/consumer"
	"github.com/recess/auth-service/internal/handler"
	"github.com/recess/auth-service/internal/store"
	"github.com/recess/shared/config"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	"github.com/recess/shared/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "auth-service")
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
	clientID := config.MustEnv("GITHUB_CLIENT_ID")
	clientSecret := config.MustEnv("GITHUB_CLIENT_SECRET")
	secure := config.Env("ENV", "production") != "development"

	st, err := store.New(ctx, config.MustEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		panic(err)
	}
	defer st.Close()

	// --- Redis ---------------------------------------------------------------
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer rdb.Close()

	cons := consumer.New(rdb, slog.Default(), st)
	consErr := make(chan error, 1)
	go func() {
		consErr <- cons.Run(ctx)
	}()

	h := handler.New(st, clientID, clientSecret, jwtSecret, secure)
	authMW := sharedmw.Require([]byte(jwtSecret))

	r := chi.NewRouter()
	r.Use(sharedmw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Use(sharedmw.MaxBodySize(16 << 10)) // 16 KB

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/auth", func(r chi.Router) {
		r.Get("/github", h.HandleGitHubLogin)
		r.Get("/github/callback", h.HandleGitHubCallback)
		r.Post("/refresh", h.HandleRefresh) // No auth middleware — access token may be expired
		r.With(authMW).Post("/logout", h.HandleLogout)
		r.With(authMW).Get("/me", h.HandleMe)

		if handler.TestModeEnabled() {
			r.Get("/test-login", h.HandleTestLogin)
		}
		// Bot login is always mounted. The handler itself returns 401 when
		// BOT_SERVICE_SECRET is unset, so forgetting to configure the secret
		// is a safe default (endpoint refuses everything).
		r.Post("/bot-login", h.HandleBotLogin)
	})

	addr := config.Env("ADDR", ":8081")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("auth-service listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			panic(err)
		}
	}()

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
