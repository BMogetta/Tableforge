package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer is a chi-compatible middleware that recovers from panics,
// logs the stack trace via slog, and returns a generic 500 JSON error.
// Unlike chi's built-in Recoverer, it never exposes internal details to clients.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				slog.Error("panic recovered",
					"error", rv,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)

				if r.Header.Get("Connection") != "Upgrade" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"internal error"}`))
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}
