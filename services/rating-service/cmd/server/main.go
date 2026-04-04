package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/recess/rating-service/internal/consumer"
	grpchandler "github.com/recess/rating-service/internal/grpc"
	httphandler "github.com/recess/rating-service/internal/handler"
	"github.com/recess/rating-service/internal/service"
	"github.com/recess/rating-service/internal/store"
	"github.com/recess/shared/config"
	"github.com/recess/shared/domain/rating"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"github.com/recess/shared/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "rating-service")
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

	// ── Store (Postgres via pgxpool) ──────────────────────────────────────────
	st, err := store.New(ctx, config.MustEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		panic(err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer rdb.Close()

	// ── Wire ──────────────────────────────────────────────────────────────────
	engine := rating.NewDefaultEngine()
	svc := service.New(st, engine, slog.Default())
	cons := consumer.New(rdb, svc, slog.Default())
	grpcH := grpchandler.New(st)

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcAddr := config.Env("GRPC_ADDR", ":9085")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("failed to listen on grpc addr", "addr", grpcAddr, "error", err)
		panic(err)
	}
	grpcServer := grpc.NewServer()
	ratingv1.RegisterRatingServiceServer(grpcServer, grpcH)
	reflection.Register(grpcServer)

	go func() {
		slog.Info("gRPC listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("grpc serve error", "error", err)
			panic(err)
		}
	}()

	// ── HTTP server ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
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

	authMW := sharedmw.Require([]byte(config.MustEnv("JWT_SECRET")))
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		httphandler.New(st, slog.Default()).RegisterRoutes(r)
	})

	addr := config.Env("HTTP_ADDR", ":8085")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		slog.Info("rating-service listening", "addr", addr)
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

	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown error", "error", err)
	}
}
