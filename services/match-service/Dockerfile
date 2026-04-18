FROM golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder
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

FROM alpine:3.20@sha256:c64c687cbea9300178b30c95835354e34c4e4febc4badfe27102879de0483b5e
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /bin/match-service /app/match-service

RUN adduser -D -u 1001 appuser
USER appuser

EXPOSE 8087
ENTRYPOINT ["/app/match-service"]
