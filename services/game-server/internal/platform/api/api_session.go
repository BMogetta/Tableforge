package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/events"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	error_message "github.com/recess/shared/errors"
	sharedmw "github.com/recess/shared/middleware"
)

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

// @Summary  Apply a move
// @Tags     sessions
// @Accept   json
// @Produce  json
// @Param    sessionID path     string      true "Session UUID"
// @Param    body      body     moveRequest true "Move payload"
// @Success  200       {object} MoveResponse
// @Failure  400       {object} map[string]string
// @Failure  404       {object} map[string]string
// @Failure  409       {object} map[string]string
// @Router   /sessions/{sessionID}/move [post]
func handleMove(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
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

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
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
		players, _ := st.ListRoomPlayers(r.Context(), result.Session.RoomID)
		rt.BroadcastMove(r.Context(), hub, result, eventType, players)
		movesTotal.WithLabelValues(result.Session.GameID).Inc()
		if result.IsOver {
			activeSessions.Dec()
		} else {
			// If the next player is a registered bot, fire its move
			// asynchronously. This is a no-op if no bot is registered
			// for the next player.
			nextPlayerUUID, parseErr := uuid.Parse(string(result.State.CurrentPlayerID))
			if parseErr == nil {
				rt.MaybeFireBot(r.Context(), hub, sessionID, nextPlayerUUID)
			}
		}
		writeJSON(w, http.StatusOK, MoveResponse{
			Session: sessionToDTO(result.Session),
			State:   result.State,
			IsOver:  result.IsOver,
			Result:  result.Result,
		})
	}
}

// @Summary  Get game session
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {object} SessionResponse
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID} [get]
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
		writeJSON(w, http.StatusOK, SessionResponse{
			Session: sessionToDTO(session),
			State:   state,
			Result:  result,
		})
	}
}

// @Summary  Get session events
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {array}  SessionEventDTO
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID}/events [get]
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
		dtos := make([]SessionEventDTO, len(evts))
		for i, e := range evts {
			dtos[i] = sessionEventToDTO(e)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

// @Summary  Vote ready
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {object} runtime.ReadyVoteResult
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID}/ready [post]
func handlePlayerReady(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := mustParseUUID(w, r, "sessionID")
		if sessionID == uuid.Nil {
			return
		}
		playerID, _ := sharedmw.PlayerIDFromContext(r.Context())

		result, err := rt.VoteReady(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		if result.AllReady {
			session, err := st.GetGameSession(r.Context(), sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			// Cancel ready timer — all players confirmed in time.
			// Start turn timer now that the game is truly underway.
			rt.OnAllReady(r.Context(), session, hub)
		} else {
			// Broadcast partial progress so clients can show "waiting" UI.
			session, _ := st.GetGameSession(r.Context(), sessionID)
			hub.Broadcast(session.RoomID, ws.Event{
				Type:    ws.EventPlayerReady,
				Payload: result,
			})
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type surrenderRequest struct {
	PlayerID string `json:"player_id"`
}

// @Summary  Surrender a game
// @Tags     sessions
// @Accept   json
// @Produce  json
// @Param    sessionID path     string          true "Session UUID"
// @Param    body      body     surrenderRequest true "Player ID"
// @Success  200       {object} MoveResponse
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID}/surrender [post]
func handleSurrender(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
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

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		result, err := rt.Surrender(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}
		players, _ := st.ListRoomPlayers(r.Context(), result.Session.RoomID)
		rt.BroadcastMove(r.Context(), hub, result, ws.EventGameOver, players)
		activeSessions.Dec()
		writeJSON(w, http.StatusOK, MoveResponse{
			Session: sessionToDTO(result.Session),
			State:   result.State,
			IsOver:  result.IsOver,
			Result:  result.Result,
		})
	}
}

type rematchRequest struct {
	PlayerID string `json:"player_id"`
}

// @Summary  Vote for rematch
// @Tags     sessions
// @Accept   json
// @Produce  json
// @Param    sessionID path     string         true "Session UUID"
// @Param    body      body     rematchRequest true "Player ID"
// @Success  200       {object} RematchResponse
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID}/rematch [post]
func handleRematch(lobbySvc *lobby.Service, rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
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

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		votes, totalPlayers, roomID, rematchReady, err := rt.VoteRematch(r.Context(), sessionID, playerID)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}
		if rematchReady {
			// Auto-start a new session with the same settings.
			newSession, err := lobbySvc.StartGame(r.Context(), roomID, playerID, store.SessionModeCasual)
			if err != nil {
				slog.Error("rematch: auto-start failed", "room_id", roomID, "error", err)
				// Fall back to sending players to the room.
				hub.Broadcast(roomID, ws.Event{
					Type:    ws.EventRematchReady,
					Payload: map[string]any{"room_id": roomID},
				})
			} else {
				rt.StartSession(r.Context(), newSession, hub, runtime.DefaultReadyTimeout)
				hub.Broadcast(roomID, ws.Event{
					Type:    ws.EventGameStarted,
					Payload: map[string]any{"session": newSession},
				})

				// If the first player to move is a bot, fire immediately.
				var initialState struct {
					CurrentPlayerID string `json:"current_player_id"`
				}
				if err := json.Unmarshal(newSession.State, &initialState); err == nil {
					if firstPlayer, err := uuid.Parse(initialState.CurrentPlayerID); err == nil {
						rt.MaybeFireBot(r.Context(), hub, newSession.ID, firstPlayer)
					}
				}

				slog.Info("rematch: new session started", "room_id", roomID, "session_id", newSession.ID)
			}
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
		writeJSON(w, http.StatusOK, RematchResponse{
			Votes:        len(votes),
			TotalPlayers: totalPlayers,
		})
	}
}

// @Summary  Get session history
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {array}  MoveDTO
// @Failure  404       {object} map[string]string
// @Router   /sessions/{sessionID}/history [get]
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
		dtos := make([]MoveDTO, len(moves))
		for i, m := range moves {
			dtos[i] = moveToDTO(m)
		}
		writeJSON(w, http.StatusOK, dtos)
	}
}

// --- Pause / Resume ----------------------------------------------------------

// @Summary  Vote to pause
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {object} runtime.PauseVoteResult
// @Failure  404       {object} map[string]string
// @Failure  409       {object} map[string]string
// @Router   /sessions/{sessionID}/pause [post]
func handleVotePause(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		result, err := rt.VotePause(r.Context(), sessionID, playerID)
		if err != nil {
			switch {
			case errors.Is(err, runtime.ErrSessionNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, runtime.ErrGameOver):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, runtime.ErrAlreadyPaused):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, runtime.ErrNotParticipant):
				writeError(w, http.StatusForbidden, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		session, _, _, err := rt.GetSessionAndState(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if result.AllVoted {
			hub.Broadcast(session.RoomID, ws.Event{
				Type:    ws.EventSessionSuspended,
				Payload: map[string]any{"suspended_at": time.Now()},
			})
		} else {
			hub.Broadcast(session.RoomID, ws.Event{
				Type: ws.EventPauseVoteUpdate,
				Payload: map[string]any{
					"votes":    result.Votes,
					"required": result.Required,
				},
			})
		}

		writeJSON(w, http.StatusOK, result)
	}
}

// @Summary  Vote to resume
// @Tags     sessions
// @Produce  json
// @Param    sessionID path     string true "Session UUID"
// @Success  200       {object} runtime.PauseVoteResult
// @Failure  404       {object} map[string]string
// @Failure  409       {object} map[string]string
// @Router   /sessions/{sessionID}/resume [post]
func handleVoteResume(rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		result, err := rt.VoteResume(r.Context(), sessionID, playerID)
		if err != nil {
			switch {
			case errors.Is(err, runtime.ErrSessionNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, runtime.ErrGameOver):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, runtime.ErrNotSuspended):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, runtime.ErrNotParticipant):
				writeError(w, http.StatusForbidden, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		session, _, _, err := rt.GetSessionAndState(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if result.AllVoted {
			hub.Broadcast(session.RoomID, ws.Event{
				Type:    ws.EventSessionResumed,
				Payload: map[string]any{"resumed_at": time.Now()},
			})
		} else {
			hub.Broadcast(session.RoomID, ws.Event{
				Type: ws.EventResumeVoteUpdate,
				Payload: map[string]any{
					"votes":    result.Votes,
					"required": result.Required,
				},
			})
		}

		writeJSON(w, http.StatusOK, result)
	}
}

// DELETE /api/v1/sessions/{sessionID}
// Force-closes a suspended session. Manager-only.
// Broadcasts room_closed so all clients can navigate away.
func handleForceCloseSession(st store.Store, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		session, err := st.GetGameSession(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}

		if err := st.ForceCloseSession(r.Context(), sessionID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to close session")
			return
		}

		hub.Broadcast(session.RoomID, ws.Event{
			Type:    ws.EventRoomClosed,
			Payload: nil,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
