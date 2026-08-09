---
sidebar_position: 2
title: CLI 参考
description: backupx 子命令 — server / agent / backint / reset-password。
---

# CLI 参考

`backupx` 二进制内置多个子命令。无子命令时默认启动主服务进程。

## `backupx`（默认：服务进程）

```bash
backupx --config /opt/backupx/config.yaml
backupx --version
```

| 参数 | 说明 |
|------|------|
| `--config <path>` | 显式配置文件路径；省略时使用下方查找路径 |
| `--version` | 打印版本后退出 |

未提供 `--config` 时，服务端依次查找 `./config.yaml`、`./server/config.yaml` 和 `/etc/backupx/config.yaml`。`BACKUPX_*` 环境变量会覆盖对应服务端配置项，详见[配置参考](../deployment/configuration)。

## `backupx agent`

以 Agent 模式运行，连接到 Master。详见 [多节点集群](../features/multi-node)。

```bash
backupx agent --master https://backup.example.com --token-file /etc/backupx-agent/agent.token
```

| 参数 | 说明 |
|------|------|
| `--master <url>` | Master URL |
| `--token <token>` | Agent 认证令牌 |
| `--token-file <path>` | 从文件读取 Agent Token，服务与容器部署推荐使用 |
| `--config <path>` | 加载 Agent YAML；提供后不再加载基于环境变量的 Agent 配置 |
| `--temp-dir <path>` | 本地临时目录（默认 `/var/lib/backupx-agent/tmp`） |
| `--proxy-url <url>` | 显式 HTTP(S) 或 SOCKS5(H) 代理 |
| `--ca-cert <path>` | 用于校验 Master 的 PEM CA 证书 |
| `--insecure-tls` | 跳过 TLS 校验（仅测试用） |

Agent 配置优先级为显式 CLI 参数高于 YAML。未提供 `--config` 时，配置从 `BACKUPX_AGENT_MASTER`、`BACKUPX_AGENT_TOKEN`、`BACKUPX_AGENT_TOKEN_FILE`、`BACKUPX_AGENT_HEARTBEAT`、`BACKUPX_AGENT_POLL`、`BACKUPX_AGENT_TEMP_DIR`、`BACKUPX_AGENT_PROXY_URL`、`BACKUPX_AGENT_CA_CERT_FILE` 和 `BACKUPX_AGENT_INSECURE_TLS` 加载。未设置显式代理时，Agent 还会遵循 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY`。

`--token` 优先于 `--token-file`。长期 Token 应放在仅 root 可读的文件中，不要进入命令历史。私有 CA 与 `--insecure-tls` 不能同时启用。

## `backupx backint`

SAP HANA Backint 协议代理，详见 [SAP HANA 支持](../features/sap-hana)。

```bash
backupx backint -f <function> -i <input> -o <output> -p <params>
```

| 参数 | 说明 |
|------|------|
| `-f <fn>` | `backup` / `restore` / `inquire` / `delete` |
| `-i <path>` | 输入文件 |
| `-o <path>` | 输出文件 |
| `-p <path>` | 参数文件 |
| `-u / -c / -l / -v` | 接收但忽略（兼容 SAP 约定） |

`-p` 参数文件必须定义 `STORAGE_TYPE`，并提供 `STORAGE_CONFIG_JSON` 或 `STORAGE_CONFIG`。可选项包括 `PARALLEL_FACTOR`、`COMPRESS`、`LOG_FILE`、`CATALOG_DB` 和 `KEY_PREFIX`。

## `backupx reset-password`

直接在 SQLite 中重置管理员密码，无需重启服务。

```bash
backupx reset-password --username admin --password 'newpass123' [--config /path/to/config.yaml]
```

| 参数 | 说明 |
|------|------|
| `--username` | 目标用户名（默认 `admin`） |
| `--password` | 新密码（最少 8 字符，必填） |
| `--config` | 配置文件路径（用于定位数据库文件） |

该命令应在可访问配置中 SQLite 路径的 Master 主机执行。不要把新密码直接写入长期保留的 shell 历史。
