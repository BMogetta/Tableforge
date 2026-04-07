package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

type broadcastRequest struct {
	Message string `json:"message"`
	Type    string `json:"type"` // "info" | "warning"
}

// POST /api/v1/admin/broadcast
func handleBroadcast(pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req broadcastRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Message == "" {
			writeError(w, http.StatusBadRequest, "message is required")
			return
		}
		if req.Type == "" {
			req.Type = "info"
		}
		if req.Type != "info" && req.Type != "warning" {
			writeError(w, http.StatusBadRequest, "type must be info or warning")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		pub.PublishBroadcast(r.Context(), req.Message, req.Type, callerID.String())

		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /api/v1/admin/allowed-emails
func handleListAllowedEmails(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := st.ListAllowedEmails(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list allowed emails")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

type addAllowedEmailRequest struct {
	Email string          `json:"email"`
	Role  store.PlayerRole `json:"role"`
}

// POST /api/v1/admin/allowed-emails
func handleAddAllowedEmail(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addAllowedEmailRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if req.Role == "" {
			req.Role = store.RolePlayer
		}

		callerRole, _ := sharedmw.RoleFromContext(r.Context())
		if callerRole == sharedmw.RoleManager && string(req.Role) != sharedmw.RolePlayer {
			writeError(w, http.StatusForbidden, "managers can only invite players")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		entry, err := st.AddAllowedEmail(r.Context(), store.AddAllowedEmailParams{
			Email:     req.Email,
			Role:      req.Role,
			InvitedBy: &callerID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add email")
			return
		}
		_ = st.LogAction(r.Context(), callerID, "email_added", "allowed_email", req.Email, map[string]any{
			"role": string(req.Role),
		})
		writeJSON(w, http.StatusCreated, entry)
	}
}

// DELETE /api/v1/admin/allowed-emails/{email}
func handleRemoveAllowedEmail(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := chi.URLParam(r, "email")
		if email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		if err := st.RemoveAllowedEmail(r.Context(), email); err != nil {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		_ = st.LogAction(r.Context(), callerID, "email_removed", "allowed_email", email, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /api/v1/admin/players
func handleListPlayers(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		players, err := st.ListPlayers(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list players")
			return
		}
		writeJSON(w, http.StatusOK, players)
	}
}

type setRoleRequest struct {
	Role store.PlayerRole `json:"role"`
}

// PUT /api/v1/admin/players/{playerID}/role
func handleSetPlayerRole(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		var req setRoleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Role == "" {
			writeError(w, http.StatusBadRequest, "role is required")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID == playerID {
			callerRole, _ := sharedmw.RoleFromContext(r.Context())
			if callerRole == sharedmw.RoleOwner && string(req.Role) != sharedmw.RoleOwner {
				writeError(w, http.StatusForbidden, "cannot demote yourself")
				return
			}
		}

		if err := st.SetPlayerRole(r.Context(), playerID, req.Role); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = st.LogAction(r.Context(), callerID, "role_changed", "player", playerID.String(), map[string]any{
			"new_role": string(req.Role),
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
