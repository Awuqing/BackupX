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
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Forwarded-Port $server_port;

        # Large uploads (restore flow)
        client_max_body_size 0;
        proxy_request_buffering off;

        # Live log stream uses SSE — buffering must be off
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

`proxy_request_buffering off` is required for Master-relay cluster backups. Without it, Nginx writes the complete Agent upload to its temporary storage before BackupX receives it, defeating streaming and potentially filling the proxy disk.

If Nginx runs on another host or in another container, add only that proxy IP or subnet to `server.trusted_proxies`. Do not use `0.0.0.0/0`; BackupX uses the trusted client address for login throttling, install-token throttling, and audit records.

## HTTPS with certbot

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d backup.example.com
```

Certbot rewrites the config to listen on 443 with auto-renewal.

:::caution Agent needs a stable URL
If Master is behind HTTPS, remote Agent deployments must use the public HTTPS URL for `--master`. Self-signed certs require `--insecure-tls` (testing only).
:::
