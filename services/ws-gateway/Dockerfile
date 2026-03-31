# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# This ensures "go mod download" only runs when dependencies change.
COPY shared/go.mod shared/go.sum ./shared/
COPY services/ws-gateway/go.mod services/ws-gateway/go.sum ./services/ws-gateway/

# 2. Download dependencies
# GOWORK=off is used to build the service in isolation using its own mod file
WORKDIR /app/services/ws-gateway
RUN GOWORK=off go mod download

# 3. Copy the actual source code
# We copy shared because ws-gateway depends on it via the go.mod replace directive
WORKDIR /app
COPY shared/ ./shared/
COPY services/ws-gateway/ ./services/ws-gateway/

# 4. Build the binary
# CGO_ENABLED=0 ensures a statically linked binary for the slim alpine image
WORKDIR /app/services/ws-gateway
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/ws-gateway ./cmd/server

# --- Run stage ---
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/ws-gateway /bin/ws-gateway

# Expose the WebSocket port (common for gateways, e.g., 8080 or 3000)
# EXPOSE 8080

ENTRYPOINT ["/bin/ws-gateway"]