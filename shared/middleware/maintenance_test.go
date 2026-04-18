package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubChecker implements featureflags.Checker so tests control the flag
// state without spinning up an HTTP server.
type stubChecker struct {
	enabled map[string]bool
}

func (s stubChecker) IsEnabled(name string, defaultValue bool) bool {
	v, ok := s.enabled[name]
	if !ok {
		return defaultValue
	}
	return v
}

func mustOK(t *testing.T, method, path string, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestMaintenance_Off_AllVerbsPass(t *testing.T) {
	flags := stubChecker{enabled: map[string]bool{"maintenance-mode": false}}
	h := Maintenance(flags)(okHandler())

	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		rec := mustOK(t, method, "/api/things", h)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: want 200 with flag OFF, got %d", method, rec.Code)
		}
	}
}

func TestMaintenance_On_BlocksMutations(t *testing.T) {
	flags := stubChecker{enabled: map[string]bool{"maintenance-mode": true}}
	h := Maintenance(flags)(okHandler())

	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		rec := mustOK(t, method, "/api/things", h)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: want 503 with flag ON, got %d", method, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"error":"maintenance"`) {
			t.Errorf("%s: body missing maintenance marker: %s", method, rec.Body.String())
		}
	}
}

func TestMaintenance_On_AllowsReads(t *testing.T) {
	flags := stubChecker{enabled: map[string]bool{"maintenance-mode": true}}
	h := Maintenance(flags)(okHandler())

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		rec := mustOK(t, method, "/api/things", h)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: reads should pass even in maintenance, got %d", method, rec.Code)
		}
	}
}

func TestMaintenance_On_AllowsHealthEndpoints(t *testing.T) {
	flags := stubChecker{enabled: map[string]bool{"maintenance-mode": true}}
	h := Maintenance(flags)(okHandler())

	for path := range MaintenancePaths {
		rec := mustOK(t, "POST", path, h)
		if rec.Code != http.StatusOK {
			t.Errorf("POST %s should pass allowlist even in maintenance, got %d", path, rec.Code)
		}
	}
}

func TestMaintenance_NilChecker_NeverBlocks(t *testing.T) {
	h := Maintenance(nil)(okHandler())
	rec := mustOK(t, "POST", "/api/things", h)
	if rec.Code != http.StatusOK {
		t.Errorf("nil checker should default to not blocking, got %d", rec.Code)
	}
}
