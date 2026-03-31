# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# These paths are relative to the build context (project root)
COPY shared/go.mod shared/go.sum ./shared/
COPY server/go.mod server/go.sum ./server/

# 2. Download dependencies
# GOWORK=off ensures Go doesn't look for the root go.work file
WORKDIR /app/server
RUN GOWORK=off go mod download

# 3. Copy the actual source code
WORKDIR /app
COPY shared/ ./shared/
COPY server/ ./server/

# 4. Build the binary
WORKDIR /app/server
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/seed-test ./cmd/seed-test

# --- Run stage ---
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/seed-test /bin/seed-test

CMD ["/bin/server"]