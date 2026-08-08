#!/bin/sh
set -eu

# 旧镜像曾以 root 写入数据卷。Master 启动时做一次所有权迁移，随后
# 降权运行；Agent 模式由部署命令显式决定用户，以访问宿主机备份路径。
if [ "$(id -u)" -eq 0 ] && [ "${1:-}" != "agent" ]; then
    chown backupx:backupx /app/data /tmp/backupx
    if [ ! -f /app/data/.backupx-owner-v2 ]; then
        chown -R backupx:backupx /app/data
        su-exec backupx:backupx touch /app/data/.backupx-owner-v2
    fi
    export HOME=/app
    exec su-exec backupx:backupx /app/bin/backupx "$@"
fi

# Web 静态文件由 BackupX 后端直接托管。容器只运行一个前台进程，
# 让 Docker 准确传递信号、收集退出码并执行健康检查。
exec /app/bin/backupx "$@"
