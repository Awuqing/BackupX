---
sidebar_position: 3
title: Nginx 反向代理
description: 通过 Nginx 发布 BackupX（HTTPS + SSE 友好的缓冲配置）。
---

# Nginx 反向代理

生产环境可用的 Nginx 站点模板：

```nginx title="/etc/nginx/sites-available/backupx"
server {
    listen 80;
    server_name backup.example.com;

    # 静态 UI（由 /opt/backupx/web 提供）
    location / {
        root /opt/backupx/web;
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理
    location /api/ {
        proxy_pass http://127.0.0.1:8340;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 大文件上传（用于恢复流程）
        client_max_body_size 0;

        # 实时日志使用 SSE，必须关闭缓冲
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

## certbot 配置 HTTPS

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d backup.example.com
```

certbot 会自动改写配置监听 443 并设置续期。

:::caution Agent 需要稳定的 URL
如果 Master 部署在 HTTPS 后面，远程 Agent 的 `--master` 必须使用公网 HTTPS 地址。自签名证书需加 `--insecure-tls`（仅供测试）。
:::
