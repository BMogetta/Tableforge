# OWASP Quick Fixes (B, F/I, K, N)

## Items

### B — gRPC reflection only in dev/test
**Files:** `services/game-server/cmd/server/main.go`, `services/rating-service/cmd/server/main.go`

Wrap `reflection.Register(grpcServer)` with env check:
```go
if config.Env("ENV", "development") != "production" {
    reflection.Register(grpcServer)
}
```

### F/I — Custom Recoverer (no stack traces to client)
**Files:** `shared/middleware/recoverer.go` (new), all 8 `services/*/cmd/server/main.go`

Create a custom recovery middleware in shared/middleware:
```go
func Recoverer(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rvr := recover(); rvr != nil {
                slog.Error("panic recovered",
                    "error", rvr,
                    "stack", string(debug.Stack()),
                    "method", r.Method,
                    "path", r.URL.Path,
                )
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte(`{"error":"internal error"}`))
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

Replace `middleware.Recoverer` with `sharedmw.Recoverer` in all services.

### K — Branch protection (not code — GitHub config)
Cannot be done by agent. Document in output that the user must:
1. Go to GitHub → Settings → Branches → Add rule for `main`
2. Enable "Require status checks to pass before merging"
3. Select: `Detect changes`, `Frontend`, `Validate Compose`
4. Enable "Require a pull request before merging"

### N — Image signing with cosign in CD
**File:** `.github/workflows/cd.yml`

Add cosign signing after each `docker/build-push-action`:
```yaml
- uses: sigstore/cosign-installer@v3
- name: Sign image
  run: cosign sign --yes ${{ env.REGISTRY }}/${{ github.repository_owner }}/tableforge-${{ matrix.service }}@${{ steps.build.outputs.digest }}
  env:
    COSIGN_EXPERIMENTAL: "true"
```

Same for the frontend build job. Need to add `id: build` to the build-push-action step
to capture the digest output.

## Also include while touching these files

### Auth failure logging (H)
**File:** `shared/middleware/auth.go`

In `Require()`, before returning 401:
```go
slog.Warn("auth: unauthorized request", "method", r.Method, "path", r.URL.Path)
```

In `RequireRole()`, before returning 403:
```go
slog.Warn("auth: insufficient role", "method", r.Method, "path", r.URL.Path, "required", role)
```

### Mask email in auth-service log (I-email)
**File:** `services/auth-service/internal/handler/handler.go`

Find `slog.Warn("auth: email not allowed", "email", email)` and mask:
```go
slog.Warn("auth: email not allowed", "email", maskEmail(email))
```

Add helper:
```go
func maskEmail(email string) string {
    parts := strings.SplitN(email, "@", 2)
    if len(parts) != 2 || len(parts[0]) < 2 { return "***" }
    return parts[0][:2] + "***@" + parts[1]
}
```

### ENV parameterize in compose (C)
**File:** `docker-compose.yml`

Change `ENV=development` to `ENV=${ENV:-development}`.

### game-server MustEnv for DATABASE_URL (D)
**File:** `services/game-server/cmd/server/main.go`

Change `config.Env("DATABASE_URL", "postgres://recess:recess@...")` to `config.MustEnv("DATABASE_URL")`.

### TanStack DevTools only in dev (E)
**File:** `frontend/src/main.tsx`

Wrap the `<TanStackDevtools>` block inside the existing `isDev` conditional.

### IdleTimeout on all HTTP servers (G)
**Files:** All 8 `services/*/cmd/server/main.go`

Add `IdleTimeout: 60 * time.Second` to each `&http.Server{}`.

### Avatar URL validation (J)
**File:** `services/auth-service/internal/handler/handler.go`

Before storing avatar URL, validate:
```go
if !strings.HasPrefix(avatarURL, "https://avatars.githubusercontent.com/") {
    avatarURL = "" // reject non-GitHub avatar URLs
}
```

## Testing
```bash
go test ./shared/middleware/...
go test ./services/game-server/...
go test ./services/auth-service/...
go test ./services/rating-service/...
cd frontend && npx tsc --noEmit
```
