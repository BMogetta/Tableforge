package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/tableforge/game-server/internal/domain/lobby"
	"github.com/tableforge/game-server/internal/domain/runtime"
	"github.com/tableforge/game-server/internal/platform/events"
	"github.com/tableforge/game-server/internal/platform/ratelimit"
	"github.com/tableforge/game-server/internal/platform/store"
	"github.com/tableforge/game-server/internal/platform/userclient"
	"github.com/tableforge/game-server/internal/platform/ws"
	sharedmw "github.com/tableforge/shared/middleware"
)

// NewRouter builds and returns the main HTTP router.
// jwtSecret may be nil — auth middleware is skipped (tests only).
// limiter may be nil — rate limiting is skipped (tests only).
func NewRouter(
	lobbyService *lobby.Service,
	rt *runtime.Service,
	st store.Store,
	hub *ws.Hub,
	jwtSecret []byte,
	limiter *ratelimit.Limiter,
	eventStore *events.Store,
	userClient *userclient.Client,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(otelchi.Middleware("game-server",
		otelchi.WithChiRoutes(r),
		otelchi.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/metrics" && r.URL.Path != "/health"
		}),
	))
	r.Use(metricsMiddleware)

	r.Get("/health", handleHealth)
	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	var moveLimiter func(http.Handler) http.Handler
	if limiter != nil {
		moveLimiter = limiter.WithKeyspace("rl:move", 60, time.Minute).Middleware
	} else {
		moveLimiter = func(next http.Handler) http.Handler { return next }
	}

	// authMW is nil in test mode — all protected routes are open.
	var authMW func(http.Handler) http.Handler
	if jwtSecret != nil {
		authMW = sharedmw.Require(jwtSecret)
	} else {
		authMW = func(next http.Handler) http.Handler { return next }
	}

	// API routes — protected in production, open when jwtSecret is nil (tests).
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW)

		r.Get("/games", handleListGames)
		r.Get("/bots/profiles", handleListBotProfiles())

		r.Get("/players/{playerID}/sessions", handleListPlayerSessions(st))
		r.Get("/players/{playerID}/stats", handleGetPlayerStats(st))
		r.Post("/rooms", handleCreateRoom(lobbyService))
		r.Get("/rooms", handleListRooms(lobbyService))
		r.Get("/rooms/{roomID}", handleGetRoom(lobbyService))
		r.Post("/rooms/join", handleJoinRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/leave", handleLeaveRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/start", handleStartGame(lobbyService, rt, hub))
		r.Put("/rooms/{roomID}/settings/{key}", handleUpdateRoomSetting(lobbyService, hub))
		r.Post("/rooms/{roomID}/bots", handleAddBot(rt, hub, st))
		r.Delete("/rooms/{roomID}/bots/{botID}", handleRemoveBot(rt, hub, st))

		r.Get("/sessions/{sessionID}", handleGetSession(rt))
		r.Post("/sessions/{sessionID}/ready", handlePlayerReady(rt, hub, st))
		r.Get("/sessions/{sessionID}/events", handleGetSessionEvents(eventStore))
		r.Post("/sessions/{sessionID}/surrender", handleSurrender(rt, hub, st))
		r.Post("/sessions/{sessionID}/rematch", handleRematch(rt, hub))
		r.Post("/sessions/{sessionID}/pause", handleVotePause(rt, hub))
		r.Post("/sessions/{sessionID}/resume", handleVoteResume(rt, hub))
		r.With(requireRole(sharedmw.RoleManager)).Delete("/sessions/{sessionID}", handleForceCloseSession(st, hub))
		r.Get("/sessions/{sessionID}/history", handleGetSessionHistory(st))
		r.With(moveLimiter).Post("/sessions/{sessionID}/move", handleMove(rt, hub, st))

	})

	return r
}

// --- Health ------------------------------------------------------------------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// requireRole returns a middleware that allows access only to players whose
// role is >= the required role in the hierarchy: player < manager < owner.
func requireRole(minimum string) func(http.Handler) http.Handler {
	hierarchy := map[string]int{
		sharedmw.RolePlayer:  0,
		sharedmw.RoleManager: 1,
		sharedmw.RoleOwner:   2,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := sharedmw.PlayerIDFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			role, ok := sharedmw.RoleFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			if hierarchy[role] < hierarchy[minimum] {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeLobbyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, lobby.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, lobby.ErrGameNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, lobby.ErrRoomFull):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, lobby.ErrAlreadyInRoom):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, lobby.ErrRoomNotWaiting):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, lobby.ErrNotOwner):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, lobby.ErrNotEnoughPlayer):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, lobby.ErrUnknownSetting):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, lobby.ErrInvalidSettingVal):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, runtime.ErrNotParticipant):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, runtime.ErrNotFinished):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeRuntimeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, runtime.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, runtime.ErrGameNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, runtime.ErrGameOver):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, runtime.ErrInvalidMove):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func mustParseUUID(w http.ResponseWriter, r *http.Request, param string) uuid.UUID {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+param)
		return uuid.Nil
	}
	return id
}
