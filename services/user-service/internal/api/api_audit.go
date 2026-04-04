package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
)

// GET /api/v1/admin/audit-logs
func handleListAuditLogs(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		var filter store.AuditFilter

		if v := q.Get("actor"); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid actor UUID")
				return
			}
			filter.ActorID = &id
		}
		if v := q.Get("action"); v != "" {
			filter.Action = &v
		}
		if v := q.Get("target_type"); v != "" {
			filter.TargetType = &v
		}
		if v := q.Get("from"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid from date, expected RFC3339")
				return
			}
			filter.From = &t
		}
		if v := q.Get("to"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid to date, expected RFC3339")
				return
			}
			filter.To = &t
		}
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			filter.Limit = n
		}
		if v := q.Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "invalid offset")
				return
			}
			filter.Offset = n
		}

		logs, err := st.ListAuditLogs(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list audit logs")
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}
