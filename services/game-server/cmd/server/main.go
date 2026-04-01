// @title           Recess API
// @version         1.0
// @host            localhost
// @BasePath        /api/v1
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/recess/game-server/games"
	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/api"
	"github.com/recess/game-server/internal/platform/events"
	grpchandler "github.com/recess/game-server/internal/platform/grpc"
	"github.com/recess/game-server/internal/platform/ratelimit"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/userclient"
	"github.com/recess/game-server/internal/platform/ws"
	"github.com/recess/shared/config"
	gamev1 "github.com/recess/shared/proto/game/v1"
	lobbyv1 "github.com/recess/shared/proto/lobby/v1"
	sharedredis "github.com/recess/shared/redis"
	"github.com/recess/shared/telemetry"

	_ "github.com/recess/game-server/games/loveletter"
	_ "github.com/recess/game-server/games/tictactoe"
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

	st, err := store.New(ctx, config.Env("DATABASE_URL", "postgres://recess:recess@localhost:5432/recess?sslmode=disable"))
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

	redisURL := config.Env("REDIS_URL", "redis://localhost:6379")
	redisAddr, err := runtime.ParseRedisAddr(redisURL)
	if err != nil {
		slog.Error("failed to parse redis url for asynq", "error", err)
		os.Exit(1)
	}
	asynqRedis := asynq.RedisClientOpt{Addr: redisAddr}

	asynqClient := asynq.NewClient(asynqRedis)
	defer asynqClient.Close()
	asynqInspector := asynq.NewInspector(asynqRedis)
	defer asynqInspector.Close()

	timer := runtime.NewAsynqTimer(asynqClient, asynqInspector)
	runtimeService.SetTimer(timer)

	timerHandlers := runtime.NewTimerHandlers(runtimeService, hub, st, eventStore, timer, reg)

	// --- gRPC server (lobby.v1 + game.v1 for match-service) ------------------
	grpcAddr := config.Env("GRPC_ADDR", ":9080")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("failed to listen on grpc addr", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	lobbyv1.RegisterLobbyServiceServer(grpcServer, grpchandler.NewLobbyHandler(lobbyService, st))
	gamev1.RegisterGameServiceServer(grpcServer, grpchandler.NewGameHandler(lobbyService, runtimeService, st, hub))
	reflection.Register(grpcServer)

	go func() {
		slog.Info("gRPC listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("grpc serve error", "error", err)
			os.Exit(1)
		}
	}()

	timerHandlers.ReschedulePending(ctx)

	asynqSrv := asynq.NewServer(asynqRedis, asynq.Config{
		Concurrency: 10,
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(runtime.TypeTurnTimeout, timerHandlers.HandleTurnTimeout)
	mux.HandleFunc(runtime.TypeReadyTimeout, timerHandlers.HandleReadyTimeout)
	go func() {
		if err := asynqSrv.Run(mux); err != nil {
			slog.Error("asynq server error", "error", err)
		}
	}()

	testMode := config.Env("TEST_MODE", "false") == "true"

	var limiter *ratelimit.Limiter
	if !testMode {
		limiter = ratelimit.New(rdb, 100, time.Minute)
	}

	var jwtSecret []byte
	if !testMode {
		jwtSecret = []byte(config.MustEnv("JWT_SECRET"))
	}

	router := api.NewRouter(
		lobbyService,
		runtimeService,
		st,
		hub,
		jwtSecret,
		limiter,
		eventStore,
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

	asynqSrv.Shutdown()
	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("http shutdown error", "error", err)
	}
}
