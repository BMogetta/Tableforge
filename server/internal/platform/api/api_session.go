package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/events"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

// --- Players -----------------------------------------------------------------

// POST /api/v1/players
func handleCreatePlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
		}
		if err := decodeJSON(r, &req); err != nil {
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

// --- Sessions ----------------------------------------------------------------

type moveRequest struct {
	PlayerID string         `json:"player_id"`
	Payload  map[string]any `json:"payload"`
}

// POST /api/v1/sessions/{sessionID}/move
func handleMove(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req moveRequest
		if err := decodeJSON(r, &req); err != nil {
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

// GET /api/v1/sessions/{sessionID}
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

// GET /api/v1/sessions/{sessionID}/events
// If the session is still active, reads from the Redis Stream.
// If the session is finished, reads from Postgres.
func handleGetSessionEvents(ev *events.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		evts, err := ev.Read(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get events")
			return
		}
		writeJSON(w, http.StatusOK, evts)
	}
}

type surrenderRequest struct {
	PlayerID string `json:"player_id"`
}

// POST /api/v1/sessions/{sessionID}/surrender
func handleSurrender(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req surrenderRequest
		if err := decodeJSON(r, &req); err != nil {
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

// POST /api/v1/sessions/{sessionID}/rematch
func handleRematch(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}
		var req rematchRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		votes, totalPlayers, roomID, rematchReady, err := rt.VoteRematch(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}
		if rematchReady {
			log.Printf("rematch ready: room %s returning to lobby", roomID)
			hub.Broadcast(roomID, ws.Event{
				Type:    ws.EventRematchReady,
				Payload: map[string]any{"room_id": roomID},
			})
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

// GET /api/v1/leaderboard
// Returns the top players ordered by MMR descending.
// Optional query param: limit (1-100, default 20).
func handleGetLeaderboard(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		entries, err := st.GetRatingLeaderboard(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get leaderboard")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}
