---
sidebar_position: 4
title: 故障排查
description: Master、反向代理、Agent、备份工具和 SQLite 的安全诊断顺序。
---

# 故障排查

从最先失败的边界开始并保留证据。在确认原因前，不要删除数据库、重建卷、一次性轮换所有 Token 或重新安装。

## 快速分流

| 现象 | 首项检查 | 可能边界 |
| --- | --- | --- |
| Web 控制台不可用 | 本机 `/health`，再检查代理 `/health` | 进程、监听、防火墙、代理或静态文件 |
| `/health` 正常但 `/ready` 为 503 | 服务日志、数据库路径、磁盘、权限 | SQLite 或数据文件系统 |
| 登录循环或客户端 IP 错误 | 转发头与 `trusted_proxies` | 反向代理信任 |
| 实时日志停止更新 | Nginx 响应缓冲与超时 | SSE 代理路径 |
| 中转上传停滞或代理磁盘占满 | 请求缓冲与 Body 上限 | 反向代理 |
| Agent 离线 | Agent 日志、最终 Master URL、代理、DNS、CA | Agent 到 Master 网络 |
| 备份启动后失败 | 记录日志、源路径、数据库原生工具 | Runner 或权限 |
| 恢复失败 | 记录日志、目标挂载与写权限 | 存储读取或目标权限 |

## 无敏感信息的状态采集

Docker Master：

~~~bash
docker compose ps
docker compose logs --tail=200 backupx
curl -i http://127.0.0.1:8340/health
curl -i http://127.0.0.1:8340/ready
~~~

裸机 Master：

~~~bash
sudo systemctl status backupx --no-pager
sudo journalctl -u backupx -n 200 --no-pager
sudo ss -lntp | grep 8340
curl -i http://127.0.0.1:8340/health
curl -i http://127.0.0.1:8340/ready
~~~

systemd Agent：

~~~bash
sudo systemctl status backupx-agent --no-pager
sudo journalctl -u backupx-agent -n 200 --no-pager
sudo systemctl status backupx-agent-tunnel --no-pager
~~~

最后一条只适用于堡垒机部署。共享输出前，移除 Authorization 头、API Key、Agent Token、安装 URL、数据库密码、存储凭据、代理凭据以及会暴露敏感拓扑的私有路径。

## Web 控制台或首次初始化

检查无需认证的初始化端点：

~~~bash
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
~~~

API 正常但浏览器出现空白页或 JSON 时：

- 确认 Release 包含前端文件。
- 裸机检查 `/opt/backupx/web` 可读；显式配置时确认 `server.web_root` 正确。
- Docker 确认运行正式镜像，且自定义挂载未覆盖镜像内前端目录。
- Nginx 静态模式确认 `root /opt/backupx/web` 和 SPA fallback 存在。
- 版本变更后清理旧 Service Worker 或浏览器缓存。

认证失败先校验系统时间，再排查 TOTP 或通行密钥。确认浏览器 Origin 与最终 HTTPS 主机一致，并从审计日志检查限流、禁用用户或已撤销可信设备。

## 反向代理

验证并重载 Nginx：

~~~bash
sudo nginx -t
sudo systemctl reload nginx
curl -i https://backup.example.com/health
curl -i https://backup.example.com/ready
~~~

常见修正：

- HTTP 413：API 路由设置 `client_max_body_size 0`。
- 中转上传占满代理临时目录：设置 `proxy_request_buffering off`。
- SSE 日志批量到达或断开：设置 `proxy_buffering off`、关闭代理缓存并增加读取超时。
- 一键安装返回 HTML：代理 `/api/` 并保留旧版 `/install/` 路由。
- Agent 收到重定向：配置最终 HTTPS Master URL，不使用 HTTP 地址。
- 审计中所有用户都是代理 IP：只把真实代理 IP 或网段加入 `server.trusted_proxies`。

以完整的 [Nginx 配置](../deployment/nginx)作为对照基线。

## Agent 离线

Agent 通常每 15 秒发送一次心跳，45 秒无心跳后会被标记离线。

1. 确认 Agent 与可选隧道服务运行。
2. 确认 Master URL 没有重定向，且可在 Agent 主机解析。
3. 检查显式 `proxyUrl`；DNS 必须经过 SSH 动态隧道时使用 `socks5h://`。
4. 确认私有 CA 路径存在且可读，不要长期改为跳过 TLS。
5. 检查到 Master 与分配存储后端的出站防火墙。
6. 确认 `/etc/backupx-agent/agent.token` 存在且模式为 `0600`。
7. Token 已轮换时，在重叠窗口内写入新值并重启 Agent。

不要把 Token 直接放进会保存到 shell 历史的诊断命令。Agent 日志中的 401 通常表示 Token 缺失、重叠期已结束或节点不匹配；连续连接错误通常来自 URL、DNS、代理、隧道、防火墙或 CA。

## 备份任务失败

修改任务前先打开备份记录并阅读完整日志。

- 文件任务路径在所选 Master 或 Agent 上解析，确认路径存在于该主机命名空间。
- Docker 只能看到已挂载路径，备份源通常应只读。
- MySQL 要求执行主机 `PATH` 中存在 `mysqldump`。
- PostgreSQL 要求执行主机 `PATH` 中存在 `pg_dump`。
- SAP HANA Runner 模式要求对应客户端工具与环境。
- 确认服务账号可读源路径并可写临时目录。
- 从控制台测试所选存储目标。
- 远端存储应检查 DNS、出站策略、提供商配额、时钟偏差和代理。

配置多个目标时，应查看逐目标结果，不要假定所有副本都失败。修复失败目标时保留已经成功的远端产物。

## 恢复、下载或验证失败

- 确认远端产物仍存在且存储凭据可读取。
- 确认执行恢复的主机挂载了目标路径。
- 使用独立可写恢复目录，不要把所有备份源都改为可写。
- 检查目标与 Agent 临时目录剩余空间。
- 加密备份必须能取得原 Master 加密密钥。
- CDC 仓库的 Manifest、索引和共享 Pack 必须一起保留，单独 Manifest 不是完整备份。

诊断时优先恢复到隔离目录，不要反复覆盖生产源。

## SQLite 与就绪故障

`/health` 为 200 而 `/ready` 为 503 时：

1. 从服务日志读取准确数据库错误。
2. 检查磁盘空间、inode、路径所有权和挂载状态。
3. 确认数据目录只被一个 Master 进程或容器使用。
4. SQLite 应位于本地或块存储文件系统，不放在共享多写或不可靠网络文件系统。
5. 检查外部备份或防病毒进程是否长期占用文件。

BackupX 使用 5 秒 SQLite busy timeout，但这不会把 SQLite 变成集群数据库。不能通过启动另一个 Master 解决锁冲突。文件级复制应先停服，再复制整个数据目录。

## 升级问题材料

提交 Issue 时提供：

- BackupX 版本、安装方式、操作系统和架构。
- 故障影响 Master、Agent、代理、存储目标还是单个任务。
- 覆盖首次失败时段的脱敏日志。
- `/health` 与 `/ready` 的 HTTP 状态和响应体。
- 最小复现步骤，以及是否始于升级或配置变更。
- 脱敏后的相关代理配置。

不要向公开 Issue 附加 `backupx.db`、`.env`、完整配置、Agent Token 文件、API Key、安装命令或存储凭据。

涉及完整性或回滚时，应停止破坏性变更并参考[升级与恢复](./upgrade-recovery)。
