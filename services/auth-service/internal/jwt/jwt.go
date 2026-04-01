package jwt

import (
	"net/http"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/recess/shared/middleware"
)

// SignToken issues a signed JWT for the given player.
// Role is embedded so downstream services don't need a DB call to authorize.
func SignToken(secret []byte, playerID uuid.UUID, username, role string) (string, error) {
	now := time.Now()
	c := middleware.Claims{
		PlayerID: playerID,
		Username: username,
		Role:     role,
		RegisteredClaims: gojwt.RegisteredClaims{
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(middleware.JWTTTL)),
		},
	}
	return gojwt.NewWithClaims(gojwt.SigningMethodHS256, c).SignedString(secret)
}

// SetSessionCookie writes the JWT as an HttpOnly session cookie.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(middleware.JWTTTL.Seconds()),
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
