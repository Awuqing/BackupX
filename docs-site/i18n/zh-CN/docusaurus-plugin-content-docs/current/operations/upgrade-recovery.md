---
sidebar_position: 1
title: 升级与恢复
description: 备份控制面、安全升级、配套回滚并恢复故障 Master。
---

# 升级与恢复

备份产物与 BackupX 控制面属于两个不同的恢复域。对象存储中可能仍保留全部归档，但 Master 数据库丢失会同时丢失用户、加密后的存储凭据、计划、记录、节点 Token 和审计历史，因此两者都必须保护。

## 必须遵守的规则

1. 同一个数据目录或 SQLite 数据库只能运行一个活动 Master。
2. 停止 Master 后快照完整数据目录与配置，或使用覆盖整个存储卷的原子快照。
3. 旧应用版本必须与其升级前数据快照配套保留。启动时会执行数据库迁移，只切回旧二进制或旧镜像不是安全回滚。
4. 控制面快照应保存到 Master 主机之外，并定期验证恢复。
5. 停止 Master 前，先等待正在运行的备份、恢复、验证和复制任务结束。

| 部署方式 | 持久化控制面数据 | 配置与版本状态 |
| --- | --- | --- |
| Docker | `backupx-data` 卷中的 `/app/data` | Compose 文件、受保护的 `.env`、固定的镜像标签或摘要 |
| 裸机 | `/opt/backupx/data` | `/etc/backupx`、`/opt/backupx/bin`、`/opt/backupx/web`、systemd unit |

配置未显式提供 JWT 与加密密钥时，自动生成的值保存在 SQLite 数据库中。所有控制面快照都应按敏感数据管理。

## 变更前检查

升级、迁移主机或修改安全密钥前：

- 记录当前 BackupX 版本以及准确的镜像摘要或 Release 校验和。
- 确认 `/ready` 返回 HTTP 200，并检查近期失败记录。
- 等待正在运行的备份、恢复、验证和复制结束。
- 测试至少一个存储目标，并确认 Agent 在线。
- 创建完整控制面快照，校验后复制到异机。
- 可额外导出任务定义供人工审阅。任务导出不包含数据库密码与存储凭据，不能代替数据库快照。
- 开始前确定回滚条件与维护窗口截止时间。

## 快照 Docker 部署

下面的示例无需直接访问 Docker 卷目录，即可生成一致的文件级副本：

~~~bash
snapshot="backupx-control-plane-$(date -u +%Y%m%dT%H%M%SZ)"
install -d -m 0700 "$snapshot"

docker compose stop backupx
docker cp backupx:/app/data "$snapshot/data"
cp docker-compose.yml "$snapshot/"
if [ -f .env ]; then cp .env "$snapshot/"; fi
docker compose start backupx

tar -czf "$snapshot.tar.gz" "$snapshot"
sha256sum "$snapshot.tar.gz" > "$snapshot.tar.gz.sha256"
curl -fsS http://127.0.0.1:8340/ready
~~~

复制失败时，应先恢复已停止的服务，再继续排查。归档中的 `.env` 和数据库可能包含凭据，必须限制访问。如果块存储或云平台快照能原子覆盖整个卷，也可以直接使用。

## 快照裸机部署

~~~bash
snapshot="/var/backups/backupx/backupx-control-plane-$(date -u +%Y%m%dT%H%M%SZ).tar.gz"
sudo install -d -m 0700 /var/backups/backupx

sudo systemctl stop backupx
sudo tar --acls --xattrs -C / -czf "$snapshot" \
  etc/backupx \
  etc/systemd/system/backupx.service \
  opt/backupx/bin \
  opt/backupx/web \
  opt/backupx/data
sudo systemctl start backupx

sudo sha256sum "$snapshot" | sudo tee "$snapshot.sha256"
curl -fsS http://127.0.0.1:8340/ready
~~~

把归档及校验和复制到受保护的异机存储。不要在服务运行时只复制 `backupx.db`。

## 升级 Docker

1. 在 `BACKUPX_IMAGE` 中使用 Release 标签或不可变摘要，受控生产升级不要使用 `latest`。
2. 创建并验证升级前快照。
3. 拉取并重建服务：

~~~bash
docker compose pull backupx
docker compose up -d backupx
docker compose ps
docker compose logs --tail=100 backupx
curl -fsS http://127.0.0.1:8340/ready
~~~

4. 登录后测试存储目标，确认 Agent 心跳，并执行一个小型备份以及一次恢复或验证演练。
5. 观察窗口结束前保留旧镜像引用与快照。

Master 完成后再小批量升级 Agent。除非变更目标就是网络配置，否则不要改动节点专用代理、私有 CA、Token 文件和堡垒机参数。

## 升级裸机

下载目标 Release 与校验和，完成校验后解压。安装器会保留已有的 `/etc/backupx/config.yaml`，替换二进制、前端文件和 systemd unit，并重启服务。

~~~bash
sha256sum -c backupx-vX.Y.Z-linux-amd64.tar.gz.sha256
tar xzf backupx-vX.Y.Z-linux-amd64.tar.gz
cd backupx-vX.Y.Z-linux-amd64
sudo ./install.sh

sudo systemctl status backupx --no-pager
curl -fsS http://127.0.0.1:8340/ready
~~~

运行安装器前必须先创建停服快照。升级后的业务检查与 Docker 相同。

## 回滚

回滚是配套操作：必须同时恢复旧应用版本和紧邻升级前创建的数据快照。

Docker 应保留故障卷用于分析，把快照恢复到新的空卷，Compose 同时指向该卷与旧镜像标签，然后只启动一个 Master。裸机应停止服务并保留故障现场，从同一归档恢复旧配置、二进制、前端、数据目录和 unit，重新加载 systemd 后启动。

回滚后检查：

~~~bash
curl -fsS http://127.0.0.1:8340/health
curl -fsS http://127.0.0.1:8340/ready
~~~

随后验证登录、存储访问、计划、Agent 心跳、一次备份和一次非破坏性恢复演练。在事故原因明确前不要删除故障现场。

## 恢复丢失的 Master

1. 按快照记录准备同架构主机与完全相同的应用版本。
2. 替代主机先与生产流量隔离，并确保旧 Master 无法再次启动。
3. 按原权限恢复配置和完整数据目录。
4. 只启动一个 Master，在本机检查 `/ready`。
5. 本地验证完成后再切换稳定 DNS 名称或虚拟 IP。
6. 检查用户、存储目标、任务、记录、通知和审计历史。
7. 数据库内 Token 与节点一致时，已有 Agent 会自动重连；可能泄露的 Token 必须调查并轮换。
8. 执行小型备份及恢复或验证演练后再结束事故处理。

恢复控制面不会重新生成外部备份产物，它们仍位于原存储目标。反过来，任务 JSON 导出只适合辅助重建计划，不包含密钥、存储定义和部分节点绑定，不能作为完整灾备。

## 验证恢复计划

至少每季度把近期快照恢复到隔离网络，启动快照记录的 BackupX 版本并验证：

- 不接触生产 Master 时，`/ready` 能恢复正常。
- 管理员可登录，已加密的存储配置可读取。
- 任务、节点、记录和审计数量合理。
- 可以测试一个存储目标而不写入生产数据。
- 选定备份可验证，或可恢复到隔离目录。

记录恢复耗时和最新可恢复快照时间，这两个实测值才是控制面的真实 RTO 与 RPO。
