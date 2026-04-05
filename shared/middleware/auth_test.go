package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte("test-secret-key-for-unit-tests")

func signToken(t *testing.T, claims Claims, secret []byte) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	return raw
}

func validClaims(playerID uuid.UUID) Claims {
	return Claims{
		PlayerID: playerID,
		Username: "testuser",
		Role:     RolePlayer,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(JWTTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

// --- VerifyToken tests -------------------------------------------------------

func TestVerifyToken_Valid(t *testing.T) {
	pid := uuid.New()
	raw := signToken(t, validClaims(pid), testSecret)

	c, err := VerifyToken(testSecret, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.PlayerID != pid {
		t.Errorf("expected player ID %s, got %s", pid, c.PlayerID)
	}
	if c.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", c.Username)
	}
	if c.Role != RolePlayer {
		t.Errorf("expected role player, got %s", c.Role)
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	claims := validClaims(uuid.New())
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	raw := signToken(t, claims, testSecret)

	_, err := VerifyToken(testSecret, raw)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	raw := signToken(t, validClaims(uuid.New()), testSecret)

	_, err := VerifyToken([]byte("wrong-secret"), raw)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
	if errors.Is(err, ErrTokenExpired) {
		t.Error("should not be ErrTokenExpired for wrong secret")
	}
}

func TestVerifyToken_MalformedToken(t *testing.T) {
	_, err := VerifyToken(testSecret, "not.a.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestVerifyToken_EmptyString(t *testing.T) {
	_, err := VerifyToken(testSecret, "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestVerifyToken_WrongSigningMethod(t *testing.T) {
	// Sign with RSA method — VerifyToken only accepts HMAC.
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(testRSAKey))
	if err != nil {
		t.Fatalf("parse RSA key: %v", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims(uuid.New()))
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign RSA token: %v", err)
	}

	_, err = VerifyToken(testSecret, raw)
	if err == nil {
		t.Fatal("expected error for non-HMAC signing method")
	}
}

// --- Require middleware tests ------------------------------------------------

func TestRequire_ValidCookie(t *testing.T) {
	pid := uuid.New()
	raw := signToken(t, validClaims(pid), testSecret)

	var gotPID uuid.UUID
	var gotUsername, gotRole string
	handler := Require(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPID, _ = PlayerIDFromContext(r.Context())
		gotUsername, _ = UsernameFromContext(r.Context())
		gotRole, _ = RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotPID != pid {
		t.Errorf("expected player ID %s, got %s", pid, gotPID)
	}
	if gotUsername != "testuser" {
		t.Errorf("expected username testuser, got %s", gotUsername)
	}
	if gotRole != RolePlayer {
		t.Errorf("expected role player, got %s", gotRole)
	}
}

func TestRequire_MissingCookie(t *testing.T) {
	handler := Require(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequire_ExpiredToken_ReturnsJSON(t *testing.T) {
	claims := validClaims(uuid.New())
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	raw := signToken(t, claims, testSecret)

	handler := Require(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with expired token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	// Expired token returns JSON body with "token_expired" for frontend refresh flow.
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "token_expired" {
		t.Errorf("expected error=token_expired, got %s", body["error"])
	}
}

func TestRequire_InvalidToken(t *testing.T) {
	handler := Require(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// --- RequireRole middleware tests --------------------------------------------

func TestRequireRole_Sufficient(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		minimum string
	}{
		{"player >= player", RolePlayer, RolePlayer},
		{"manager >= player", RoleManager, RolePlayer},
		{"manager >= manager", RoleManager, RoleManager},
		{"owner >= player", RoleOwner, RolePlayer},
		{"owner >= manager", RoleOwner, RoleManager},
		{"owner >= owner", RoleOwner, RoleOwner},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := RequireRole(tc.minimum)(inner)

			ctx := ContextWithPlayer(context.Background(), uuid.New(), "u", tc.role)
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		})
	}
}

func TestRequireRole_Insufficient(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		minimum string
	}{
		{"player < manager", RolePlayer, RoleManager},
		{"player < owner", RolePlayer, RoleOwner},
		{"manager < owner", RoleManager, RoleOwner},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called with insufficient role")
			})
			handler := RequireRole(tc.minimum)(inner)

			ctx := ContextWithPlayer(context.Background(), uuid.New(), "u", tc.role)
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d", rec.Code)
			}
		})
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without role in context")
	})
	handler := RequireRole(RolePlayer)(inner)

	// Empty context — no role set.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// --- Context helper tests ----------------------------------------------------

func TestContextHelpers_RoundTrip(t *testing.T) {
	pid := uuid.New()
	ctx := ContextWithPlayer(context.Background(), pid, "alice", RoleOwner)

	gotPID, ok := PlayerIDFromContext(ctx)
	if !ok || gotPID != pid {
		t.Errorf("PlayerIDFromContext: expected %s, got %s (ok=%v)", pid, gotPID, ok)
	}
	gotUser, ok := UsernameFromContext(ctx)
	if !ok || gotUser != "alice" {
		t.Errorf("UsernameFromContext: expected alice, got %s (ok=%v)", gotUser, ok)
	}
	gotRole, ok := RoleFromContext(ctx)
	if !ok || gotRole != RoleOwner {
		t.Errorf("RoleFromContext: expected owner, got %s (ok=%v)", gotRole, ok)
	}
}

func TestContextHelpers_EmptyContext(t *testing.T) {
	ctx := context.Background()
	if _, ok := PlayerIDFromContext(ctx); ok {
		t.Error("expected ok=false for empty context")
	}
	if _, ok := UsernameFromContext(ctx); ok {
		t.Error("expected ok=false for empty context")
	}
	if _, ok := RoleFromContext(ctx); ok {
		t.Error("expected ok=false for empty context")
	}
}

// --- Test RSA key for wrong-signing-method test ------------------------------

const testRSAKey = `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQDCMBnRuNPEZ4hR
5lrEnDMCKlccNvMaZu4tyUzVpEgqik8kt+I0pVRnlseMZKGFDdDlyY6ZKhf0mBAm
fwFZ7ljEefTAHf5q3Wi2HXAFeFepwwpQwmGGqxPrBxCZjgkAzkAjNJcNvexj3VVs
JnDWGMx2cpI6K6oysv++A8RP68Zz3RSNxvY4mLlXAmbFsoZEAyqgOHSue/Wncxoi
bNOAL1jtuEdxLQW9kkdyTNIGxLlDTlih+jcKI0XMPAXLvlqjR1Np9q2jd2WYSWcc
NZQETVfJzHGdcUzBjfEg3eKp46yhopmURnZ8Q0KYleS2dRx9z4x1quo8J46+cky/
k9k2GGQrAgMBAAECggEAYEWOl13lhyq497mXaH/z/e/VzgQkFAPRs1toP0a6DHUT
daXAfM82gEDNX3oIZbmKxlFd4+ttgvLclulCVz6GPaokwBZeqsZoAlmnzt5xv5UV
iSJFTYmFT3DqNuam6gJW491Pwh1Vk9EIZ5zLNimHPHXirvo03/vENIUTj3fZpFip
nlGZjbbADx9ef8RGxdkpEvVurLVStqTlbJD1pt5IW753gtlfVpHSjqjizGcpa+jV
JvK2MfBmB04Mp5UCB3iofBlMXbVyQVTqJC1OPtytrQu9Sv9FBOZ/kMsH13rKSf2e
Yj1xxz0Z1cUkL5ZPCv98WleZwQk1h6GeXX/tjioxkQKBgQD1bAzl6Fz1EmGeAxyg
rXdhkpjoSz/NTHVhr9tYnWXxmhCXFYwOGhmpEmFR03m7+kln5nX4WDxxnmmg+26A
6K7MuEOHtX+x67Lhrg2f1DOZQ8GTblZ/BSkDlHePReQEktE7fiZDZJ+NzGceIRAl
4/soYtVnU7nIa+22TNgDZUN2owKBgQDKjr2KFyOablg0c1znK7i+gI0/hJpfZiyz
+/wXKVIs2xET6nJEM0V9ecKIoPRiN0wUj309P3gAaCBpu7Hotg9V5JOu6GH9w115
ByIcs3AeXgFlAWTBvhmVkWwAZ2O7FnAKt/7W3K5bs6GxXAHItegYApJpgjCYzFHg
tzdzN0Uc2QKBgQDkYxTtrvsypVRqc4LklAkQqBfbtIs/RfPGYJzDLlZ8K19c+hRH
20od6JjgSOh0YkqFghYucg5tvXmW8eS32dExehh95g1bSXhCRHMxVYxfCIrP5FJi
Ci9MwZExp1y2VNqZfp+k/7Lrhlg/1Yztded4geEOMwAk3ytsBr7PCiFp+QKBgQCp
aAYXZtDNCLpWa6FoaYWiNetsEx/054Q9p2KnkFR81V6MFIkqhuL4VQwgrtSVDABi
NbudrOZVGMD7DRJ3OUTMJlZpc0r5LBqR7ShXbq83hpGOA0NcUfwdvjjggZfEUbi1
DjthQcHFSg/SQMvxLEoHQqdm/I5eTIux8Cm8/52ayQKBgQC/zXSIIlKwUrzj1tJx
qk3tbKPnXHree2GjrXnGCBGEqkkDsloV6+7zyzRV/4BwuaYat/9KRNv9heahctZv
p5grjfgTXk3nVPiCsXDsDtSuFsHPVzSneqY6Ln38ghmTfMAL9hm69rZcAAfEJDhE
zYfk/TUEf0VYT30glP6Z+w8mLg==
-----END PRIVATE KEY-----`
