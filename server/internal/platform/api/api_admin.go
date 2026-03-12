package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/auth"
	"github.com/tableforge/server/internal/platform/store"
)

// --- Allowed emails ----------------------------------------------------------

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
	Email string           `json:"email"`
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

		// Managers can only invite players, not other managers or owners.
		callerRole, _ := auth.RoleFromContext(r.Context())
		if callerRole == store.RoleManager && req.Role != store.RolePlayer {
			writeError(w, http.StatusForbidden, "managers can only invite players")
			return
		}

		callerID, _ := auth.PlayerIDFromContext(r.Context())
		entry, err := st.AddAllowedEmail(r.Context(), store.AddAllowedEmailParams{
			Email:     req.Email,
			Role:      req.Role,
			InvitedBy: &callerID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add email")
			return
		}
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
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Players -----------------------------------------------------------------

// GET /api/v1/admin/players
func handleListPlayers(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := st.ListPlayers(r.Context())
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

		// Prevent self-demotion.
		callerID, _ := auth.PlayerIDFromContext(r.Context())
		if callerID == playerID {
			callerRole, _ := auth.RoleFromContext(r.Context())
			if callerRole == store.RoleOwner && req.Role != store.RoleOwner {
				writeError(w, http.StatusForbidden, "cannot demote yourself")
				return
			}
		}

		if err := st.SetPlayerRole(r.Context(), playerID, req.Role); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
