package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/domain/notification"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/auth"
	"github.com/tableforge/server/internal/platform/events"
	"github.com/tableforge/server/internal/platform/presence"
	"github.com/tableforge/server/internal/platform/queue"
	"github.com/tableforge/server/internal/platform/ratelimit"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

// NewRouter builds and returns the main HTTP router.
// authHandler may be nil — auth middleware is skipped (tests only).
// limiter may be nil — rate limiting is skipped (tests only).
func NewRouter(
	lobbyService *lobby.Service,
	rt *runtime.Service,
	st store.Store,
	hub *ws.Hub,
	authHandler *auth.Handler,
	limiter *ratelimit.Limiter,
	eventStore *events.Store,
	presenceStore *presence.Store,
	queueSvc *queue.Service,
	notificationSvc *notification.Service,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(metricsMiddleware)

	r.Get("/health", handleHealth)
	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	// Per-route limiters derived from the global limiter's Redis connection.
	// nil limiter means tests — all rate limiting is skipped.
	var (
		authLimiter  func(http.Handler) http.Handler
		moveLimiter  func(http.Handler) http.Handler
		adminLimiter func(http.Handler) http.Handler
	)
	if limiter != nil {
		// 10 req/min on OAuth initiation — prevents GitHub OAuth abuse.
		authLimiter = limiter.WithKeyspace("rl:auth", 10, time.Minute).Middleware
		// 60 req/min on moves — prevents move spam while allowing fast play.
		moveLimiter = limiter.WithKeyspace("rl:move", 60, time.Minute).Middleware
		// 30 req/min on admin — tighter than general API.
		adminLimiter = limiter.WithKeyspace("rl:admin", 30, time.Minute).Middleware
	} else {
		noop := func(next http.Handler) http.Handler { return next }
		authLimiter = noop
		moveLimiter = noop
		adminLimiter = noop
	}

	requireAuth := func(r chi.Router) {
		if authHandler != nil {
			r.Use(authHandler.Middleware)
		}
	}

	// Auth routes — public
	if authHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.With(authLimiter).Get("/github", authHandler.HandleGitHubLogin)
			r.Get("/github/callback", authHandler.HandleGitHubCallback)
			r.Post("/logout", authHandler.HandleLogout)
			r.With(authHandler.Middleware).Get("/me", authHandler.HandleMe)

			// Test-only: bypasses OAuth for Playwright tests.
			// Returns 404 unless TEST_MODE=true.
			r.Get("/test-login", authHandler.HandleTestLogin)
		})
	}

	// API routes — protected in production, open when authHandler is nil (tests).
	r.Route("/api/v1", func(r chi.Router) {
		requireAuth(r)

		r.Get("/games", handleListGames)
		r.Get("/leaderboard", handleGetLeaderboard(st))

		r.Post("/players", handleCreatePlayer(st))
		r.Get("/players/{playerID}/sessions", handleListPlayerSessions(st))
		r.Get("/players/{playerID}/stats", handleGetPlayerStats(st))
		r.Get("/players/{playerID}/notifications", handleListNotifications(notificationSvc))

		r.Post("/rooms", handleCreateRoom(lobbyService))
		r.Get("/rooms", handleListRooms(lobbyService))
		r.Get("/rooms/{roomID}", handleGetRoom(lobbyService))
		r.Post("/rooms/join", handleJoinRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/leave", handleLeaveRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/start", handleStartGame(lobbyService, rt, hub))
		r.Put("/rooms/{roomID}/settings/{key}", handleUpdateRoomSetting(lobbyService, hub))

		r.Get("/sessions/{sessionID}", handleGetSession(rt))
		r.Get("/sessions/{sessionID}/events", handleGetSessionEvents(eventStore))
		r.Post("/sessions/{sessionID}/surrender", handleSurrender(rt, hub))
		r.Post("/sessions/{sessionID}/rematch", handleRematch(rt, hub))
		r.Post("/sessions/{sessionID}/pause", handleVotePause(rt, hub))
		r.Post("/sessions/{sessionID}/resume", handleVoteResume(rt, hub))
		r.With(requireRole(store.RoleManager)).Delete("/sessions/{sessionID}", handleForceCloseSession(st, hub))
		r.Get("/sessions/{sessionID}/history", handleGetSessionHistory(st))
		r.With(moveLimiter).Post("/sessions/{sessionID}/move", handleMove(rt, hub))

		// Direct messages
		r.Post("/players/{playerID}/dm", handleSendDM(st, hub))
		r.Get("/players/{playerID}/dm/unread", handleGetUnreadDMCount(st))
		r.Get("/players/{playerID}/dm/{otherPlayerID}", handleGetDMHistory(st))
		r.Post("/dm/{messageID}/read", handleMarkDMRead(st, hub))
		r.Post("/players/{playerID}/dm/{otherPlayerID}/report", handleReportDM(st))

		// Room chat
		r.Post("/rooms/{roomID}/messages", handleSendRoomMessage(st, hub))
		r.Get("/rooms/{roomID}/messages", handleGetRoomMessages(st))
		r.Post("/rooms/{roomID}/messages/{messageID}/report", handleReportRoomMessage(st))
		r.With(requireRole(store.RoleManager)).Delete("/rooms/{roomID}/messages/{messageID}", handleHideRoomMessage(st, hub))

		// Mutes
		r.Post("/players/{playerID}/mute", handleMutePlayer(st))
		r.Delete("/players/{playerID}/mute/{mutedID}", handleUnmutePlayer(st))
		r.Get("/players/{playerID}/mutes", handleGetMutedPlayers(st))

		// Ranked matchmaking queue
		r.Post("/queue", handleJoinQueue(queueSvc))
		r.Delete("/queue", handleLeaveQueue(queueSvc))
		r.Post("/queue/accept", handleAcceptMatch(queueSvc))
		r.Post("/queue/decline", handleDeclineMatch(queueSvc))

		// Notification actions
		r.Post("/notifications/{notificationID}/read", handleMarkNotificationRead(notificationSvc))
		r.Post("/notifications/{notificationID}/accept", handleAcceptNotification(notificationSvc))
		r.Post("/notifications/{notificationID}/decline", handleDeclineNotification(notificationSvc))

		// Admin routes — manager+ only, stricter rate limit.
		r.Route("/admin", func(r chi.Router) {
			r.Use(adminLimiter)
			r.Use(requireRole(store.RoleManager))

			r.Get("/allowed-emails", handleListAllowedEmails(st))
			r.Post("/allowed-emails", handleAddAllowedEmail(st))
			r.Delete("/allowed-emails/{email}", handleRemoveAllowedEmail(st))

			r.Get("/players", handleListPlayers(st))
			r.With(requireRole(store.RoleOwner)).Put("/players/{playerID}/role", handleSetPlayerRole(st))
		})
	})

	// WebSocket — protected in production, open when authHandler is nil (tests).
	wsHandler := http.HandlerFunc(ws.Handler(hub, st, eventStore, presenceStore))
	if authHandler != nil {
		wsHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.Middleware(ws.Handler(hub, st, eventStore, presenceStore)).ServeHTTP(w, r)
		})
	}
	r.Get("/ws/rooms/{roomID}", wsHandler)

	return r
}

// --- Health ------------------------------------------------------------------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// requireRole returns a middleware that allows access only to players whose
// role is >= the required role in the hierarchy: player < manager < owner.
func requireRole(minimum store.PlayerRole) func(http.Handler) http.Handler {
	hierarchy := map[store.PlayerRole]int{
		store.RolePlayer:  0,
		store.RoleManager: 1,
		store.RoleOwner:   2,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			playerID, ok := auth.PlayerIDFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			role, ok := auth.RoleFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			_ = playerID
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
