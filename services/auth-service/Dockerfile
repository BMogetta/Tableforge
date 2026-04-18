# Build stage
FROM golang:1.26-alpine@sha256:80fbb8f9b2fa541a7d34378f1ad10f4f1c433817c4ed39ddb3e2f3ec3e961271 AS builder
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
FROM alpine:3.23@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601
RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /bin/auth-service /bin/auth-service

RUN adduser -D -u 1001 appuser
USER appuser

ENTRYPOINT ["/bin/auth-service"]