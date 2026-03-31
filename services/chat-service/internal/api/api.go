package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	"github.com/tableforge/services/chat-service/internal/store"
	sharedmw "github.com/tableforge/shared/middleware"
)

func NewRouter(st store.Store, pub *Publisher, authMW func(http.Handler) http.Handler, serviceName string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		r.Use(authMW)

	// --- Room chat -----------------------------------------------------------
	r.Post("/api/v1/rooms/{roomID}/messages", handleSendRoomMessage(st, pub))
	r.Get("/api/v1/rooms/{roomID}/messages", handleGetRoomMessages(st))
	r.Post("/api/v1/rooms/{roomID}/messages/{messageID}/report", handleReportRoomMessage(st))
	r.With(requireRole(sharedmw.RoleManager)).
		Delete("/api/v1/rooms/{roomID}/messages/{messageID}", handleHideRoomMessage(st, pub))

	// --- Direct messages -----------------------------------------------------
	r.Post("/api/v1/players/{playerID}/dm", handleSendDM(st, pub))
	r.Get("/api/v1/players/{playerID}/dm/unread", handleGetUnreadDMCount(st))
	r.Get("/api/v1/players/{playerID}/dm/{otherPlayerID}", handleGetDMHistory(st))
	r.Post("/api/v1/dm/{messageID}/read", handleMarkDMRead(st, pub))
	r.Post("/api/v1/players/{playerID}/dm/{otherPlayerID}/report", handleReportDM(st))
	}) // end auth group

	return r
}

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callerRole, ok := sharedmw.RoleFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !hasRole(string(callerRole), role) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasRole(callerRole, required string) bool {
	rank := map[string]int{
		sharedmw.RolePlayer:  0,
		sharedmw.RoleManager: 1,
		sharedmw.RoleOwner:   2,
	}
	return rank[callerRole] >= rank[required]
}
