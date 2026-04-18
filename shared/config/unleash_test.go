package config

import (
	"os"
	"testing"
)

func TestLoadUnleash_Defaults(t *testing.T) {
	os.Unsetenv("UNLEASH_URL")
	os.Unsetenv("UNLEASH_API_TOKEN")
	os.Unsetenv("UNLEASH_ENV")

	got := LoadUnleash("game-server")

	if got.URL != "http://unleash:4242/api" {
		t.Errorf("URL default: got %q", got.URL)
	}
	if got.Token != "*:*.unleash-insecure-api-token" {
		t.Errorf("Token default: got %q", got.Token)
	}
	if got.Environment != "development" {
		t.Errorf("Environment default: got %q", got.Environment)
	}
	if got.AppName != "game-server" {
		t.Errorf("AppName passthrough: got %q", got.AppName)
	}
}

func TestLoadUnleash_Overrides(t *testing.T) {
	t.Setenv("UNLEASH_URL", "http://other-host:5000/api")
	t.Setenv("UNLEASH_API_TOKEN", "scoped:client.abc")
	t.Setenv("UNLEASH_ENV", "production")

	got := LoadUnleash("chat-service")

	if got.URL != "http://other-host:5000/api" {
		t.Errorf("URL override: got %q", got.URL)
	}
	if got.Token != "scoped:client.abc" {
		t.Errorf("Token override: got %q", got.Token)
	}
	if got.Environment != "production" {
		t.Errorf("Environment override: got %q", got.Environment)
	}
	if got.AppName != "chat-service" {
		t.Errorf("AppName passthrough: got %q", got.AppName)
	}
}
