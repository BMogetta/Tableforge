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

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /bin/match-service /app/match-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8087
ENTRYPOINT ["/app/match-service"]
