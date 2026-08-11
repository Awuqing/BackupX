#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
PREFIX="${PREFIX:-/opt/backupx}"
ETC_DIR="${ETC_DIR:-/etc/backupx}"
SERVICE_NAME="backupx"
APP_USER="backupx"
APP_GROUP="backupx"
if [ -f "$SCRIPT_DIR/backupx" ] && [ -d "$SCRIPT_DIR/web" ]; then
    BIN_SOURCE="${BIN_SOURCE:-$SCRIPT_DIR/backupx}"
    WEB_SOURCE="${WEB_SOURCE:-$SCRIPT_DIR/web}"
    CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-$SCRIPT_DIR/config.example.yaml}"
    NGINX_SOURCE="${NGINX_SOURCE:-$SCRIPT_DIR/nginx.conf}"
    SERVICE_SOURCE_DEFAULT="$SCRIPT_DIR/backupx.service"
else
    SOURCE_BIN_DEFAULT="$PROJECT_ROOT/server/bin/backupx"
    # Keep compatibility with contributors who built the historical path by
    # hand, while matching the canonical `make build` output first.
    if [ ! -f "$SOURCE_BIN_DEFAULT" ] && [ -f "$PROJECT_ROOT/server/backupx" ]; then
        SOURCE_BIN_DEFAULT="$PROJECT_ROOT/server/backupx"
    fi
    BIN_SOURCE="${BIN_SOURCE:-$SOURCE_BIN_DEFAULT}"
    WEB_SOURCE="${WEB_SOURCE:-$PROJECT_ROOT/web/dist}"
    CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-$PROJECT_ROOT/server/config.example.yaml}"
    NGINX_SOURCE="${NGINX_SOURCE:-$PROJECT_ROOT/deploy/nginx.conf}"
    SERVICE_SOURCE_DEFAULT="$PROJECT_ROOT/deploy/backupx.service"
fi
SERVICE_SOURCE_EXPLICIT=0
if [ -n "${SERVICE_SOURCE:-}" ]; then
    SERVICE_SOURCE_EXPLICIT=1
fi
SERVICE_SOURCE="${SERVICE_SOURCE:-$SERVICE_SOURCE_DEFAULT}"
INSTALL_NGINX="${INSTALL_NGINX:-0}"

if [ "$(id -u)" -ne 0 ]; then
    echo "请使用 root 或 sudo 执行安装脚本。" >&2
    exit 1
fi

validate_install_path() {
    path_name="$1"
    path_value="$2"
    case "$path_value" in
        /*) ;;
        *) echo "$path_name 必须是绝对路径: $path_value" >&2; exit 1 ;;
    esac
    case "$path_value" in
        /|*"//"*|*"/./"*|*"/."|*"/../"*|*"/.."|*[!A-Za-z0-9_./+-]*)
            echo "$path_name 必须是规范、安全且非根目录的绝对路径: $path_value" >&2
            exit 1
            ;;
    esac
}

validate_install_path PREFIX "$PREFIX"
validate_install_path ETC_DIR "$ETC_DIR"

if [ ! -f "$BIN_SOURCE" ]; then
    echo "Backend binary not found / 未找到后端二进制：$BIN_SOURCE" >&2
    echo "源码树安装请先在仓库根目录执行 make build（产物：server/bin/backupx）。" >&2
    echo "For a source install, run 'make build' in the repository root first." >&2
    echo "发布包安装请确认当前目录包含 ./backupx、./web 和 ./install.sh。" >&2
    exit 1
fi

if [ ! -d "$WEB_SOURCE" ]; then
    echo "未找到前端构建产物：$WEB_SOURCE" >&2
    echo "源码树安装请先执行：cd \"$PROJECT_ROOT/web\" && npm run build" >&2
    echo "发布包安装请确认当前目录包含 ./web。" >&2
    exit 1
fi

if [ ! -f "$CONFIG_TEMPLATE" ]; then
    echo "未找到配置模板：$CONFIG_TEMPLATE" >&2
    exit 1
fi

if [ "$SERVICE_SOURCE_EXPLICIT" = "1" ] && [ ! -f "$SERVICE_SOURCE" ]; then
    echo "指定的 systemd unit 不存在：$SERVICE_SOURCE" >&2
    exit 1
fi

for managed_path in "$PREFIX" "$PREFIX/bin" "$PREFIX/web" "$PREFIX/data" "$ETC_DIR"; do
    if [ -L "$managed_path" ]; then
        echo "拒绝通过符号链接写入受管目录：$managed_path" >&2
        exit 1
    fi
done

if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
    groupadd --system "$APP_GROUP"
fi

if ! id "$APP_USER" >/dev/null 2>&1; then
    useradd --system --gid "$APP_GROUP" --home-dir "$PREFIX" --shell /usr/sbin/nologin "$APP_USER"
fi

install -d -o root -g root -m 0755 "$PREFIX" "$PREFIX/bin" "$PREFIX/web"
install -d -o "$APP_USER" -g "$APP_GROUP" -m 0750 "$PREFIX/data"
install -d -o root -g "$APP_GROUP" -m 0750 "$ETC_DIR"
install -o root -g root -m 0755 "$BIN_SOURCE" "$PREFIX/bin/backupx.new"
mv -f "$PREFIX/bin/backupx.new" "$PREFIX/bin/backupx"
cp -R "$WEB_SOURCE/." "$PREFIX/web/"
chown -R root:root "$PREFIX/bin" "$PREFIX/web"
find "$PREFIX/web" -type d -exec chmod 0755 {} \;
find "$PREFIX/web" -type f -exec chmod 0644 {} \;
chown -R "$APP_USER:$APP_GROUP" "$PREFIX/data"

if [ ! -f "$ETC_DIR/config.yaml" ]; then
    install -o root -g "$APP_GROUP" -m 0640 "$CONFIG_TEMPLATE" "$ETC_DIR/config.yaml"
fi
# 服务账户只需读取配置，不应拥有修改 /etc 配置或可执行文件的权限。
chown root:"$APP_GROUP" "$ETC_DIR/config.yaml"
chmod 0640 "$ETC_DIR/config.yaml"

# 仓库 unit 使用标准路径；自定义 PREFIX/ETC_DIR 时动态生成以保持路径一致。
# 显式传入 SERVICE_SOURCE 表示调用方已经审核其中的路径，始终优先使用。
if [ -f "$SERVICE_SOURCE" ] && { [ "$SERVICE_SOURCE_EXPLICIT" = "1" ] || { [ "$PREFIX" = "/opt/backupx" ] && [ "$ETC_DIR" = "/etc/backupx" ]; }; }; then
    install -m 0644 "$SERVICE_SOURCE" "/etc/systemd/system/$SERVICE_NAME.service"
else
    cat > "/etc/systemd/system/$SERVICE_NAME.service" <<UNIT
[Unit]
Description=BackupX API Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_GROUP
WorkingDirectory=$PREFIX
ExecStart=$PREFIX/bin/backupx -config $ETC_DIR/config.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
UMask=0027
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
fi
systemctl daemon-reload
if ! systemctl enable "$SERVICE_NAME" || ! systemctl restart "$SERVICE_NAME"; then
    echo "BackupX systemd 服务启动失败。" >&2
    systemctl status "$SERVICE_NAME" --no-pager >&2 || true
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager >&2 || true
    exit 1
fi

# systemctl may return before the process has opened its HTTP listener. Verify
# the same unauthenticated endpoint used by the first-administrator screen so a
# broken bare-metal install cannot print a false success message.
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8340/api/auth/setup/status}"
READY=0
ATTEMPT=1
while [ "$ATTEMPT" -le 30 ]; do
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        if command -v curl >/dev/null 2>&1; then
            if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
                READY=1
                break
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q -T 2 -O /dev/null "$HEALTH_URL"; then
                READY=1
                break
            fi
        else
            echo "Warning / 警告：未找到 curl 或 wget，仅验证 systemd 服务状态。" >&2
            READY=1
            break
        fi
    fi
    ATTEMPT=$((ATTEMPT + 1))
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "BackupX did not become ready at $HEALTH_URL / 服务未通过就绪检查。" >&2
    systemctl status "$SERVICE_NAME" --no-pager >&2 || true
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager >&2 || true
    exit 1
fi

if [ "$INSTALL_NGINX" = "1" ]; then
    if [ ! -d "/etc/nginx/conf.d" ] || [ ! -f "$NGINX_SOURCE" ]; then
        echo "已请求安装 Nginx 配置，但未找到 /etc/nginx/conf.d 或配置模板。" >&2
        exit 1
    fi
    install -o root -g root -m 0644 "$NGINX_SOURCE" "/etc/nginx/conf.d/$SERVICE_NAME.conf"
    command -v nginx >/dev/null 2>&1 || { echo "未找到 nginx 命令。" >&2; exit 1; }
    nginx -t
    systemctl reload nginx
fi

cat <<MESSAGE
安装完成。

- 二进制目录：$PREFIX/bin/backupx
- 前端目录：$PREFIX/web
- 配置文件：$ETC_DIR/config.yaml
- systemd 服务：/etc/systemd/system/$SERVICE_NAME.service

Web 控制台已由后端直接托管，无需额外的 nginx 反向代理即可访问：
  http://<本机IP>:8340

首次访问 / First sign-in:
  1. 打开上面的地址，并可在登录页右上角选择 中文 或 English。
  2. 页面显示“系统初始化 / System setup”时，创建首个管理员用户名和密码。
  3. 如果未显示初始化表单，请先检查：$HEALTH_URL

如需安装仓库提供的 Nginx 模板，请审核域名与 TLS 配置后重新执行：
  sudo INSTALL_NGINX=1 ./install.sh

排查：若服务未监听端口，请查看日志：
  journalctl -u "$SERVICE_NAME" -n 50 --no-pager

如需修改监听地址、数据库路径或日志级别，请编辑 "$ETC_DIR/config.yaml" 后执行：
  systemctl restart "$SERVICE_NAME"
MESSAGE
