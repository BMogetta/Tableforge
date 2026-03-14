package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/notification"
)

// GET /api/v1/players/{playerID}/notifications?include_read=false
// Returns inbox notifications for the player.
// Read notifications older than 30 days are always excluded.
// Query param include_read=true includes read notifications within the cutoff.
func handleListNotifications(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		callerID := r.URL.Query().Get("player_id")
		if callerID == "" {
			writeError(w, http.StatusBadRequest, "player_id query param required")
			return
		}
		if callerID != playerID.String() {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		includeRead := r.URL.Query().Get("include_read") == "true"

		notifications, err := svc.List(r.Context(), playerID, includeRead)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list notifications")
			return
		}

		writeJSON(w, http.StatusOK, notifications)
	}
}

// POST /api/v1/notifications/{notificationID}/read
// Body: { "player_id": "uuid" }
// Marks a notification as read. Idempotent.
func handleMarkNotificationRead(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		notificationID, err := uuid.Parse(chi.URLParam(r, "notificationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid notification_id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		playerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		if err := svc.MarkRead(r.Context(), playerID, notificationID); err != nil {
			if errors.Is(err, notification.ErrNotificationNotFound) {
				writeError(w, http.StatusNotFound, "notification not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to mark notification read")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /api/v1/notifications/{notificationID}/accept
// Body: { "player_id": "uuid" }
// Records that the player accepts the notification action (friend request, room invitation).
func handleAcceptNotification(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeNotificationAction(w, r, svc, "accepted")
	}
}

// POST /api/v1/notifications/{notificationID}/decline
// Body: { "player_id": "uuid" }
// Records that the player declines the notification action (friend request, room invitation).
func handleDeclineNotification(svc *notification.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeNotificationAction(w, r, svc, "declined")
	}
}

func writeNotificationAction(w http.ResponseWriter, r *http.Request, svc *notification.Service, action string) {
	notificationID, err := uuid.Parse(chi.URLParam(r, "notificationID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification_id")
		return
	}

	var req struct {
		PlayerID string `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid player_id")
		return
	}

	if err := svc.TakeAction(r.Context(), playerID, notificationID, action); err != nil {
		switch {
		case errors.Is(err, notification.ErrNotificationNotFound):
			writeError(w, http.StatusNotFound, "notification not found")
		case errors.Is(err, notification.ErrActionAlreadyTaken):
			writeError(w, http.StatusConflict, "action already taken")
		case errors.Is(err, notification.ErrNotificationActionExpired):
			writeError(w, http.StatusGone, "notification action has expired")
		case errors.Is(err, notification.ErrActionNotSupported):
			writeError(w, http.StatusUnprocessableEntity, "this notification does not support actions")
		default:
			writeError(w, http.StatusInternalServerError, "failed to process action")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
