# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -o /bin/seeder ./cmd/seed \
 && CGO_ENABLED=0 GOOS=linux go build -o /bin/seed-test ./cmd/seed-test

# Run stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/seeder /bin/seeder
COPY --from=builder /bin/seed-test /bin/seed-test

EXPOSE 8080

CMD ["/bin/server"]