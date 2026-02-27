// Package api wires up the HTTP routes and handlers for Tableforge.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tableforge/server/internal/auth"
	"github.com/tableforge/server/internal/lobby"
	"github.com/tableforge/server/internal/runtime"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/ws"
)

// --- Router ------------------------------------------------------------------

// NewRouter builds and returns the main HTTP router.
// authHandler may be nil — auth middleware is skipped (tests only).
func NewRouter(lobbyService *lobby.Service, rt *runtime.Service, st store.Store, hub *ws.Hub, authHandler *auth.Handler) http.Handler {
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

	// Auth routes — public (skipped in tests)
	if authHandler != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/github", authHandler.HandleGitHubLogin)
			r.Get("/github/callback", authHandler.HandleGitHubCallback)
			r.Post("/logout", authHandler.HandleLogout)
			r.With(authHandler.Middleware).Get("/me", authHandler.HandleMe)
		})
	}

	requireAuth := func(r chi.Router) {
		if authHandler != nil {
			r.Use(authHandler.Middleware)
		}
	}

	// API routes — protected in production, open when authHandler is nil (tests)
	r.Route("/api/v1", func(r chi.Router) {
		requireAuth(r)

		r.Post("/players", handleCreatePlayer(st))

		r.Post("/rooms", handleCreateRoom(lobbyService))
		r.Get("/rooms", handleListRooms(lobbyService))
		r.Get("/rooms/{roomID}", handleGetRoom(lobbyService))
		r.Post("/rooms/join", handleJoinRoom(lobbyService))
		r.Post("/rooms/{roomID}/leave", handleLeaveRoom(lobbyService))
		r.Post("/rooms/{roomID}/start", handleStartGame(lobbyService))

		r.Post("/sessions/{sessionID}/move", handleMove(rt, hub))
		r.Get("/sessions/{sessionID}", handleGetSession(rt))
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

// --- Rooms -------------------------------------------------------------------

type createRoomRequest struct {
	GameID   string `json:"game_id"`
	PlayerID string `json:"player_id"`
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

		view, err := svc.CreateRoom(r.Context(), req.GameID, ownerID)
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

func handleJoinRoom(svc *lobby.Service) http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, view)
	}
}

type leaveRoomRequest struct {
	PlayerID string `json:"player_id"`
}

func handleLeaveRoom(svc *lobby.Service) http.HandlerFunc {
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

		w.WriteHeader(http.StatusNoContent)
	}
}

type startGameRequest struct {
	PlayerID string `json:"player_id"`
}

func handleStartGame(svc *lobby.Service) http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, session)
		activeSessions.Inc()
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
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
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

		state, err := rt.GetState(r.Context(), sessionID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, state)
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
