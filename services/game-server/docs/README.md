# Type Generation Pipeline

This project uses [swaggo](https://github.com/swaggo/swag) + [swagger-typescript-api](https://github.com/acacode/swagger-typescript-api) to automatically generate TypeScript types from Go structs.

## How it works

```txt

Go structs + swag annotations
        ↓
swag init → swagger.json
        ↓
jq patch (swagger.required-patch.json) → swagger.patched.json
        ↓
swagger-typescript-api → frontend/src/lib/api-generated.ts

```

## Usage

```bash
make gen-types
```

Output: `frontend/src/lib/api-generated.ts`

---

## Adding a new endpoint

### 1. Add swag annotations to the handler

```go
// @Summary  Create something
// @Tags     things
// @Produce  json
// @Param    id path string true "Thing UUID"
// @Success  200 {object} MyNewResponse
// @Failure  404 {object} map[string]string
// @Router   /things/{id} [get]
func handleGetThing(rt *runtime.Service) http.HandlerFunc { ... }
```

### 2. Define the response type

Add the struct to the appropriate types file (e.g. `server/internal/platform/api/api_session_types.go`).

```go
type MyNewResponse struct {
    ID    string     `json:"id"`
    Name  string     `json:"name"`
    Meta  *SomeMeta  `json:"meta"` // pointer → nullable → optional in TypeScript
}
```

### 3. Update the required-fields patch

Open `server/docs/swagger.required-patch.json` and add an entry for the new type.
Only include fields that are **non-pointer** in Go — those are always present in the response.

```json
{
  "MyNewResponse": ["id", "name"]
}
```

> **Rule:** pointer in Go (`*T`) → omit from patch → `field?: T` in TypeScript.
> Non-pointer in Go (`T`) → include in patch → `field: T` in TypeScript.

### 4. Regenerate

```bash
make gen-types
```

---

## Why the patch file?

swaggo does not emit a `required` array in the JSON Schema for non-pointer struct fields.
Without it, `swagger-typescript-api` marks every property as optional (`?`).

The patch file (`server/docs/swagger.required-patch.json`) is the source of truth for
which fields are required. It is applied via `server/docs/patch-swagger.jq` as a
post-processing step before the TypeScript generator runs.

Keep this file in sync with the Go structs — if you add or remove a non-pointer field,
update the patch accordingly.

---

## Files

| File | Description |
| --- | --- |
| `server/docs/swagger.json` | Raw output from swaggo. Do not edit manually. |
| `server/docs/swagger.required-patch.json` | Required fields per type. Edit this when adding new types. |
| `server/docs/patch-swagger.jq` | jq script that merges the patch into the swagger spec. |
| `server/docs/swagger.patched.json` | Intermediate patched spec. Do not edit manually. |
| `frontend/src/lib/api-generated.ts` | Generated TypeScript types. Do not edit manually. |
