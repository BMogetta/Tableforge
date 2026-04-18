FROM golang:1.26-alpine@sha256:80fbb8f9b2fa541a7d34378f1ad10f4f1c433817c4ed39ddb3e2f3ec3e961271 AS builder
WORKDIR /app

COPY shared/go.mod shared/go.sum ./shared/
COPY services/match-service/go.mod services/match-service/go.sum ./services/match-service/

WORKDIR /app/services/match-service
RUN GOWORK=off go mod download

WORKDIR /app
COPY shared/ ./shared/
COPY services/match-service/ ./services/match-service/

WORKDIR /app/services/match-service
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /bin/match-service ./cmd/server

FROM alpine:3.23@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /bin/match-service /app/match-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8087
ENTRYPOINT ["/app/match-service"]
