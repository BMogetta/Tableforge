package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tableforge/services/user-service/internal/store"
	sharedmw "github.com/tableforge/shared/middleware"
)

func NewRouter(st store.Store, pub *Publisher, authMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		r.Use(authMW)

	// --- Profiles ------------------------------------------------------------
	r.Get("/api/v1/players/{playerID}/profile", handleGetProfile(st))
	r.Put("/api/v1/players/{playerID}/profile", handleUpsertProfile(st))

	// --- Friendships ---------------------------------------------------------
	r.Get("/api/v1/players/{playerID}/friends", handleListFriends(st))
	r.Get("/api/v1/players/{playerID}/friends/pending", handleListPendingRequests(st))
	r.Post("/api/v1/players/{playerID}/friends/{targetID}", handleSendFriendRequest(st))
	r.Put("/api/v1/players/{playerID}/friends/{requesterID}/accept", handleAcceptFriendRequest(st, pub))
	r.Delete("/api/v1/players/{playerID}/friends/{requesterID}/decline", handleDeclineFriendRequest(st))
	r.Delete("/api/v1/players/{playerID}/friends/{friendID}", handleRemoveFriend(st))

	// --- Blocks --------------------------------------------------------------
	r.Post("/api/v1/players/{playerID}/block/{targetID}", handleBlockPlayer(st))
	r.Delete("/api/v1/players/{playerID}/block/{targetID}", handleUnblockPlayer(st))

	// --- Mutes ---------------------------------------------------------------
	r.Post("/api/v1/players/{playerID}/mute", handleMutePlayer(st))
	r.Delete("/api/v1/players/{playerID}/mute/{mutedID}", handleUnmutePlayer(st))
	r.Get("/api/v1/players/{playerID}/mutes", handleGetMutes(st))

	// --- Achievements --------------------------------------------------------
	r.Get("/api/v1/players/{playerID}/achievements", handleListAchievements(st))

	// --- Player settings -----------------------------------------------------
	r.Get("/api/v1/players/{playerID}/settings", handleGetPlayerSettings(st))
	r.Put("/api/v1/players/{playerID}/settings", handleUpsertPlayerSettings(st))

	// --- Admin: bans ---------------------------------------------------------
	r.Group(func(r chi.Router) {
		r.Use(requireRole(sharedmw.RoleManager))
		r.Post("/api/v1/admin/players/{playerID}/ban", handleIssueBan(st, pub))
		r.Delete("/api/v1/admin/bans/{banID}", handleLiftBan(st, pub))
		r.Get("/api/v1/admin/players/{playerID}/bans", handleListBans(st))
	})

	// --- Admin: reports ------------------------------------------------------
	r.Group(func(r chi.Router) {
		r.Use(requireRole(sharedmw.RoleManager))
		r.Get("/api/v1/admin/reports", handleListPendingReports(st))
		r.Get("/api/v1/admin/players/{playerID}/reports", handleListReportsByPlayer(st))
		r.Put("/api/v1/admin/reports/{reportID}/review", handleReviewReport(st))
	})

	// --- Admin: players & allowed emails -------------------------------------
	r.Group(func(r chi.Router) {
		r.Use(requireRole(sharedmw.RoleManager))
		r.Get("/api/v1/admin/allowed-emails", handleListAllowedEmails(st))
		r.Post("/api/v1/admin/allowed-emails", handleAddAllowedEmail(st))
		r.Delete("/api/v1/admin/allowed-emails/{email}", handleRemoveAllowedEmail(st))
		r.Get("/api/v1/admin/players", handleListPlayers(st))
		r.With(requireRole(sharedmw.RoleOwner)).Put("/api/v1/admin/players/{playerID}/role", handleSetPlayerRole(st))
	})

	// --- Player: reports -----------------------------------------------------
	r.Post("/api/v1/players/{playerID}/report", handleCreateReport(st))
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

// hasRole returns true if the caller's role meets or exceeds the required role.
// Role hierarchy: player < manager < owner.
func hasRole(callerRole, required string) bool {
	rank := map[string]int{
		sharedmw.RolePlayer:  0,
		sharedmw.RoleManager: 1,
		sharedmw.RoleOwner:   2,
	}
	return rank[callerRole] >= rank[required]
}
