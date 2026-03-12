package auth

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
)

type contextKey int

const (
	keyPlayerID contextKey = iota
	keyUsername
	keyRole
)

// Middleware returns a chi-compatible middleware that requires a valid session
// cookie. On success it injects the player ID and username into the context.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		c, err := verifyToken(h.jwtSecret, cookie.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		player, err := h.store.GetPlayer(r.Context(), c.PlayerID)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), keyPlayerID, c.PlayerID)
		ctx = context.WithValue(ctx, keyUsername, c.Username)
		ctx = context.WithValue(ctx, keyRole, player.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RoleFromContext(ctx context.Context) (store.PlayerRole, bool) {
	role, ok := ctx.Value(keyRole).(store.PlayerRole)
	return role, ok
}

// PlayerIDFromContext extracts the authenticated player's ID.
// Returns uuid.Nil, false if not present.
func PlayerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(keyPlayerID).(uuid.UUID)
	return id, ok
}

// UsernameFromContext extracts the authenticated player's username.
func UsernameFromContext(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(keyUsername).(string)
	return u, ok
}
