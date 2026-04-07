package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/recess/services/chat-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// POST /api/v1/players/{playerID}/dm
// Sends a direct message. playerID in path is the receiver.
// Broadcasts dm_received to the receiver's player channel.
func handleSendDM(st store.Store, pub *Publisher, uc UserChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receiverID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		senderID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req struct {
			Content string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Content == "" {
			writeError(w, http.StatusBadRequest, "content is required")
			return
		}
		if senderID == receiverID {
			writeError(w, http.StatusBadRequest, "cannot send a DM to yourself")
			return
		}

		// --- Privacy gate: respect receiver's allow_dms setting -----------------
		allowDMs, err := st.GetAllowDMs(r.Context(), receiverID)
		if err != nil {
			slog.Error("failed to fetch allow_dms setting", "receiver", receiverID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to check recipient settings")
			return
		}
		switch allowDMs {
		case "nobody":
			writeError(w, http.StatusForbidden, "player does not accept direct messages")
			return
		case "friends_only":
			if uc == nil {
				// No user-service client — fail closed.
				slog.Warn("friends_only DM gate: user checker unavailable, rejecting")
				writeError(w, http.StatusForbidden, "player only accepts DMs from friends")
				return
			}
			friends, err := uc.AreFriends(r.Context(), senderID.String(), receiverID.String())
			if err != nil {
				slog.Error("failed to check friendship", "sender", senderID, "receiver", receiverID, "error", err)
				writeError(w, http.StatusInternalServerError, "failed to verify friendship")
				return
			}
			if !friends {
				writeError(w, http.StatusForbidden, "player only accepts DMs from friends")
				return
			}
		}
		// "anyone" (default) — proceed normally.

		msg, err := st.SaveDM(r.Context(), senderID, receiverID, req.Content)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				writeError(w, http.StatusNotFound, "player not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to send message")
			return
		}

		pub.PublishPlayerEvent(r.Context(), receiverID, eventDMReceived, map[string]any{
			"message_id": msg.ID,
			"from":       senderID,
			"content":    msg.Content,
			"timestamp":  msg.CreatedAt,
		})

		writeJSON(w, http.StatusCreated, msg)
	}
}

// GET /api/v1/players/{playerID}/dm/{otherPlayerID}
func handleGetDMHistory(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		otherID, err := uuid.Parse(chi.URLParam(r, "otherPlayerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid other player id")
			return
		}

		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Must be one of the two participants or an owner.
		if callerID != playerID && callerID != otherID {
			role, ok := sharedmw.RoleFromContext(r.Context())
			if !ok || !hasRole(string(role), sharedmw.RoleOwner) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		limit := 50
		offset := 0
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		messages, err := st.GetDMHistory(r.Context(), playerID, otherID, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get messages")
			return
		}

		writeJSON(w, http.StatusOK, messages)
	}
}

// GET /api/v1/players/{playerID}/dm/unread
func handleGetUnreadDMCount(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		count, err := st.GetUnreadDMCount(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get unread count")
			return
		}

		writeJSON(w, http.StatusOK, map[string]int{"count": count})
	}
}

// GET /api/v1/players/{playerID}/dm/conversations
func handleListDMConversations(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		convos, err := st.ListDMConversations(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list conversations")
			return
		}

		writeJSON(w, http.StatusOK, convos)
	}
}

// POST /api/v1/dm/{messageID}/read
// Broadcasts dm_read to the sender's player channel.
func handleMarkDMRead(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if err := st.MarkDMRead(r.Context(), messageID, callerID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to mark message as read")
			return
		}

		// Notify the sender that their message was read.
		pub.PublishPlayerEvent(r.Context(), callerID, eventDMRead, map[string]any{
			"message_id": messageID,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /api/v1/players/{playerID}/dm/{otherPlayerID}/report
func handleReportDM(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		otherID, err := uuid.Parse(chi.URLParam(r, "otherPlayerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid other player id")
			return
		}

		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req struct {
			MessageID string `json:"message_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		messageID, err := uuid.Parse(req.MessageID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message_id")
			return
		}

		if callerID != playerID && callerID != otherID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		if err := st.ReportDM(r.Context(), messageID, playerID, otherID); err != nil {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
