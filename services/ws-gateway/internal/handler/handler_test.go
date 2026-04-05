package handler

import (
	"net/http"
	"testing"
)

func TestParseAllowedOrigins_Empty(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	m := parseAllowedOrigins()
	if m != nil {
		t.Errorf("expected nil for empty env var, got %v", m)
	}
}

func TestParseAllowedOrigins_Single(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://example.com")
	m := parseAllowedOrigins()
	if len(m) != 1 {
		t.Fatalf("expected 1 origin, got %d", len(m))
	}
	if _, ok := m["https://example.com"]; !ok {
		t.Error("expected https://example.com in map")
	}
}

func TestParseAllowedOrigins_Multiple(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://a.com, https://b.com , https://c.com")
	m := parseAllowedOrigins()
	if len(m) != 3 {
		t.Fatalf("expected 3 origins, got %d", len(m))
	}
	for _, o := range []string{"https://a.com", "https://b.com", "https://c.com"} {
		if _, ok := m[o]; !ok {
			t.Errorf("expected %s in map", o)
		}
	}
}

func TestParseAllowedOrigins_IgnoresEmptyEntries(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://a.com,,, ,https://b.com")
	m := parseAllowedOrigins()
	if len(m) != 2 {
		t.Fatalf("expected 2 origins (ignoring blanks), got %d", len(m))
	}
}

func TestCheckOrigin_DevMode(t *testing.T) {
	// Save and restore module-level var.
	old := allowedOrigins
	allowedOrigins = nil // dev mode
	defer func() { allowedOrigins = old }()

	r, _ := http.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "http://anything.example.com")
	if !checkOrigin(r) {
		t.Error("expected true in dev mode (nil allowedOrigins)")
	}
}

func TestCheckOrigin_EmptyMap(t *testing.T) {
	old := allowedOrigins
	allowedOrigins = map[string]struct{}{}
	defer func() { allowedOrigins = old }()

	r, _ := http.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "http://anything.example.com")
	// Empty map has len 0, so dev mode applies.
	if !checkOrigin(r) {
		t.Error("expected true when allowedOrigins is empty map")
	}
}

func TestCheckOrigin_Allowed(t *testing.T) {
	old := allowedOrigins
	allowedOrigins = map[string]struct{}{"https://ok.com": {}}
	defer func() { allowedOrigins = old }()

	r, _ := http.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://ok.com")
	if !checkOrigin(r) {
		t.Error("expected true for allowed origin")
	}
}

func TestCheckOrigin_Rejected(t *testing.T) {
	old := allowedOrigins
	allowedOrigins = map[string]struct{}{"https://ok.com": {}}
	defer func() { allowedOrigins = old }()

	r, _ := http.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://evil.com")
	if checkOrigin(r) {
		t.Error("expected false for non-allowed origin")
	}
}

func TestCheckOrigin_NoOriginHeader(t *testing.T) {
	old := allowedOrigins
	allowedOrigins = map[string]struct{}{"https://ok.com": {}}
	defer func() { allowedOrigins = old }()

	r, _ := http.NewRequest("GET", "/ws", nil)
	// No Origin header -> empty string not in map
	if checkOrigin(r) {
		t.Error("expected false when origin header is missing and allowedOrigins is set")
	}
}
