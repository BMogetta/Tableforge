package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/chat-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// POST /api/v1/rooms/{roomID}/messages
func handleSendRoomMessage(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
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

		// Verify the player is a participant, not a spectator.
		participant, err := st.IsRoomParticipant(r.Context(), roomID, playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify participant")
			return
		}
		if !participant {
			writeError(w, http.StatusForbidden, "spectators cannot send messages")
			return
		}

		msg, err := st.SaveRoomMessage(r.Context(), roomID, playerID, req.Content)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save message")
			return
		}

		pub.PublishRoomEvent(r.Context(), roomID, eventChatMessage, map[string]any{
			"message_id": msg.ID,
			"room_id":    msg.RoomID,
			"player_id":  msg.PlayerID,
			"content":    msg.Content,
			"reported":   msg.Reported,
			"hidden":     msg.Hidden,
			"timestamp":  msg.CreatedAt,
		})

		writeJSON(w, http.StatusCreated, msg)
	}
}

// GET /api/v1/rooms/{roomID}/messages
func handleGetRoomMessages(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		participant, err := st.IsRoomParticipant(r.Context(), roomID, playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify participant")
			return
		}
		if !participant {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		messages, err := st.GetRoomMessages(r.Context(), roomID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get messages")
			return
		}

		writeJSON(w, http.StatusOK, messages)
	}
}

// POST /api/v1/rooms/{roomID}/messages/{messageID}/report
func handleReportRoomMessage(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		participant, err := st.IsRoomParticipant(r.Context(), roomID, playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify participant")
			return
		}
		if !participant {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		if err := st.ReportRoomMessage(r.Context(), messageID); err != nil {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DELETE /api/v1/rooms/{roomID}/messages/{messageID}
// Manager-only. Broadcasts chat_message_hidden to the room.
func handleHideRoomMessage(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		if err := st.HideRoomMessage(r.Context(), messageID); err != nil {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}

		pub.PublishRoomEvent(r.Context(), roomID, eventChatMessageHidden, map[string]any{
			"message_id": messageID,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
