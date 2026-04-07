package config

import (
	"os"
	"testing"
)

func TestEnv_ReturnsValue(t *testing.T) {
	os.Setenv("TEST_ENV_VAR", "hello")
	defer os.Unsetenv("TEST_ENV_VAR")

	got := Env("TEST_ENV_VAR", "default")
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestEnv_ReturnsDefault(t *testing.T) {
	os.Unsetenv("TEST_ENV_MISSING")

	got := Env("TEST_ENV_MISSING", "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
}

func TestEnv_EmptyValueReturnsDefault(t *testing.T) {
	os.Setenv("TEST_ENV_EMPTY", "")
	defer os.Unsetenv("TEST_ENV_EMPTY")

	got := Env("TEST_ENV_EMPTY", "default")
	if got != "default" {
		t.Errorf("expected default for empty value, got %q", got)
	}
}
