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
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Forwarded-Port $server_port;
        proxy_set_header Connection "";

        # 大文件上传（用于恢复流程）
        client_max_body_size 0;
        proxy_request_buffering off;

        # 实时日志使用 SSE，必须关闭缓冲
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # 兼容旧版本生成的安装地址；新版本通过上面的 /api/install/ 访问。
    location /install/ {
        proxy_pass http://127.0.0.1:8340/install/;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header X-Forwarded-Port $server_port;
    }

    # 避免探针和指标请求落入 SPA fallback。
    location = /health { proxy_pass http://127.0.0.1:8340/health; }
    location = /ready  { proxy_pass http://127.0.0.1:8340/ready; }
    location = /metrics { proxy_pass http://127.0.0.1:8340/metrics; }
}
```

集群使用 Master 中转备份时必须保留 `proxy_request_buffering off`。否则 Nginx 会先把 Agent 上传的完整备份写入代理临时目录，再交给 BackupX，既失去流式传输优势，也可能占满代理磁盘。

如果 Nginx 运行在另一台主机或另一个容器，只把该代理的 IP 或网段加入 `server.trusted_proxies`，不要配置 `0.0.0.0/0`。登录限流、安装令牌限流和审计日志都依赖可信的客户端地址。

`/health`、`/ready` 和 `/metrics` 不需要 BackupX 认证。应只放行探针与 Prometheus 来源网段，或把这些 location 放在内部监听端口，避免直接暴露到互联网。

## certbot 配置 HTTPS

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d backup.example.com
```

certbot 会自动改写配置监听 443 并设置续期。

:::caution Agent 需要稳定的 URL
如果 Master 部署在 HTTPS 后面，远程 Agent 的 `--master` 必须使用最终 HTTPS 地址，Agent 不会跟随重定向。私有 CA 应预先下发 PEM 证书并使用 `--ca-cert /path/to/ca.pem`；`--insecure-tls` 只用于短期测试。
:::
