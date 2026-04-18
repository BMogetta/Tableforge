# Build stage
FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
WORKDIR /app

COPY shared/go.mod shared/go.sum ./shared/
COPY services/notification-service/go.mod services/notification-service/go.sum ./services/notification-service/

WORKDIR /app/services/notification-service
RUN GOWORK=off go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY services/notification-service/ ./services/notification-service/

WORKDIR /app/services/notification-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/notification-service ./cmd/server

# Run stage
FROM alpine:3.19@sha256:b58899f069c47216f6002a6850143dc6fae0d35eb8b0df9300bbe6327b9c2171
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /bin/notification-service /app/notification-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8086

ENTRYPOINT ["/app/notification-service"]
