package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/match-service/internal/queue"
	apierrors "github.com/recess/shared/errors"
	sharedmw "github.com/recess/shared/middleware"
)

// NewRouter builds the match-service HTTP router.
func NewRouter(svc *queue.Service, authMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW)

		// Ranked matchmaking queue
		r.Post("/queue", handleJoinQueue(svc))
		r.Delete("/queue", handleLeaveQueue(svc))
		r.Post("/queue/accept", handleAcceptMatch(svc))
		r.Post("/queue/decline", handleDeclineMatch(svc))
	})

	return r
}

// POST /api/v1/queue
// Adds the authenticated player to the ranked matchmaking queue.
// 429 if the player is currently banned from queueing.
// 409 if the player is already in the queue.
func handleJoinQueue(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
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
// Removes the authenticated player from the ranked matchmaking queue.
// 204 whether or not the player was actually queued (idempotent).
func handleLeaveQueue(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
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
// Body: { "match_id": "uuid" }
// Records that the player accepts the proposed match.
// 404 if the match has expired or already been resolved.
func handleAcceptMatch(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MatchID string `json:"match_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
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
// Body: { "match_id": "uuid" }
// Records that the player declines the proposed match.
// 404 if the match has expired or already been resolved.
func handleDeclineMatch(svc *queue.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MatchID string `json:"match_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
