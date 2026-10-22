FROM golang:1.25-alpine AS builder

ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct

RUN apk add --no-cache git

WORKDIR /build

# 1. 先只复制依赖文件，利用 Docker 层缓存
COPY go.mod go.sum ./

# 2. 使用 BuildKit 缓存挂载，下载的模块可跨构建复用
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 3. go mod tidy 应该在本机运行，不要放在 Dockerfile 里
#    如果必须在这里运行，用缓存挂载
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod verify

# 4. 复制源码（.dockerignore 会排除不必要文件）
COPY . .

# 5. 构建时使用 Go 构建缓存，大幅加速重复构建
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/bin/web ./cmd

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/bin/worker ./cmd/worker

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /build/bin/web /app/bin/web
COPY --from=builder /build/bin/worker /app/bin/worker
COPY --from=builder /build/config/config.docker.yaml /app/config/config.yaml
COPY --from=builder /build/config/worker.docker.yaml /app/config/worker.yaml
COPY --from=builder /build/static /app/static

EXPOSE 3333

# Default command (docker-compose overrides this per service)
CMD ["/app/bin/web"]

HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
    CMD curl -f http://localhost:3333/api/v1/ping || exit 1

# Run as non-root
RUN adduser -D -g '' appuser && chown -R appuser:appuser /app
USER appuser
