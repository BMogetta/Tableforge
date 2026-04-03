package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/recess/shared/schemas"
)

// SchemaRegistry holds pre-compiled JSON schemas keyed by name
// (e.g. "create_room.request", "apply_move.request").
type SchemaRegistry struct {
	compiled map[string]*jsonschema.Schema
}

// NewSchemaRegistry compiles all embedded JSON schemas from shared/schemas/.
// Defs are loaded first so $ref resolution works across files.
func NewSchemaRegistry() (*SchemaRegistry, error) {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	// Load all JSON files (defs + endpoint schemas) into the compiler.
	if err := fs.WalkDir(schemas.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := schemas.FS.ReadFile(path)
		if err != nil {
			return err
		}
		return compiler.AddResource(path, bytes.NewReader(data))
	}); err != nil {
		return nil, err
	}

	// Compile only endpoint schemas (root-level .json files, not defs/).
	reg := &SchemaRegistry{compiled: make(map[string]*jsonschema.Schema)}
	entries, err := schemas.FS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		schema, err := compiler.Compile(e.Name())
		if err != nil {
			return nil, err
		}
		// "create_room.request.json" → "create_room.request"
		name := strings.TrimSuffix(e.Name(), ".json")
		reg.compiled[name] = schema
	}
	return reg, nil
}

// ValidateBody returns chi middleware that validates the request body against
// the named schema. Returns 400 with structured errors on validation failure.
// The body is buffered and restored so the handler can decode it normally.
func (sr *SchemaRegistry) ValidateBody(name string) func(http.Handler) http.Handler {
	schema, ok := sr.compiled[name]
	if !ok {
		panic("validate: unknown schema " + name)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				writeValidationError(w, http.StatusBadRequest, "failed to read request body", nil)
				return
			}

			// Restore body for the handler.
			r.Body = io.NopCloser(bytes.NewReader(body))

			var v any
			if err := json.Unmarshal(body, &v); err != nil {
				writeValidationError(w, http.StatusBadRequest, "invalid JSON", nil)
				return
			}

			if err := schema.Validate(v); err != nil {
				ve, ok := err.(*jsonschema.ValidationError)
				if !ok {
					writeValidationError(w, http.StatusBadRequest, err.Error(), nil)
					return
				}
				details := flattenErrors(ve)
				writeValidationError(w, http.StatusBadRequest, "validation failed", details)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type validationErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

func writeValidationError(w http.ResponseWriter, status int, msg string, details []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(validationErrorResponse{
		Error:   msg,
		Details: details,
	})
}

// flattenErrors extracts human-readable messages from nested validation errors.
func flattenErrors(ve *jsonschema.ValidationError) []string {
	var msgs []string
	if ve.Message != "" {
		prefix := ve.InstanceLocation
		if prefix == "" {
			prefix = "/"
		}
		msgs = append(msgs, prefix+": "+ve.Message)
	}
	for _, cause := range ve.Causes {
		msgs = append(msgs, flattenErrors(cause)...)
	}
	return msgs
}
