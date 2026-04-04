package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role constants — used across all services for role comparisons.
// These mirror the player_role enum in Postgres but are decoupled from any
// store package so the middleware can be imported without DB dependencies.
const (
	RolePlayer  = "player"
	RoleManager = "manager"
	RoleOwner   = "owner"
)

const (
	CookieName = "tf_session"
	JWTTTL     = 7 * 24 * time.Hour
)

// Claims is the JWT payload shared across all services.
type Claims struct {
	PlayerID uuid.UUID `json:"pid"`
	Username string    `json:"usr"`
	Role     string    `json:"role"`
	jwt.RegisteredClaims
}

// ContextKey is the type for context keys to avoid collisions.
type contextKey int

const (
	keyPlayerID contextKey = iota
	keyUsername
	keyRole
)

// PlayerIDFromContext extracts the authenticated player's ID from the context.
func PlayerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(keyPlayerID).(uuid.UUID)
	return id, ok
}

// UsernameFromContext extracts the authenticated player's username.
func UsernameFromContext(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(keyUsername).(string)
	return u, ok
}

// RoleFromContext extracts the authenticated player's role.
func RoleFromContext(ctx context.Context) (string, bool) {
	r, ok := ctx.Value(keyRole).(string)
	return r, ok
}

// VerifyToken parses and validates a JWT string, returning the claims.
func VerifyToken(secret []byte, raw string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	c, ok := tok.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return c, nil
}

// Require returns a chi-compatible middleware that validates the session cookie
// and injects player data into the context. No DB call — role comes from JWT.
func Require(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				slog.Warn("auth: unauthorized", "method", r.Method, "path", r.URL.Path, "reason", "missing cookie")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			c, err := VerifyToken(jwtSecret, cookie.Value)
			if err != nil {
				slog.Warn("auth: unauthorized", "method", r.Method, "path", r.URL.Path, "reason", "invalid token")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), keyPlayerID, c.PlayerID)
			ctx = context.WithValue(ctx, keyUsername, c.Username)
			ctx = context.WithValue(ctx, keyRole, c.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns a middleware that enforces a minimum role level.
// Hierarchy: player < manager < owner
func RequireRole(minimum string) func(http.Handler) http.Handler {
	rank := map[string]int{
		RolePlayer:  1,
		RoleManager: 2,
		RoleOwner:   3,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok || rank[role] < rank[minimum] {
				slog.Warn("auth: insufficient role", "method", r.Method, "path", r.URL.Path, "role", role, "required", minimum)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ContextWithPlayer injects player data into a context. Use in tests only.
func ContextWithPlayer(ctx context.Context, id uuid.UUID, username, role string) context.Context {
	ctx = context.WithValue(ctx, keyPlayerID, id)
	ctx = context.WithValue(ctx, keyUsername, username)
	ctx = context.WithValue(ctx, keyRole, role)
	return ctx
}
