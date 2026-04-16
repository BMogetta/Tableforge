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
// Each token gets a unique jti (JWT ID) for traceability.
func SignToken(secret []byte, playerID uuid.UUID, username, role string) (string, error) {
	now := time.Now()
	c := middleware.Claims{
		PlayerID: playerID,
		Username: username,
		Role:     role,
		RegisteredClaims: gojwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(middleware.JWTTTL)),
		},
	}
	return gojwt.NewWithClaims(gojwt.SigningMethodHS256, c).SignedString(secret)
}

// SetSessionCookie writes the JWT as an HttpOnly session cookie.
//
// MaxAge deliberately tracks RefreshTTL, not JWTTTL: the cookie must outlive
// the JWT so that expired-token requests still reach the server with the
// cookie attached. The auth middleware then returns {"error":"token_expired"}
// and the frontend triggers a silent POST /auth/refresh that rotates the
// cookie. If MaxAge matched JWTTTL the browser would delete the cookie first,
// the server would see a missing-cookie request and return a bare 401 with
// no "token_expired" hint, and the refresh flow would never fire — silently
// logging the user out every JWTTTL window.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(middleware.RefreshTTL.Seconds()),
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

// SetRefreshCookie writes the refresh token (session UUID) as an HttpOnly cookie.
// Path is restricted to /auth/refresh so the cookie is only sent on refresh requests.
func SetRefreshCookie(w http.ResponseWriter, sessionID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.RefreshCookieName,
		Value:    sessionID,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(middleware.RefreshTTL.Seconds()),
	})
}

// ClearRefreshCookie removes the refresh cookie.
func ClearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.RefreshCookieName,
		Value:    "",
		Path:     "/auth/refresh",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// ClearAllCookies removes both session and refresh cookies.
func ClearAllCookies(w http.ResponseWriter) {
	ClearSessionCookie(w)
	ClearRefreshCookie(w)
}
