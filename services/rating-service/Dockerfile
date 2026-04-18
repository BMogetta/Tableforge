# Build stage
FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
WORKDIR /app

# 1. Copy module definitions first to leverage Docker cache
# Includes shared and any other local module the rating-service depends on.
COPY shared/go.mod shared/go.sum ./shared/
COPY services/rating-service/go.mod services/rating-service/go.sum ./services/rating-service/

# 2. Download dependencies
# We bypass the root go.work to ensure the build is self-contained.
WORKDIR /app/services/rating-service
RUN GOWORK=off go mod download

# 3. Copy the actual source code
WORKDIR /app
COPY shared/ ./shared/
COPY services/rating-service/ ./services/rating-service/

# 4. Build the binary
# Targeted to ./cmd/server as requested.
WORKDIR /app/services/rating-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/rating-service ./cmd/server

# --- Run stage ---
FROM alpine:3.19@sha256:b58899f069c47216f6002a6850143dc6fae0d35eb8b0df9300bbe6327b9c2171
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/rating-service /app/rating-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8085 9085

ENTRYPOINT ["/app/rating-service"]