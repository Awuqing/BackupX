---
sidebar_position: 4
title: Multi-Node Cluster
description: Master-Agent mode — route backups to remote servers via HTTP long-polling.
---

# Multi-Node Cluster

BackupX supports Master-Agent mode: backup tasks can be routed to specific nodes. The Agent runs the backup locally and uploads straight to storage. All connections are initiated by the Agent, so remote networks only need outbound HTTP access.

## Architecture

```
[Web Console] ─── JWT ──→ [Master (backupx)]
                              ↑  ↓
                              │  │ HTTP long-poll (token auth)
                              │  ↓
                         [Agent (backupx agent)]   ← runs on remote host
                              ↓
                     [70+ Storage Backends]
```

- **Protocol** — HTTP long-polling; the Agent initiates every connection
- **Heartbeat** — Agent reports every 15s; Master marks nodes offline after 45s of silence
- **Dispatch** — Master persists `run_task` commands to a queue; Agent polls and claims them
- **Execution** — Agent reuses the same BackupRunner (file / mysql / postgresql / sqlite / saphana) and uploads directly to storage
- **Security** — Each node has its own token; the Agent never holds the Master's JWT secret or AES-256 key

## Walkthrough

### 1. Create a node on Master

Web Console → **Node Management** → **Add Node**. A 64-byte hex token is shown **once** — keep it safe.

### 2. Deploy the Agent on a remote host

Upload the BackupX binary (same file as Master) to the target host, then start the Agent:

**Option A: CLI flags**

```bash
backupx agent --master http://master.example.com:8340 --token <token>
```

**Option B: config file**

```yaml title="/etc/backupx/agent.yaml"
master: http://master.example.com:8340
token: <token>
heartbeatInterval: 15s
pollInterval: 5s
tempDir: /var/lib/backupx-agent
```

```bash
backupx agent --config /etc/backupx/agent.yaml
```

**Option C: environment variables** (Docker / systemd friendly)

```bash
BACKUPX_AGENT_MASTER=http://master.example.com:8340 \
BACKUPX_AGENT_TOKEN=<token> \
backupx agent
```

Once connected, the node shows as **online** in the list.

### 3. Route a task to the node

In the **Backup Tasks** page, pick the target node when creating the task. When the task runs:

- Local (`nodeId=0`) → Master executes in-process
- Remote node → Master enqueues the command → Agent claims → Agent runs locally → uploads → reports back

## Known limitations

- **Encrypted backups don't work via Agent** — the Agent doesn't hold Master's AES-256 key. Tasks with `encrypt: true` will fail if routed to an Agent
- **Directory browser timeout** — remote dir listing is a synchronous RPC through the queue (15s default)
- **Dispatched command timeout** — claimed-but-unfinished commands are marked `timeout` after 10 minutes

## CLI reference

```
backupx agent --help
  -master string    Master URL
  -token string     Agent auth token
  -config string    YAML config path (takes precedence over env)
  -temp-dir string  Local temp directory (default /tmp/backupx-agent)
  -insecure-tls     Skip TLS verification (testing only)
```

## systemd unit

```ini title="/etc/systemd/system/backupx-agent.service"
[Unit]
Description=BackupX Agent
After=network.target

[Service]
Type=simple
User=backupx
Environment="BACKUPX_AGENT_MASTER=https://master.example.com"
Environment="BACKUPX_AGENT_TOKEN=your-token"
ExecStart=/opt/backupx/backupx agent
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable --now backupx-agent
sudo journalctl -u backupx-agent -f
```
