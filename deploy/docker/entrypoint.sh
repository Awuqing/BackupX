#!/bin/sh
set -e

# Backend listens on internal port 8341, Nginx exposes 8340
export BACKUPX_SERVER_PORT="${BACKUPX_SERVER_PORT_INTERNAL:-8341}"

# Start Nginx in background
nginx -g "daemon off;" &
NGINX_PID=$!

# Start BackupX backend
/app/bin/backupx &
APP_PID=$!

# Trap signals for graceful shutdown
trap 'kill $APP_PID $NGINX_PID 2>/dev/null; wait $APP_PID $NGINX_PID 2>/dev/null' SIGTERM SIGINT

echo "BackupX started — Nginx :8340 -> Backend :8341"

# Wait for either process to exit
wait -n $APP_PID $NGINX_PID 2>/dev/null || true
kill $APP_PID $NGINX_PID 2>/dev/null || true
wait $APP_PID $NGINX_PID 2>/dev/null || true
