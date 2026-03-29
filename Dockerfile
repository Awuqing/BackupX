# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS web-builder

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build


# ---- Stage 2: Build backend ----
FROM golang:1.25-alpine AS server-builder

WORKDIR /build/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN go build -trimpath -ldflags="-s -w" -o backupx ./cmd/backupx


# ---- Stage 3: Production image ----
FROM alpine:3.21

RUN apk add --no-cache \
    nginx \
    tzdata \
    ca-certificates \
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
