package jwt

import (
	"net/http/httptest"
	"testing"

	"github.com/recess/shared/middleware"
)

func TestSetRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetRefreshCookie(w, "session-uuid-here", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != middleware.RefreshCookieName {
		t.Errorf("Name = %q, want %q", c.Name, middleware.RefreshCookieName)
	}
	if c.Value != "session-uuid-here" {
		t.Errorf("Value = %q, want %q", c.Value, "session-uuid-here")
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly = true")
	}
	if c.Secure {
		t.Error("expected Secure = false in dev mode")
	}
	if c.Path != "/auth/refresh" {
		t.Errorf("Path = %q, want /auth/refresh", c.Path)
	}
	if c.MaxAge <= 0 {
		t.Errorf("MaxAge = %d, want > 0", c.MaxAge)
	}
}

func TestSetRefreshCookie_Secure(t *testing.T) {
	w := httptest.NewRecorder()
	SetRefreshCookie(w, "session-uuid", true)

	c := w.Result().Cookies()[0]
	if !c.Secure {
		t.Error("expected Secure = true")
	}
}

func TestClearRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearRefreshCookie(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != middleware.RefreshCookieName {
		t.Errorf("Name = %q, want %q", c.Name, middleware.RefreshCookieName)
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want < 0", c.MaxAge)
	}
}

func TestClearAllCookies(t *testing.T) {
	w := httptest.NewRecorder()
	ClearAllCookies(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies (session + refresh), got %d", len(cookies))
	}

	names := map[string]bool{}
	for _, c := range cookies {
		names[c.Name] = true
		if c.MaxAge >= 0 {
			t.Errorf("cookie %q: MaxAge = %d, want < 0", c.Name, c.MaxAge)
		}
	}
	if !names[middleware.CookieName] {
		t.Error("expected session cookie to be cleared")
	}
	if !names[middleware.RefreshCookieName] {
		t.Error("expected refresh cookie to be cleared")
	}
}
