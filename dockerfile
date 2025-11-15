# syntax=docker/dockerfile:1.6
FROM golang:1.24 AS builder
FROM debian:bookworm-slim AS runtime
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /bin/go-web ./main.go
RUN useradd -r -u 10001 app
WORKDIR /app
COPY --from=builder /bin/go-web /usr/local/bin/go-web
COPY --from=builder /src/config /app/config
COPY --from=builder /src/static /app/static
COPY --from=builder /src/templates /app/templates
COPY --from=builder /src/assets /app/assets
# 可选：如果需要内置初始上传目录
COPY --from=builder /src/files /app/files
ENV GIN_MODE=release \
    TZ=Asia/Shanghai
EXPOSE 3000
USER app
CMD ["go-web"]