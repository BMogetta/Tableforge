package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/queue"
	error_message "github.com/tableforge/shared/errors"
	sharedmw "github.com/tableforge/shared/middleware"
)

// POST /api/v1/queue
// Body: { "player_id": "uuid" }
// Adds the player to the ranked matchmaking queue.
// 429 if the player is currently banned from queueing.
// 409 if the player is already in the queue.
func handleJoinQueue(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		pos, err := svc.Enqueue(r.Context(), playerID)
		if err != nil {
			var banned *queue.ErrBanned
			if errors.As(err, &banned) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", banned.RetryAfterSecs))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]any{
					"error":            "queue ban active",
					"retry_after_secs": banned.RetryAfterSecs,
				})
				return
			}
			if errors.Is(err, queue.ErrAlreadyQueued) {
				writeError(w, http.StatusConflict, "already in queue")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to join queue")
			return
		}

		writeJSON(w, http.StatusOK, pos)
	}
}

// DELETE /api/v1/queue
// Body: { "player_id": "uuid" }
// Removes the player from the ranked matchmaking queue.
// 204 whether or not the player was actually queued (idempotent).
func handleLeaveQueue(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		if _, err := svc.Dequeue(r.Context(), playerID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to leave queue")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /api/v1/queue/accept
// Body: { "player_id": "uuid", "match_id": "uuid" }
// Records that the player accepts the proposed match.
// If both players have accepted the match is started and match_ready is
// broadcast to both via their player channels.
// 404 if the match has expired or already been resolved.
func handleAcceptMatch(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PlayerID string `json:"player_id"`
			MatchID  string `json:"match_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		matchID, err := uuid.Parse(req.MatchID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid match_id")
			return
		}

		if err := svc.Accept(r.Context(), playerID, matchID); err != nil {
			if errors.Is(err, queue.ErrMatchNotFound) {
				writeError(w, http.StatusNotFound, "match not found or already expired")
				return
			}
			if errors.Is(err, queue.ErrNotMatchParticipant) {
				writeError(w, http.StatusForbidden, "not a participant in this match")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to accept match")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /api/v1/queue/decline
// Body: { "player_id": "uuid", "match_id": "uuid" }
// Records that the player declines the proposed match.
// The declining player is dropped from the queue and receives a penalty.
// The other player (if they had already accepted) is automatically re-queued.
// 404 if the match has expired or already been resolved.
func handleDeclineMatch(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PlayerID string `json:"player_id"`
			MatchID  string `json:"match_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		matchID, err := uuid.Parse(req.MatchID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid match_id")
			return
		}

		if err := svc.Decline(r.Context(), playerID, matchID); err != nil {
			if errors.Is(err, queue.ErrMatchNotFound) {
				writeError(w, http.StatusNotFound, "match not found or already expired")
				return
			}
			if errors.Is(err, queue.ErrNotMatchParticipant) {
				writeError(w, http.StatusForbidden, "not a participant in this match")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to decline match")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
