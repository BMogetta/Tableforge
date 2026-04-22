package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/recess/shared/middleware"
	"github.com/recess/services/user-service/internal/store"
)

func TestCreateReport(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	reporter := uuid.New()
	reported := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/report", reporter)
	rec := postJSONAs(t, router, path, reporter, sharedmw.RolePlayer, map[string]any{
		"reported_id": reported.String(),
		"reason":      "offensive language",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	reports, _ := st.ListPendingReports(context.Background(), 50, 0)
	if len(reports) != 1 {
		t.Fatalf("expected 1 pending report, got %d", len(reports))
	}
	if reports[0].ReporterID != reporter {
		t.Fatalf("expected reporter=%s, got %s", reporter, reports[0].ReporterID)
	}
}

func TestCreateReport_SelfReport(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/report", player)
	rec := postJSONAs(t, router, path, player, sharedmw.RolePlayer, map[string]any{
		"reported_id": player.String(),
		"reason":      "self-report",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListPendingReports(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()

	// Seed two reports directly.
	if _, err := st.CreateReport(context.Background(), store.CreateReportParams{
		ReporterID: uuid.New(),
		ReportedID: uuid.New(),
		Reason:     "spam",
		Context:    []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateReport(context.Background(), store.CreateReportParams{
		ReporterID: uuid.New(),
		ReportedID: uuid.New(),
		Reason:     "harassment",
		Context:    []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	rec := getJSONAs(t, router, "/api/v1/admin/reports", manager, sharedmw.RoleManager)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var reports []store.PlayerReport
	reports = decodeResponse[[]store.PlayerReport](t, rec)
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
}

func TestReviewReport(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	manager := uuid.New()

	// Seed a report.
	rpt, _ := st.CreateReport(context.Background(), store.CreateReportParams{
		ReporterID: uuid.New(),
		ReportedID: uuid.New(),
		Reason:     "cheating",
		Context:    []byte(`{}`),
	})

	path := fmt.Sprintf("/api/v1/admin/reports/%s/review", rpt.ID)
	resolution := "warning issued"
	rec := putJSONAs(t, router, path, manager, sharedmw.RoleManager, map[string]any{
		"resolution": resolution,
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the report is now reviewed.
	pending, _ := st.ListPendingReports(context.Background(), 50, 0)
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending reports after review, got %d", len(pending))
	}
}
