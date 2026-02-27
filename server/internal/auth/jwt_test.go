package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte("test-secret")

func TestSignAndVerifyToken(t *testing.T) {
	playerID := uuid.New()
	username := "alice"

	token, err := signToken(testSecret, playerID, username)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	c, err := verifyToken(testSecret, token)
	if err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if c.PlayerID != playerID {
		t.Errorf("expected player ID %s, got %s", playerID, c.PlayerID)
	}
	if c.Username != username {
		t.Errorf("expected username %s, got %s", username, c.Username)
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	playerID := uuid.New()
	token, _ := signToken(testSecret, playerID, "alice")

	_, err := verifyToken([]byte("wrong-secret"), token)
	if err == nil {
		t.Fatal("expected error with wrong secret, got nil")
	}
}

func TestVerifyToken_InvalidToken(t *testing.T) {
	_, err := verifyToken(testSecret, "not.a.token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	playerID := uuid.New()

	// Build an already-expired token manually.
	now := time.Now().Add(-2 * time.Hour)
	c := claims{
		PlayerID: playerID,
		Username: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)), // expired 119 min ago
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(testSecret)

	_, err := verifyToken(testSecret, token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}
