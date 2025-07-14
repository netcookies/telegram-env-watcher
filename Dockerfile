FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o telegram-env-watcher main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/telegram-env-watcher /usr/local/bin/

CMD ["telegram-env-watcher"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD pgrep telegram-env-watcher >/dev/null || exit 1
