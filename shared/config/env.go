package config

import (
    "log/slog"
    "os"
)

// MustEnv returns the value of the environment variable or fatals.
func MustEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        slog.Error("required env var is not set", "key", key)
        os.Exit(1)
    }
    return v
}

// Env returns the value of the environment variable or a default.
func Env(key, defaultValue string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultValue
}