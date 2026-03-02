// Package api wires up the HTTP routes and handlers for Tableforge.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tableforge/server/games"
	"github.com/tableforge/server/internal/auth"
	"github.com/tableforge/server/internal/engine"
	"github.com/tableforge/server/internal/lobby"
	"github.com/tableforge/server/internal/ratelimit"
	"github.com/tableforge/server/internal/runtime"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/ws"
)

// --- Router ------------------------------------------------------------------

// NewRouter builds and returns the main HTTP router.
// authHandler may be nil — auth middleware is skipped (tests only).
// limiter may be nil — rate limiting is skipped (tests only).
func NewRouter(lobbyService *lobby.Service, rt *runtime.Service, st store.Store, hub *ws.Hub, authHandler *auth.Handler, limiter *ratelimit.Limiter) http.Handler {
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

	// API routes — protected in production, open when authHandler is nil (tests)
	r.Route("/api/v1", func(r chi.Router) {
		requireAuth(r)

		r.Get("/games", handleListGames(games.All))
		r.Get("/leaderboard", handleGetLeaderboard(st))

		r.Post("/players", handleCreatePlayer(st))
		r.Get("/players/{playerID}/sessions", handleListPlayerSessions(st))
		r.Get("/players/{playerID}/stats", handleGetPlayerStats(st))

		r.Post("/rooms", handleCreateRoom(lobbyService))
		r.Get("/rooms", handleListRooms(lobbyService))
		r.Get("/rooms/{roomID}", handleGetRoom(lobbyService))
		r.Post("/rooms/join", handleJoinRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/leave", handleLeaveRoom(lobbyService, hub))
		r.Post("/rooms/{roomID}/start", handleStartGame(lobbyService, rt, hub))

		r.Get("/sessions/{sessionID}", handleGetSession(rt))
		r.Post("/sessions/{sessionID}/surrender", handleSurrender(rt, hub))
		r.Post("/sessions/{sessionID}/rematch", handleRematch(rt, hub))
		r.Get("/sessions/{sessionID}/history", handleGetSessionHistory(st))
		r.With(moveLimiter).Post("/sessions/{sessionID}/move", handleMove(rt, hub))

		// Admin routes — manager+ only, stricter rate limit
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

	// WebSocket — protected in production, open when authHandler is nil (tests)
	wsHandler := http.HandlerFunc(ws.Handler(hub))
	if authHandler != nil {
		wsHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.Middleware(ws.Handler(hub)).ServeHTTP(w, r)
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

// --- Allowed emails ----------------------------------------------------------

// GET /api/v1/admin/allowed-emails
func handleListAllowedEmails(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := st.ListAllowedEmails(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list allowed emails")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

type addAllowedEmailRequest struct {
	Email string           `json:"email"`
	Role  store.PlayerRole `json:"role"`
}

// POST /api/v1/admin/allowed-emails
func handleAddAllowedEmail(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addAllowedEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if req.Role == "" {
			req.Role = store.RolePlayer
		}

		// Managers can only invite players, not other managers or owners.
		callerRole, _ := auth.RoleFromContext(r.Context())
		if callerRole == store.RoleManager && req.Role != store.RolePlayer {
			writeError(w, http.StatusForbidden, "managers can only invite players")
			return
		}

		callerID, _ := auth.PlayerIDFromContext(r.Context())
		entry, err := st.AddAllowedEmail(r.Context(), store.AddAllowedEmailParams{
			Email:     req.Email,
			Role:      req.Role,
			InvitedBy: &callerID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add email")
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	}
}

// DELETE /api/v1/admin/allowed-emails/{email}
func handleRemoveAllowedEmail(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := chi.URLParam(r, "email")
		if email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if err := st.RemoveAllowedEmail(r.Context(), email); err != nil {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Players -----------------------------------------------------------------

type createPlayerRequest struct {
	Username string `json:"username"`
}

func handleCreatePlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPlayerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Username == "" {
			writeError(w, http.StatusBadRequest, "username is required")
			return
		}
		player, err := st.CreatePlayer(r.Context(), req.Username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create player")
			return
		}
		writeJSON(w, http.StatusCreated, player)
	}
}

// GET /api/v1/admin/players
func handleListPlayers(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := st.ListPlayers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list players")
			return
		}
		writeJSON(w, http.StatusOK, players)
	}
}

type setRoleRequest struct {
	Role store.PlayerRole `json:"role"`
}

// PUT /api/v1/admin/players/{playerID}/role
func handleSetPlayerRole(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		var req setRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Role == "" {
			writeError(w, http.StatusBadRequest, "role is required")
			return
		}

		// Prevent self-demotion.
		callerID, _ := auth.PlayerIDFromContext(r.Context())
		if callerID == playerID {
			callerRole, _ := auth.RoleFromContext(r.Context())
			if callerRole == store.RoleOwner && req.Role != store.RoleOwner {
				writeError(w, http.StatusForbidden, "cannot demote yourself")
				return
			}
		}

		if err := st.SetPlayerRole(r.Context(), playerID, req.Role); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Games -------------------------------------------------------------------

// gameInfo is the public representation of a registered game.
type gameInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MinPlayers int    `json:"min_players"`
	MaxPlayers int    `json:"max_players"`
}

// handleListGames returns all games available in the engine registry.
func handleListGames(all func() []engine.Game) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := all()
		result := make([]gameInfo, 0, len(list))
		for _, g := range list {
			result = append(result, gameInfo{
				ID:         g.ID(),
				Name:       g.Name(),
				MinPlayers: g.MinPlayers(),
				MaxPlayers: g.MaxPlayers(),
			})
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// GET /api/v1/players/{playerID}/sessions
func handleListPlayerSessions(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		sessions, err := st.ListActiveSessions(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list sessions")
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	}
}

// GET /api/v1/players/{playerID}/stats
func handleGetPlayerStats(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		stats, err := st.GetPlayerStats(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get stats")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

// --- Rooms -------------------------------------------------------------------

type createRoomRequest struct {
	GameID          string `json:"game_id"`
	PlayerID        string `json:"player_id"`
	TurnTimeoutSecs *int   `json:"turn_timeout_secs,omitempty"`
}

func handleCreateRoom(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		ownerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		view, err := svc.CreateRoom(r.Context(), req.GameID, ownerID, req.TurnTimeoutSecs)
		if err != nil {
			writeLobbyError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, view)
	}
}

func handleListRooms(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rooms, err := svc.ListWaitingRooms(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list rooms")
			return
		}
		writeJSON(w, http.StatusOK, rooms)
	}
}

func handleGetRoom(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		view, err := svc.GetRoom(r.Context(), roomID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}

type joinRoomRequest struct {
	Code     string `json:"code"`
	PlayerID string `json:"player_id"`
}

func handleJoinRoom(svc *lobby.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req joinRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		view, err := svc.JoinRoom(r.Context(), req.Code, playerID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}

		hub.Broadcast(view.Room.ID, ws.Event{
			Type:    ws.EventPlayerJoined,
			Payload: view,
		})

		writeJSON(w, http.StatusOK, view)
	}
}

type leaveRoomRequest struct {
	PlayerID string `json:"player_id"`
}

func handleLeaveRoom(svc *lobby.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		var req leaveRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		if err := svc.LeaveRoom(r.Context(), roomID, playerID); err != nil {
			writeLobbyError(w, err)
			return
		}

		hub.Broadcast(roomID, ws.Event{
			Type:    ws.EventPlayerLeft,
			Payload: map[string]string{"player_id": playerID.String()},
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

type startGameRequest struct {
	PlayerID string `json:"player_id"`
}

func handleStartGame(svc *lobby.Service, rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		var req startGameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		session, err := svc.StartGame(r.Context(), roomID, playerID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}

		// Schedule the initial turn timer for the new session.
		rt.StartSession(session)

		hub.Broadcast(roomID, ws.Event{
			Type:    ws.EventGameStarted,
			Payload: map[string]any{"session": session},
		})

		writeJSON(w, http.StatusOK, session)
		activeSessions.Inc()
	}
}

// --- Sessions ----------------------------------------------------------------

type moveRequest struct {
	PlayerID string         `json:"player_id"`
	Payload  map[string]any `json:"payload"`
}

func handleMove(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		result, err := rt.ApplyMove(r.Context(), sessionID, playerID, req.Payload)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		eventType := ws.EventMoveApplied
		if result.IsOver {
			eventType = ws.EventGameOver
		}
		hub.Broadcast(result.Session.RoomID, ws.Event{
			Type:    eventType,
			Payload: result,
		})

		movesTotal.WithLabelValues(result.Session.GameID).Inc()
		if result.IsOver {
			activeSessions.Dec()
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleGetSession(rt *runtime.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		session, state, result, err := rt.GetSessionAndState(r.Context(), sessionID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"session": session,
			"state":   state,
			"result":  result,
		})
	}
}

type surrenderRequest struct {
	PlayerID string `json:"player_id"`
}

func handleSurrender(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req surrenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		result, err := rt.Surrender(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		hub.Broadcast(result.Session.RoomID, ws.Event{
			Type:    ws.EventGameOver,
			Payload: result,
		})
		activeSessions.Dec()

		writeJSON(w, http.StatusOK, result)
	}
}

type rematchRequest struct {
	PlayerID string `json:"player_id"`
}

func handleRematch(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req rematchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		votes, totalPlayers, roomID, newSession, err := rt.VoteRematch(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		if newSession != nil {
			log.Printf("broadcasting rematch_started to room %s, new session %s", roomID, newSession.ID)
			hub.Broadcast(roomID, ws.Event{
				Type:    ws.EventRematchStarted,
				Payload: map[string]any{"session": newSession},
			})
			activeSessions.Inc()
		} else {
			hub.Broadcast(roomID, ws.Event{
				Type: ws.EventRematchVote,
				Payload: map[string]any{
					"player_id":     playerID.String(),
					"votes":         len(votes),
					"total_players": totalPlayers,
				},
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"votes":         len(votes),
			"total_players": totalPlayers,
			"session":       newSession,
		})
	}
}

// GET /api/v1/sessions/{sessionID}/history
func handleGetSessionHistory(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		moves, err := st.ListSessionMoves(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get history")
			return
		}
		writeJSON(w, http.StatusOK, moves)
	}
}

// --- Leaderboard -------------------------------------------------------------

// GET /api/v1/leaderboard?game_id=chess&limit=20
func handleGetLeaderboard(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID := r.URL.Query().Get("game_id")
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		entries, err := st.GetLeaderboard(r.Context(), gameID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get leaderboard")
			return
		}
		writeJSON(w, http.StatusOK, entries)
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
