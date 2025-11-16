# syntax=docker/dockerfile:1.6

### ===== 1. 构建阶段（有 Go 环境） =====
FROM golang:1.24 AS builder

WORKDIR /src

# 使用国内 go module 镜像
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /bin/go-web ./main.go


### ===== 2. 运行时阶段（小镜像） =====
FROM debian:bookworm-slim AS runtime

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata && \
    rm -rf /var/lib/apt/lists/*

# 创建非 root 用户
RUN useradd -r -u 10001 app

# 修复 Go modcache 权限问题
RUN mkdir -p /home/app && chown -R app:app /home/app

WORKDIR /app
# 让 app 用户拥有 /app 的写权限
RUN chown -R app:app /app

# 复制编译后的程序和所有资源
COPY --from=builder --chown=app:app /bin/go-web /usr/local/bin/go-web
COPY --from=builder --chown=app:app /src/config /app/config
COPY --from=builder --chown=app:app /src/static /app/static
COPY --from=builder --chown=app:app /src/templates /app/templates
COPY --from=builder --chown=app:app /src/assets /app/assets

RUN mkdir -p /app/files && chown -R app:app /app/files

# 环境变量
ENV GIN_MODE=release \
    TZ=Asia/Shanghai

EXPOSE 8080
USER app
CMD ["go-web"]
