package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/recess/services/user-service/internal/api"
	"github.com/recess/services/user-service/internal/consumer"
	usgrpc "github.com/recess/services/user-service/internal/grpc"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/config"
	"github.com/recess/shared/featureflags"
	sharedmw "github.com/recess/shared/middleware"
	sharedredis "github.com/recess/shared/redis"
	userv1 "github.com/recess/shared/proto/user/v1"
	"github.com/recess/shared/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	// Blank imports wire the achievement registry via init(). Adding a new
	// game with achievements means adding one more line here.
	_ "github.com/recess/shared/achievements/games/tictactoe"
	_ "github.com/recess/shared/achievements/global"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serviceName := config.Env("OTEL_SERVICE_NAME", "user-service")
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

	st, err := store.New(ctx, config.MustEnv("DATABASE_URL"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		panic(err)
	}
	defer st.Close()

	// --- Redis ---------------------------------------------------------------
	rdb := sharedredis.MustConnect(ctx, config.MustEnv("REDIS_URL"))
	defer func() { _ = rdb.Close() }()

	pub := api.NewPublisher(rdb)

	// --- JSON Schema validation ----------------------------------------------
	schemaReg, err := sharedmw.NewSchemaRegistry()
	if err != nil {
		slog.Error("failed to compile JSON schemas", "error", err)
		panic(err)
	}

	// --- gRPC server ---------------------------------------------------------
	grpcAddr := config.Env("GRPC_ADDR", ":9082")
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	userv1.RegisterUserServiceServer(grpcServer, usgrpc.NewServer(st))

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("failed to listen on gRPC addr", "addr", grpcAddr, "error", err)
		panic(err)
	}

	go func() {
		slog.Info("user-service gRPC listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server error", "error", err)
			panic(err)
		}
	}()

	// --- Feature flags -------------------------------------------------------
	flags, err := featureflags.Init(config.LoadUnleash(serviceName))
	if err != nil {
		slog.Warn("feature flags init failed, using defaults", "error", err)
	}
	defer func() { _ = flags.Close() }()

	// --- HTTP server ---------------------------------------------------------
	authMW := sharedmw.Require([]byte(jwtSecret))
	router := api.NewRouter(st, pub, authMW, schemaReg)
	handler := sharedmw.Maintenance(flags)(router)

	httpAddr := config.Env("HTTP_ADDR", ":8082")
	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("user-service HTTP listening", "addr", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			panic(err)
		}
	}()

	// --- Achievement consumer ------------------------------------------------
	achievementConsumer := consumer.New(rdb, st, pub, slog.Default(), flags)
	go func() {
		if err := achievementConsumer.Run(ctx); err != nil {
			slog.Error("achievement consumer error", "error", err)
		}
	}()

	// --- Graceful shutdown ---------------------------------------------------
	<-ctx.Done()
	slog.Info("shutting down user-service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
	grpcServer.GracefulStop()
}
