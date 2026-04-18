# Build stage
FROM golang:1.26-alpine@sha256:80fbb8f9b2fa541a7d34378f1ad10f4f1c433817c4ed39ddb3e2f3ec3e961271 AS builder
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
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /bin/rating-service /app/rating-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8085 9085

ENTRYPOINT ["/app/rating-service"]