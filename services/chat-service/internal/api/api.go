package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"github.com/recess/services/chat-service/internal/store"
	"github.com/recess/shared/featureflags"
	sharedmw "github.com/recess/shared/middleware"
)

// UserChecker abstracts the gRPC call to user-service for friendship checks.
// Nil is accepted — the DM handler skips the friends_only gate when nil (tests).
type UserChecker interface {
	// AreFriends returns true if the two players have an accepted friendship.
	AreFriends(ctx context.Context, playerAID, playerBID string) (bool, error)
}

// FlagChatEnabled is the Unleash flag that gates message sends (room + DM).
// When OFF, send endpoints return 503; reads, reports, and moderation still
// work so a temporarily-disabled chat can still be inspected/cleaned up.
const FlagChatEnabled = "chat-enabled"

func NewRouter(st store.Store, pub *Publisher, authMW func(http.Handler) http.Handler, schemas *sharedmw.SchemaRegistry, serviceName string, uc UserChecker, flags featureflags.Checker) http.Handler {
	r := chi.NewRouter()
	r.Use(sharedmw.Recoverer)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Use(sharedmw.MaxBodySize(64 << 10)) // 64 KB

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{},
	).ServeHTTP)

	// validate wraps schema validation; no-op when schemas is nil (tests).
	validate := func(name string) func(http.Handler) http.Handler {
		if schemas == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return schemas.ValidateBody(name)
	}

	sendGate := chatSendGate(flags)

	r.Group(func(r chi.Router) {
		r.Use(authMW)

	// --- Room chat -----------------------------------------------------------
	r.With(sendGate, validate("send_room_message.request")).Post("/api/v1/rooms/{roomID}/messages", handleSendRoomMessage(st, pub))
	r.Get("/api/v1/rooms/{roomID}/messages", handleGetRoomMessages(st))
	r.Post("/api/v1/rooms/{roomID}/messages/{messageID}/report", handleReportRoomMessage(st))
	r.With(requireRole(sharedmw.RoleManager)).
		Delete("/api/v1/rooms/{roomID}/messages/{messageID}", handleHideRoomMessage(st, pub))

	// --- Direct messages -----------------------------------------------------
	r.With(sendGate, validate("send_dm.request")).Post("/api/v1/players/{playerID}/dm", handleSendDM(st, pub, uc))
	r.Get("/api/v1/players/{playerID}/dm/conversations", handleListDMConversations(st))
	r.Get("/api/v1/players/{playerID}/dm/unread", handleGetUnreadDMCount(st))
	r.Get("/api/v1/players/{playerID}/dm/{otherPlayerID}", handleGetDMHistory(st))
	r.Post("/api/v1/dm/{messageID}/read", handleMarkDMRead(st, pub))
	r.With(validate("report_dm.request")).Post("/api/v1/players/{playerID}/dm/{otherPlayerID}/report", handleReportDM(st))
	}) // end auth group

	return r
}

// chatSendGate returns a middleware that rejects requests with 503 when the
// chat-enabled flag is OFF. Default when the flag is unknown (cold start,
// Unleash unreachable) is ON, so chat stays usable by default.
func chatSendGate(flags featureflags.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if flags != nil && !flags.IsEnabled(FlagChatEnabled, true) {
				writeError(w, http.StatusServiceUnavailable, "chat_disabled")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
