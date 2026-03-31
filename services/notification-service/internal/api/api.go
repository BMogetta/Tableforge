package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apierrors "github.com/tableforge/shared/errors"
	sharedmw "github.com/tableforge/shared/middleware"
	sharedws "github.com/tableforge/shared/ws"
	"github.com/tableforge/notification-service/internal/store"
)

const (
	readNotificationCutoff     = 30 * 24 * time.Hour
	friendRequestActionExpiry  = 7 * 24 * time.Hour
	roomInvitationActionExpiry = 24 * time.Hour
)

var (
	errNotificationNotFound      = errors.New("notification not found")
	errActionNotSupported        = errors.New("this notification type does not support actions")
	errActionAlreadyTaken        = errors.New("action already taken")
	errNotificationActionExpired = errors.New("notification action has expired")
)

// Publisher delivers real-time notifications to clients via Redis pub/sub.
type Publisher interface {
	PublishToPlayer(ctx context.Context, playerID uuid.UUID, event sharedws.Event)
}

// Handler holds the API dependencies.
type Handler struct {
	store *store.Store
	pub   Publisher
	log   *slog.Logger
}

func New(st *store.Store, pub Publisher, log *slog.Logger) *Handler {
	return &Handler{store: st, pub: pub, log: log}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/players/{playerID}/notifications", h.list)
	r.Post("/notifications/{notificationID}/read", h.markRead)
	r.Post("/notifications/{notificationID}/accept", h.accept)
	r.Post("/notifications/{notificationID}/decline", h.decline)

	// Internal endpoint — called by other services to create notifications.
	// Must only be reachable on the internal network, not exposed via Traefik.
	r.Post("/internal/notifications", h.create)
}

// GET /players/{playerID}/notifications?include_read=false
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, apierrors.InvalidInput)
		return
	}
	callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
		return
	}
	if callerID != playerID {
		writeError(w, http.StatusForbidden, apierrors.Forbidden)
		return
	}

	includeRead := r.URL.Query().Get("include_read") == "true"
	cutoff := time.Now().Add(-readNotificationCutoff)
	notifications, err := h.store.List(r.Context(), playerID, includeRead, cutoff)
	if err != nil {
		h.log.Error("list notifications", "player_id", playerID, "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.InternalError)
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

// POST /notifications/{notificationID}/read
func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	notificationID, playerID, ok := h.parseNotificationRequest(w, r)
	if !ok {
		return
	}
	n, err := h.store.Get(r.Context(), notificationID)
	if err != nil {
		writeError(w, http.StatusNotFound, errNotificationNotFound.Error())
		return
	}
	if n.PlayerID != playerID {
		writeError(w, http.StatusNotFound, errNotificationNotFound.Error())
		return
	}
	if err := h.store.MarkRead(r.Context(), notificationID); err != nil {
		h.log.Error("mark read", "notification_id", notificationID, "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.InternalError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /notifications/{notificationID}/accept
func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	h.takeAction(w, r, "accepted")
}

// POST /notifications/{notificationID}/decline
func (h *Handler) decline(w http.ResponseWriter, r *http.Request) {
	h.takeAction(w, r, "declined")
}

func (h *Handler) takeAction(w http.ResponseWriter, r *http.Request, action string) {
	notificationID, playerID, ok := h.parseNotificationRequest(w, r)
	if !ok {
		return
	}
	n, err := h.store.Get(r.Context(), notificationID)
	if err != nil {
		writeError(w, http.StatusNotFound, errNotificationNotFound.Error())
		return
	}
	if n.PlayerID != playerID {
		writeError(w, http.StatusNotFound, errNotificationNotFound.Error())
		return
	}
	if n.Type == store.NotificationTypeBanIssued {
		writeError(w, http.StatusUnprocessableEntity, errActionNotSupported.Error())
		return
	}
	if n.ActionTaken != nil {
		writeError(w, http.StatusConflict, errActionAlreadyTaken.Error())
		return
	}
	if n.ActionExpiresAt != nil && time.Now().After(*n.ActionExpiresAt) {
		writeError(w, http.StatusGone, errNotificationActionExpired.Error())
		return
	}
	if err := h.store.SetAction(r.Context(), notificationID, action); err != nil {
		h.log.Error("set notification action", "notification_id", notificationID, "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.InternalError)
		return
	}
	_ = h.store.MarkRead(r.Context(), notificationID)
	w.WriteHeader(http.StatusNoContent)
}

// POST /internal/notifications
// Body: store.CreateParams JSON. No auth — internal network only.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var p store.CreateParams
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, apierrors.InvalidInput)
		return
	}
	n, err := h.store.Create(r.Context(), p)
	if err != nil {
		h.log.Error("create notification", "player_id", p.PlayerID, "type", p.Type, "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.InternalError)
		return
	}
	h.pub.PublishToPlayer(r.Context(), n.PlayerID, sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: n,
	})
	writeJSON(w, http.StatusCreated, n)
}

func (h *Handler) parseNotificationRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	notificationID, err := uuid.Parse(chi.URLParam(r, "notificationID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, apierrors.InvalidInput)
		return uuid.Nil, uuid.Nil, false
	}
	playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, apierrors.Unauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	return notificationID, playerID, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
