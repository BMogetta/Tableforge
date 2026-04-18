# Build stage
FROM golang:1.26-alpine@sha256:80fbb8f9b2fa541a7d34378f1ad10f4f1c433817c4ed39ddb3e2f3ec3e961271 AS builder

WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# These paths are relative to the build context (project root)
COPY shared/go.mod shared/go.sum ./shared/
COPY services/game-server/go.mod services/game-server/go.sum ./services/game-server/

# 2. Download dependencies
# GOWORK=off ensures Go doesn't look for the root go.work file
WORKDIR /app/services/game-server
RUN GOWORK=off go mod download

# 3. Copy the actual source code
WORKDIR /app
COPY shared/ ./shared/
COPY services/game-server/ ./services/game-server/

# 4. Build the binary
WORKDIR /app/services/game-server
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/server ./cmd/server

# --- Run stage ---
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /bin/server /bin/server

RUN adduser -D -u 1001 appuser
USER appuser

CMD ["/bin/server"]