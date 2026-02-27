# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# Run stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/server /bin/server

EXPOSE 8080

CMD ["/bin/server"]