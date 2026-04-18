# Build stage
FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# This ensures "go mod download" only runs when dependencies change.
COPY shared/go.mod shared/go.sum ./shared/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/

# 2. Download dependencies
# GOWORK=off is used here to build the service in isolation using its own mod file
WORKDIR /app/services/user-service
RUN GOWORK=off go mod download

# 3. Copy the actual source code
# We copy shared because user-service depends on it via the go.mod replace directive
WORKDIR /app
COPY shared/ ./shared/
COPY services/user-service/ ./services/user-service/

# 4. Build the binary
# CGO_ENABLED=0 ensures a statically linked binary for the slim alpine image
WORKDIR /app/services/user-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/user-service ./cmd/server

# --- Run stage ---
FROM alpine:3.19@sha256:b58899f069c47216f6002a6850143dc6fae0d35eb8b0df9300bbe6327b9c2171
RUN apk --no-cache add ca-certificates tzdata wget

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/user-service /bin/user-service

RUN adduser -D -u 1001 appuser
USER appuser

ENTRYPOINT ["/bin/user-service"]