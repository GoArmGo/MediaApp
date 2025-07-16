# Dockerfile
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
RUN go build -o /mediaapp ./cmd/mediaapp

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /mediaapp .
COPY .env ./.env
COPY internal/database/postgres/migrations ./internal/database/postgres/migrations 
# Если миграции нужны для запуска в контейнере

EXPOSE 8080

CMD ["./mediaapp"]