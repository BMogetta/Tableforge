package middleware

import (
	"net/http"

	"github.com/recess/shared/featureflags"
)

// MaintenancePaths is the set of URL paths that always pass through, even
// when maintenance-mode is ON. Kubernetes/Traefik probe these and we don't
// want the cluster to mark the pod unhealthy (which would cascade: pod
// restart → more maintenance → more restarts).
var MaintenancePaths = map[string]struct{}{
	"/healthz": {},
	"/readyz":  {},
	"/metrics": {},
}

// Maintenance returns a middleware that serves 503 for mutating requests
// (POST/PUT/PATCH/DELETE) whenever the `maintenance-mode` flag is ON.
// Read-only verbs (GET/HEAD/OPTIONS) always pass — the UI needs to keep
// rendering state so users see the maintenance banner instead of a hard
// error.
//
// Accepts a Checker (nil-safe) so tests can pass a stub. When client is nil
// the flag defaults to false (never block) to keep services functional if
// Unleash is unreachable at boot.
func Maintenance(flags featureflags.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutation(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := MaintenancePaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if flags != nil && flags.IsEnabled("maintenance-mode", false) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"maintenance"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
