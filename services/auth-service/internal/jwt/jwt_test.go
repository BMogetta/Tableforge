package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/recess/shared/middleware"
)

var testSecret = []byte("test-secret-key-for-jwt-tests")

func TestSignToken_ValidClaims(t *testing.T) {
	playerID := uuid.New()
	token, err := SignToken(testSecret, playerID, "alice", "player")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	parsed, err := gojwt.ParseWithClaims(token, &middleware.Claims{}, func(*gojwt.Token) (any, error) {
		return testSecret, nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	claims := parsed.Claims.(*middleware.Claims)
	if claims.PlayerID != playerID {
		t.Errorf("PlayerID = %v, want %v", claims.PlayerID, playerID)
	}
	if claims.Username != "alice" {
		t.Errorf("Username = %q, want %q", claims.Username, "alice")
	}
	if claims.Role != "player" {
		t.Errorf("Role = %q, want %q", claims.Role, "player")
	}
}

func TestSignToken_WrongSecretFails(t *testing.T) {
	playerID := uuid.New()
	token, err := SignToken(testSecret, playerID, "alice", "player")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	_, err = gojwt.ParseWithClaims(token, &middleware.Claims{}, func(*gojwt.Token) (any, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Error("expected error when parsing with wrong secret")
	}
}

func TestSetSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "my-jwt-token", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != middleware.CookieName {
		t.Errorf("Name = %q, want %q", c.Name, middleware.CookieName)
	}
	if c.Value != "my-jwt-token" {
		t.Errorf("Value = %q, want %q", c.Value, "my-jwt-token")
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly = true")
	}
	if c.Secure {
		t.Error("expected Secure = false in dev mode")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.MaxAge <= 0 {
		t.Errorf("MaxAge = %d, want > 0", c.MaxAge)
	}
}

func TestSetSessionCookie_Secure(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "token", true)

	c := w.Result().Cookies()[0]
	if !c.Secure {
		t.Error("expected Secure = true in production mode")
	}
}

func TestClearSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearSessionCookie(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != middleware.CookieName {
		t.Errorf("Name = %q, want %q", c.Name, middleware.CookieName)
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want < 0 (delete cookie)", c.MaxAge)
	}
}
