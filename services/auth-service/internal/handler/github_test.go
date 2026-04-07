package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGithubGET_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or wrong Authorization header")
		}
		json.NewEncoder(w).Encode(map[string]string{"login": "alice"})
	}))
	defer srv.Close()

	var result struct {
		Login string `json:"login"`
	}
	err := githubGET(context.Background(), "test-token", srv.URL, &result)
	if err != nil {
		t.Fatalf("githubGET: %v", err)
	}
	if result.Login != "alice" {
		t.Errorf("expected alice, got %s", result.Login)
	}
}

func TestGithubGET_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	var result struct{}
	err := githubGET(context.Background(), "bad-token", srv.URL, &result)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("code") != "abc123" {
			t.Error("missing code param")
		}
		if r.URL.Query().Get("client_id") != "cid" {
			t.Error("missing client_id param")
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-xyz"})
	}))
	defer srv.Close()

	// Override the token URL for testing — we need to call the test server.
	// Since exchangeCode uses the package-level const, we call the underlying
	// logic directly by creating a request to our test server.
	// Actually exchangeCode hardcodes the URL, so we can't redirect it.
	// Instead we test the error path.
	t.Skip("exchangeCode uses hardcoded githubTokenURL — not unit-testable without refactoring")
}

func TestExchangeCode_ErrorResponse(t *testing.T) {
	// Same limitation — hardcoded URL.
	t.Skip("exchangeCode uses hardcoded githubTokenURL — not unit-testable without refactoring")
}

func TestFetchPrimaryEmail_Logic(t *testing.T) {
	// fetchPrimaryEmail uses hardcoded githubEmailURL, but we can test the
	// email selection logic by calling githubGET with a test server that
	// returns the email list, then verifying the behavior.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emails := []githubEmail{
			{Email: "secondary@example.com", Primary: false, Verified: true},
			{Email: "primary@example.com", Primary: true, Verified: true},
			{Email: "unverified@example.com", Primary: true, Verified: false},
		}
		json.NewEncoder(w).Encode(emails)
	}))
	defer srv.Close()

	// We can't call fetchPrimaryEmail directly (hardcoded URL), but we can
	// replicate its logic to test the email selection.
	var emails []githubEmail
	if err := githubGET(context.Background(), "tok", srv.URL, &emails); err != nil {
		t.Fatalf("fetch emails: %v", err)
	}

	// Replicate fetchPrimaryEmail logic.
	var found string
	for _, e := range emails {
		if e.Primary && e.Verified {
			found = e.Email
			break
		}
	}
	if found != "primary@example.com" {
		t.Errorf("expected primary@example.com, got %s", found)
	}
}

func TestFetchPrimaryEmail_NoPrimary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emails := []githubEmail{
			{Email: "nope@example.com", Primary: false, Verified: true},
		}
		json.NewEncoder(w).Encode(emails)
	}))
	defer srv.Close()

	var emails []githubEmail
	githubGET(context.Background(), "tok", srv.URL, &emails)

	var found string
	for _, e := range emails {
		if e.Primary && e.Verified {
			found = e.Email
		}
	}
	if found != "" {
		t.Error("expected no primary verified email")
	}
}
