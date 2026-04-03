package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SchemaRegistry
// ---------------------------------------------------------------------------

func TestNewSchemaRegistry_CompilesAllSchemas(t *testing.T) {
	reg, err := NewSchemaRegistry()
	if err != nil {
		t.Fatalf("NewSchemaRegistry() error: %v", err)
	}

	// Endpoint schemas that must be present (root-level .json files).
	want := []string{
		"create_room.request",
		"create_room.response",
		"join_room.request",
		"join_room.response",
		"apply_move.request",
		"apply_move.response",
		"get_session.response",
		"vote_rematch.request",
		"vote_rematch.response",
		"leave_room.request",
		"start_game.request",
		"update_room_setting.request",
		"add_bot.request",
		"remove_bot.request",
		"surrender.request",
		"vote_pause.request",
		"vote_resume.request",
	}
	for _, name := range want {
		if _, ok := reg.compiled[name]; !ok {
			t.Errorf("schema %q not found in registry", name)
		}
	}
}

func TestNewSchemaRegistry_DoesNotIncludeDefs(t *testing.T) {
	reg, err := NewSchemaRegistry()
	if err != nil {
		t.Fatalf("NewSchemaRegistry() error: %v", err)
	}

	// Defs should NOT appear as top-level compiled schemas.
	for name := range reg.compiled {
		if strings.HasPrefix(name, "defs/") {
			t.Errorf("def %q should not be a top-level compiled schema", name)
		}
	}
}

func TestNewSchemaRegistry_RefResolution(t *testing.T) {
	// create_room.response references defs/room.json and defs/room_player.json.
	// If $ref resolution fails, Compile() returns an error.
	reg, err := NewSchemaRegistry()
	if err != nil {
		t.Fatalf("NewSchemaRegistry() error: %v", err)
	}

	if _, ok := reg.compiled["create_room.response"]; !ok {
		t.Fatal("create_room.response not compiled — $ref resolution may have failed")
	}
}

// ---------------------------------------------------------------------------
// ValidateBody middleware
// ---------------------------------------------------------------------------

// echoHandler reads the body and echoes it back — proves the body was preserved.
var echoHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
})

func mustRegistry(t *testing.T) *SchemaRegistry {
	t.Helper()
	reg, err := NewSchemaRegistry()
	if err != nil {
		t.Fatalf("NewSchemaRegistry() error: %v", err)
	}
	return reg
}

func TestValidateBody_ValidRequest(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	body := `{"game_id":"tictactoe","player_id":"abc-123"}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Body should be preserved for the handler.
	if rec.Body.String() != body {
		t.Errorf("body not preserved: got %q, want %q", rec.Body.String(), body)
	}
}

func TestValidateBody_ValidRequestWithOptionalField(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	body := `{"game_id":"tictactoe","player_id":"abc-123","turn_timeout_secs":30}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_MissingRequiredField(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	// Missing game_id — required.
	body := `{"player_id":"abc-123"}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp validationErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "validation failed" {
		t.Errorf("unexpected error message: %q", resp.Error)
	}
	if len(resp.Details) == 0 {
		t.Error("expected validation details, got none")
	}
}

func TestValidateBody_WrongFieldType(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	// turn_timeout_secs should be integer, not string.
	body := `{"game_id":"tictactoe","player_id":"abc","turn_timeout_secs":"not_a_number"}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_MinLengthViolation(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	// game_id has minLength: 1 — empty string should fail.
	body := `{"game_id":"","player_id":"abc"}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_MinimumViolation(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	// turn_timeout_secs has minimum: 5 — 2 should fail.
	body := `{"game_id":"tictactoe","player_id":"abc","turn_timeout_secs":2}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_MalformedJSON(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp validationErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "invalid JSON" {
		t.Errorf("expected 'invalid JSON', got %q", resp.Error)
	}
}

func TestValidateBody_EmptyBody(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("create_room.request")
	handler := mw(echoHandler)

	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(""))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestValidateBody_SchemaWithRef(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("apply_move.request")
	handler := mw(echoHandler)

	body := `{"player_id":"abc","payload":{"row":0,"col":1}}`
	req := httptest.NewRequest(http.MethodPost, "/sessions/123/move", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_PayloadMustBeObject(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("apply_move.request")
	handler := mw(echoHandler)

	// payload should be object, not string.
	body := `{"player_id":"abc","payload":"not an object"}`
	req := httptest.NewRequest(http.MethodPost, "/sessions/123/move", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateBody_UnknownSchemaPanics(t *testing.T) {
	reg := mustRegistry(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unknown schema")
		}
	}()

	reg.ValidateBody("nonexistent.request")
}

func TestValidateBody_BodyPreservedAfterValidation(t *testing.T) {
	reg := mustRegistry(t)
	mw := reg.ValidateBody("join_room.request")

	original := `{"code":"ABCD","player_id":"xyz"}`

	// Handler that decodes the body into a struct — simulates real handler behavior.
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code     string `json:"code"`
			PlayerID string `json:"player_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("handler failed to decode body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if req.Code != "ABCD" || req.PlayerID != "xyz" {
			t.Errorf("unexpected decoded values: code=%q player_id=%q", req.Code, req.PlayerID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/rooms/join", strings.NewReader(original))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
