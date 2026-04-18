# Build stage
FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# These paths are relative to the build context (project root)
COPY shared/go.mod shared/go.sum ./shared/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/

# 2. Download dependencies
# GOWORK=off ensures Go doesn't look for the root go.work file
WORKDIR /app/services/auth-service
RUN GOWORK=off go mod download

# 3. Copy the actual source code
WORKDIR /app
COPY shared/ ./shared/
COPY services/auth-service/ ./services/auth-service/

# 4. Build the binary
WORKDIR /app/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/auth-service ./cmd/server

# --- Run stage ---
FROM alpine:3.19@sha256:b58899f069c47216f6002a6850143dc6fae0d35eb8b0df9300bbe6327b9c2171
RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /bin/auth-service /bin/auth-service

RUN adduser -D -u 1001 appuser
USER appuser

ENTRYPOINT ["/bin/auth-service"]