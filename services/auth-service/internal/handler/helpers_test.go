package handler

import (
	"net/http"
	"testing"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice@example.com", "al***@example.com"},
		{"ab@example.com", "***@example.com"},     // len == 2, falls into <=2 branch
		{"a@example.com", "***@example.com"},    // len <= 2
		{"@example.com", "***"},                  // at == 0
		{"noemail", "***"},                       // no @
		{"", "***"},                              // empty
		{"alice+tag@example.com", "al***@example.com"},
	}
	for _, tt := range tests {
		got := maskEmail(tt.input)
		if got != tt.want {
			t.Errorf("maskEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name string
		xff  string
		want string
	}{
		{"single IP", "1.2.3.4", "1.2.3.4"},
		{"multiple IPs", "1.2.3.4, 5.6.7.8, 9.10.11.12", "1.2.3.4"},
		{"with spaces", "  1.2.3.4  , 5.6.7.8", "1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			r.Header.Set("X-Forwarded-For", tt.xff)
			got := clientIP(r)
			if got != tt.want {
				t.Errorf("clientIP with XFF=%q = %q, want %q", tt.xff, got, tt.want)
			}
		})
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	got := clientIP(r)
	if got != "192.168.1.1" {
		t.Errorf("clientIP(RemoteAddr=192.168.1.1:12345) = %q, want 192.168.1.1", got)
	}
}

func TestClientIP_RemoteAddrNoPort(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1"
	got := clientIP(r)
	if got != "192.168.1.1" {
		t.Errorf("clientIP(RemoteAddr=192.168.1.1) = %q, want 192.168.1.1", got)
	}
}

func TestSanitizeUsername_Extended(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello-world", "hello_world"},
		{"CamelCase", "camelcase"},
		{"special!@#$%", "special"},
		{"dots.and.more", "dotsandmore"},
		{"under_score", "under_score"},
		{"123numeric", "123numeric"},
		{"", ""},
		{"---", "___"},
	}
	for _, tt := range tests {
		got := sanitizeUsername(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeUsername(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomState(t *testing.T) {
	s1, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	s2, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if s1 == "" {
		t.Error("expected non-empty state")
	}
	if s1 == s2 {
		t.Error("expected different states on consecutive calls")
	}
}

func TestWriteJSON(t *testing.T) {
	// Verify writeJSON sets content-type and status
	rec := &responseRecorder{headers: make(http.Header)}
	writeJSON(rec, http.StatusTeapot, map[string]string{"hello": "world"})

	if rec.status != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", rec.status)
	}
	if rec.headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content-type, got %q", rec.headers.Get("Content-Type"))
	}
}

// minimal ResponseWriter for testing writeJSON
type responseRecorder struct {
	headers http.Header
	status  int
	body    []byte
}

func (r *responseRecorder) Header() http.Header         { return r.headers }
func (r *responseRecorder) WriteHeader(s int)            { r.status = s }
func (r *responseRecorder) Write(b []byte) (int, error)  { r.body = append(r.body, b...); return len(b), nil }
