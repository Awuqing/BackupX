---
sidebar_position: 3
title: Nginx Reverse Proxy
description: Expose BackupX behind Nginx with HTTPS and SSE-friendly buffering disabled.
---

# Nginx Reverse Proxy

A minimal production-ready Nginx site for BackupX:

```nginx title="/etc/nginx/sites-available/backupx"
server {
    listen 80;
    server_name backup.example.com;

    # Static UI (served from /opt/backupx/web)
    location / {
        root /opt/backupx/web;
        try_files $uri $uri/ /index.html;
    }

    # API reverse proxy
    location /api/ {
        proxy_pass http://127.0.0.1:8340;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Large uploads (restore flow)
        client_max_body_size 0;

        # Live log stream uses SSE — buffering must be off
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

## HTTPS with certbot

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d backup.example.com
```

Certbot rewrites the config to listen on 443 with auto-renewal.

:::caution Agent needs a stable URL
If Master is behind HTTPS, remote Agent deployments must use the public HTTPS URL for `--master`. Self-signed certs require `--insecure-tls` (testing only).
:::
