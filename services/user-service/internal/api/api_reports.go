package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

func handleCreateReport(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		var req struct {
			ReportedID string          `json:"reported_id"`
			Reason     string          `json:"reason"`
			Context    json.RawMessage `json:"context"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required")
			return
		}
		reportedID, err := uuid.Parse(req.ReportedID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid reported_id")
			return
		}
		if reportedID == playerID {
			writeError(w, http.StatusBadRequest, "cannot report yourself")
			return
		}

		ctx := json.RawMessage(`{}`)
		if req.Context != nil {
			ctx = req.Context
		}

		report, err := st.CreateReport(r.Context(), store.CreateReportParams{
			ReporterID: playerID,
			ReportedID: reportedID,
			Reason:     req.Reason,
			Context:    ctx,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create report")
			return
		}
		writeJSON(w, http.StatusCreated, report)
	}
}

func handleListPendingReports(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reports, err := st.ListPendingReports(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list reports")
			return
		}
		writeJSON(w, http.StatusOK, reports)
	}
}

func handleListReportsByPlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		reports, err := st.ListReportsByPlayer(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list reports")
			return
		}
		writeJSON(w, http.StatusOK, reports)
	}
}

func handleReviewReport(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportID, err := uuid.Parse(chi.URLParam(r, "reportID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid report id")
			return
		}
		var req struct {
			Resolution *string `json:"resolution"`
			BanID      *string `json:"ban_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())

		var banID *uuid.UUID
		if req.BanID != nil {
			parsed, err := uuid.Parse(*req.BanID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid ban_id")
				return
			}
			banID = &parsed
		}

		if err := st.ReviewReport(r.Context(), store.ReviewReportParams{
			ReportID:   reportID,
			ReviewedBy: callerID,
			Resolution: req.Resolution,
			BanID:      banID,
		}); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "report not found or already reviewed")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to review report")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
