# Build stage
FROM golang:1.26-alpine AS builder

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
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /bin/server /bin/server

CMD ["/bin/server"]