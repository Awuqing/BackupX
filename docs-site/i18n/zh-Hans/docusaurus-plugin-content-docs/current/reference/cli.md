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
| `--config <path>` | 配置文件路径（默认 `./config.yaml`） |
| `--version` | 打印版本后退出 |

## `backupx agent`

以 Agent 模式运行，连接到 Master。详见 [多节点集群](../features/multi-node)。

```bash
backupx agent --master http://master:8340 --token <token>
```

| 参数 | 说明 |
|------|------|
| `--master <url>` | Master URL |
| `--token <token>` | Agent 认证令牌 |
| `--config <path>` | YAML 配置文件（优先级高于环境变量） |
| `--temp-dir <path>` | 本地临时目录（默认 `/tmp/backupx-agent`） |
| `--insecure-tls` | 跳过 TLS 校验（仅测试用） |

环境变量：`BACKUPX_AGENT_MASTER`、`BACKUPX_AGENT_TOKEN`、`BACKUPX_AGENT_HEARTBEAT`、`BACKUPX_AGENT_POLL`、`BACKUPX_AGENT_TEMP_DIR`、`BACKUPX_AGENT_INSECURE_TLS`。

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
