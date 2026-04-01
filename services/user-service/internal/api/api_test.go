package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/recess/shared/middleware"
)

// newTestRouter creates an http.Handler backed by the given mock store
// and a stub publisher. Auth middleware is a passthrough that injects the
// caller identity from the request context (set via the helper functions).
func newTestRouter(st *mockStore) http.Handler {
	noopAuth := func(next http.Handler) http.Handler { return next }
	return NewRouter(st, stubPublisher(), noopAuth)
}

// withAuth returns a new request whose context carries the given player
// identity. role should be one of sharedmw.RolePlayer, RoleManager, RoleOwner.
func withAuth(r *http.Request, playerID uuid.UUID, role string) *http.Request {
	ctx := sharedmw.ContextWithPlayer(r.Context(), playerID, "testuser", role)
	return r.WithContext(ctx)
}

// postJSONAs sends an authenticated POST with a JSON body and returns the
// recorded response.
func postJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// putJSONAs sends an authenticated PUT with a JSON body.
func putJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPut, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// getJSONAs sends an authenticated GET and returns the recorded response.
func getJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// deleteJSONAs sends an authenticated DELETE and returns the recorded response.
func deleteJSONAs(t *testing.T, handler http.Handler, path string, playerID uuid.UUID, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req = withAuth(req, playerID, role)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// decodeResponse is a generic helper to unmarshal a JSON response body.
func decodeResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return v
}
