package config

import (
    "log"
    "os"
)

// MustEnv returns the value of the environment variable or fatals.
func MustEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("required env var %s is not set", key)
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