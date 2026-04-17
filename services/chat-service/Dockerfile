# Build stage
FROM golang:1.26-alpine@sha256:80fbb8f9b2fa541a7d34378f1ad10f4f1c433817c4ed39ddb3e2f3ec3e961271 AS builder
WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# This ensures "go mod download" only runs when dependencies change.
COPY shared/go.mod shared/go.sum ./shared/
COPY services/chat-service/go.mod services/chat-service/go.sum ./services/chat-service/

# 2. Download dependencies
# GOWORK=off allows the build to run using the specific service's go.mod
WORKDIR /app/services/chat-service
RUN GOWORK=off go mod download

# 3. Copy the actual source code
# We copy shared because chat-service depends on it via the go.mod replace directive
WORKDIR /app
COPY shared/ ./shared/
COPY services/chat-service/ ./services/chat-service/

# 4. Build the binary
# CGO_ENABLED=0 ensures a statically linked binary for the slim alpine image
WORKDIR /app/services/chat-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/chat-service ./cmd/server

# --- Run stage ---
FROM alpine:3.19@sha256:b58899f069c47216f6002a6850143dc6fae0d35eb8b0df9300bbe6327b9c2171
RUN apk --no-cache add ca-certificates tzdata wget

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/chat-service /bin/chat-service

RUN adduser -D -u 1001 appuser
USER appuser

ENTRYPOINT ["/bin/chat-service"]