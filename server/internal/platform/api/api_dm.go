package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tableforge/server/internal/platform/auth"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

// POST /api/v1/players/{playerID}/dm
// Sends a direct message to playerID (the receiver).
// player_id in the body is the sender. Sender must differ from receiver.
// Broadcasts dm_received to the receiver's player channel.
func handleSendDM(st store.Store, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		receiverID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
			Content  string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Content == "" {
			writeError(w, http.StatusBadRequest, "content is required")
			return
		}

		senderID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		if senderID == receiverID {
			writeError(w, http.StatusBadRequest, "cannot send a DM to yourself")
			return
		}

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

		hub.BroadcastToPlayer(receiverID, ws.Event{
			Type: ws.EventDMReceived,
			Payload: map[string]any{
				"message_id": msg.ID,
				"from":       senderID,
				"content":    msg.Content,
				"timestamp":  msg.CreatedAt,
			},
		})

		writeJSON(w, http.StatusCreated, msg)
	}
}

// GET /api/v1/players/{playerID}/dm/{otherPlayerID}
// Returns the full conversation history between the two players.
// Caller must be one of the two participants or hold owner role.
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

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		// Must be one of the two participants or an owner.
		if callerID != playerID && callerID != otherID {
			role, ok := auth.RoleFromContext(r.Context())
			if !ok || role != store.RoleOwner {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		messages, err := st.GetDMHistory(r.Context(), playerID, otherID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get messages")
			return
		}

		writeJSON(w, http.StatusOK, messages)
	}
}

// GET /api/v1/players/{playerID}/dm/unread
// Returns the unread DM count for playerID.
// Caller must match playerID.
func handleGetUnreadDMCount(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
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

// POST /api/v1/dm/{messageID}/read
// Marks a direct message as read. No-op if already read.
// Broadcasts dm_read to the sender's player channel.
func handleMarkDMRead(st store.Store, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		if err := st.MarkDMRead(r.Context(), messageID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to mark message as read")
			return
		}

		// Notify the sender that their message was read.
		hub.BroadcastToPlayer(callerID, ws.Event{
			Type:    ws.EventDMRead,
			Payload: map[string]any{"message_id": messageID},
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /api/v1/players/{playerID}/dm/{otherPlayerID}/report
// Reports a direct message. Caller must be one of the two participants.
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

		var req struct {
			PlayerID  string `json:"player_id"`
			MessageID string `json:"message_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		messageID, err := uuid.Parse(req.MessageID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message_id")
			return
		}

		// Caller must be one of the two participants.
		if callerID != playerID && callerID != otherID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		if err := st.ReportDM(r.Context(), messageID); err != nil {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
