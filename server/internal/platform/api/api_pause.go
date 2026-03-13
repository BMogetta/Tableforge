package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

// POST /api/v1/sessions/{sessionID}/pause
// Registers a pause vote for the calling player.
// When all participants have voted the session is suspended and
// a session_suspended event is broadcast; otherwise pause_vote_update is sent.
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

		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
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

// POST /api/v1/sessions/{sessionID}/resume
// Registers a resume vote for the calling player.
// When all participants have voted the session is resumed and
// a session_resumed event is broadcast; otherwise resume_vote_update is sent.
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

		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
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
