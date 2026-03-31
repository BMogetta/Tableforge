package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/services/user-service/internal/store"
	sharedmw "github.com/tableforge/shared/middleware"
)

func handleIssueBan(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		var req struct {
			Reason    *string `json:"reason"`
			ExpiresAt *string `json:"expires_at"` // RFC3339; omit for permanent
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		var expiresAt *time.Time
		if req.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid expires_at format, expected RFC3339")
				return
			}
			expiresAt = &t
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		ban, err := st.IssueBan(r.Context(), store.IssueBanParams{
			PlayerID:  playerID,
			BannedBy:  callerID,
			Reason:    req.Reason,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue ban")
			return
		}
		pub.PublishPlayerBanned(r.Context(), ban)
		writeJSON(w, http.StatusCreated, ban)
	}
}

func handleLiftBan(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		banID, err := uuid.Parse(chi.URLParam(r, "banID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ban id")
			return
		}

		ban, err := st.GetBan(r.Context(), banID)
		if err != nil {
			writeError(w, http.StatusNotFound, "ban not found")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if err := st.LiftBan(r.Context(), banID, callerID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "ban not found or already lifted")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to lift ban")
			return
		}
		pub.PublishPlayerUnbanned(r.Context(), banID, ban.PlayerID, callerID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListBans(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		bans, err := st.ListBans(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list bans")
			return
		}
		writeJSON(w, http.StatusOK, bans)
	}
}
