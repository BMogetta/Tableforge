# Request Body Size Limits

## Why
No service uses `http.MaxBytesReader`. A malicious client can send a multi-GB
request body and cause OOM on the server. Found in OWASP A10 audit.

## What to change

### Option A: chi middleware (recommended)
Add a global middleware to every service's router that limits request body size:

```go
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}
```

Apply in each service's router setup: `r.Use(MaxBodySize(1 << 20))` (1MB default).

### Option B: shared middleware
Add `MaxBodySize` to `shared/middleware/` so all services use the same implementation.

### Per-service limits
- Game-server: 1MB (game moves are small JSON)
- Chat-service: 64KB (messages are short)
- Auth-service: 16KB (OAuth payloads are tiny)
- User-service: 64KB (profile updates, settings)
- Match-service: 16KB
- Notification-service: 16KB
- Rating-service: read-only, no POST body needed
- WS-gateway: WebSocket frame size (already handled by gorilla/websocket `ReadLimit`)

## Files
- `shared/middleware/body_limit.go` (new)
- All 8 `services/*/cmd/server/main.go` — add `r.Use(sharedmw.MaxBodySize(...))`

## Testing
- `go test ./shared/middleware/...`
- curl test: `curl -X POST -d @/dev/urandom http://localhost/api/v1/rooms | head`
  should return 413 Request Entity Too Large
