# BackupX 多阶段构建
#
# 用法：
#   国际构建（默认）：docker build -t backupx .
#   国内加速构建：    docker build --build-arg USE_CHINA_MIRROR=true -t backupx .
#   注入版本号：      docker build --build-arg VERSION=v1.2.3 -t backupx .

# 全局构建参数
ARG USE_CHINA_MIRROR=false


# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS web-builder
ARG USE_CHINA_MIRROR

# 国内镜像：npm 使用淘宝源
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
      npm config set registry https://registry.npmmirror.com; \
    fi

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build


# ---- Stage 2: Build backend ----
FROM golang:1.25-alpine AS server-builder
ARG USE_CHINA_MIRROR
ARG VERSION=dev

# 国内镜像：Go 模块使用七牛代理
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
      go env -w GOPROXY=https://goproxy.cn,direct; \
    fi

WORKDIR /build/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o backupx ./cmd/backupx


# ---- Stage 3: Production image ----
FROM alpine:3.21
ARG USE_CHINA_MIRROR

# 国内镜像：Alpine apk 使用阿里云源
RUN if [ "$USE_CHINA_MIRROR" = "true" ]; then \
      sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories; \
    fi

RUN apk add --no-cache \
    nginx \
    tzdata \
    ca-certificates \
    docker-cli docker-cli-compose \
    # Required by mysql/postgresql backup tasks
    mysql-client \
    postgresql16-client \
    && rm -rf /var/cache/apk/*

# Create app user
RUN addgroup -S backupx && adduser -S -G backupx -h /app backupx

# Copy backend binary
COPY --from=server-builder /build/server/backupx /app/bin/backupx

# Copy frontend static files
COPY --from=web-builder /build/web/dist /app/web

# Copy nginx config
COPY deploy/docker/nginx.conf /etc/nginx/http.d/default.conf

# Copy entrypoint
COPY deploy/docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create data directories
RUN mkdir -p /app/data /tmp/backupx && \
    chown -R backupx:backupx /app /tmp/backupx

# Nginx needs to write to these dirs
RUN mkdir -p /var/lib/nginx/tmp /var/log/nginx && \
    chown -R backupx:backupx /var/lib/nginx /var/log/nginx /run/nginx

WORKDIR /app
EXPOSE 8340

VOLUME ["/app/data"]

ENTRYPOINT ["/app/entrypoint.sh"]
