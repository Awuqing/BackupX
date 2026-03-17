#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PREFIX="${PREFIX:-/opt/backupx}"
ETC_DIR="${ETC_DIR:-/etc/backupx}"
SERVICE_NAME="backupx"
APP_USER="backupx"
APP_GROUP="backupx"
BIN_SOURCE="${BIN_SOURCE:-$PROJECT_ROOT/server/backupx}"
WEB_SOURCE="${WEB_SOURCE:-$PROJECT_ROOT/web/dist}"
CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-$PROJECT_ROOT/server/config.example.yaml}"
SERVICE_SOURCE="${SERVICE_SOURCE:-$PROJECT_ROOT/deploy/backupx.service}"
NGINX_SOURCE="${NGINX_SOURCE:-$PROJECT_ROOT/deploy/nginx.conf}"

if [ "$(id -u)" -ne 0 ]; then
    echo "请使用 root 或 sudo 执行安装脚本。" >&2
    exit 1
fi

if [ ! -f "$BIN_SOURCE" ]; then
    echo "未找到后端二进制：$BIN_SOURCE" >&2
    echo "请先执行：cd \"$PROJECT_ROOT/server\" && go build -o backupx ./cmd/backupx" >&2
    exit 1
fi

if [ ! -d "$WEB_SOURCE" ]; then
    echo "未找到前端构建产物：$WEB_SOURCE" >&2
    echo "请先执行：cd \"$PROJECT_ROOT/web\" && npm run build" >&2
    exit 1
fi

if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
    groupadd --system "$APP_GROUP"
fi

if ! id "$APP_USER" >/dev/null 2>&1; then
    useradd --system --gid "$APP_GROUP" --home-dir "$PREFIX" --shell /usr/sbin/nologin "$APP_USER"
fi

install -d -o "$APP_USER" -g "$APP_GROUP" "$PREFIX" "$PREFIX/bin" "$PREFIX/web" "$PREFIX/data" "$ETC_DIR"
install -m 0755 "$BIN_SOURCE" "$PREFIX/bin/backupx"
cp -R "$WEB_SOURCE/." "$PREFIX/web/"
chown -R "$APP_USER:$APP_GROUP" "$PREFIX"

if [ ! -f "$ETC_DIR/config.yaml" ]; then
    install -m 0640 "$CONFIG_TEMPLATE" "$ETC_DIR/config.yaml"
fi

install -m 0644 "$SERVICE_SOURCE" "/etc/systemd/system/$SERVICE_NAME.service"
systemctl daemon-reload
systemctl enable --now "$SERVICE_NAME"

if [ -d "/etc/nginx/conf.d" ]; then
    install -m 0644 "$NGINX_SOURCE" "/etc/nginx/conf.d/$SERVICE_NAME.conf"
    if command -v nginx >/dev/null 2>&1; then
        nginx -t
        systemctl reload nginx || true
    fi
fi

cat <<MESSAGE
安装完成。

- 二进制目录：$PREFIX/bin/backupx
- 前端目录：$PREFIX/web
- 配置文件：$ETC_DIR/config.yaml
- systemd 服务：/etc/systemd/system/$SERVICE_NAME.service

如需修改监听地址、数据库路径或日志级别，请编辑 "$ETC_DIR/config.yaml" 后执行：
  systemctl restart "$SERVICE_NAME"
MESSAGE
