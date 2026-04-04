package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodySize_UnderLimit(t *testing.T) {
	body := strings.NewReader("hello")

	handler := MaxBodySize(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected error reading body: %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("expected body %q, got %q", "hello", string(data))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestMaxBodySize_OverLimit(t *testing.T) {
	// 20 bytes body with a 10 byte limit
	body := strings.NewReader("12345678901234567890")

	handler := MaxBodySize(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected MaxBytesError, got nil")
		}
		var maxErr *http.MaxBytesError
		if ok := isMaxBytesError(err, &maxErr); !ok {
			t.Fatalf("expected *http.MaxBytesError, got %T: %v", err, err)
		}
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", rec.Code)
	}
}

// isMaxBytesError unwraps to check for *http.MaxBytesError.
func isMaxBytesError(err error, target **http.MaxBytesError) bool {
	for err != nil {
		if mbe, ok := err.(*http.MaxBytesError); ok {
			*target = mbe
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
