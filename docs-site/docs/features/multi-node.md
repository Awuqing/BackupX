---
sidebar_position: 4
title: Multi-Node Cluster
description: Deploy BackupX Agents through direct HTTPS, forward proxies, or SSH bastions.
---

# Multi-Node Cluster

BackupX uses a single active Master as the control plane and an Agent on each source server. Agents initiate every connection, report a heartbeat every 15 seconds, and poll for commands every 5 seconds. No inbound Agent port is required.

## Architecture and boundaries

```text
[Web console] ────────> [Active Master + SQLite]
                              ^
                              | outbound HTTP(S) polling
                    +---------+---------+
                    |         |         |
                 [Agent B] [Agent C] [Agent D]
                    |         |         |
                    +----> storage targets
```

- Each node has an independent Agent Token. The Agent never receives the Master's JWT or encryption key.
- A node is marked offline after 45 seconds without a heartbeat.
- The Master persists commands; an Agent claims and executes them locally.
- Network storage is normally written directly by the Agent. A Master-local target can opt into authenticated streaming relay.

:::warning Single-active Master
The embedded SQLite database is not a shared multi-writer database. Run exactly one active Master against a data directory. For control-plane recovery, use an active/passive host, persistent-volume snapshots, and a stable DNS name or virtual IP. Never scale multiple Master replicas over the same `/app/data` or `backupx.db`.
:::

BackupX applies a five-second SQLite busy timeout and command-queue indexes to reduce contention from concurrent Agent polls and task updates. Keep the database on a local or block-backed filesystem. For a file-level control-plane backup, stop the Master before copying the whole data directory; do not copy only `backupx.db` while it is running.

## Choose a network path

| Scenario | Agent Master URL | Agent proxy URL | Notes |
| --- | --- | --- | --- |
| Routed network or public service | `https://backup.example.com` | empty | Recommended; allow only outbound TCP 443 |
| Corporate forward proxy | `https://backup.example.com` | `http://proxy.internal:3128` | HTTP(S) and SOCKS5(H) are supported |
| SSH dynamic tunnel through a bastion | `https://backup.internal` | `socks5h://127.0.0.1:1080` | Preserves TLS hostname and resolves internal DNS through the tunnel |
| SSH fixed local forward | `http://127.0.0.1:18340` | empty | The HTTP hop is protected by SSH; bind the forward to loopback only |

For private PKI, provide the absolute path of a pre-provisioned PEM CA certificate. Do not use `--insecure-tls` in production.

When no explicit proxy is configured, Agent-to-Master HTTP traffic follows `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`. A system service does not normally inherit an interactive shell's environment, so set the proxy in the install wizard or Agent YAML for systemd deployments.

## Prepare the Master

Set a stable URL before generating commands:

```yaml title="/etc/backupx/config.yaml"
server:
  external_url: "https://backup.example.com"
  trusted_proxies:
    - "127.0.0.1"
    - "::1"
    # Add the exact reverse-proxy IP or subnet when it is not local.
    # - "172.18.0.0/16"
```

`external_url` is the default install and Agent runtime address. A restricted node can override both generated target-side URLs with its tunnel or internal address while the browser continues to use the public address.

Use HTTPS across untrusted networks. For Master-relay uploads, configure the reverse proxy with unlimited request body size and request buffering disabled; see [Nginx Reverse Proxy](../deployment/nginx).

Configure the Agent with the final API URL, not an HTTP-to-HTTPS redirect. The Agent deliberately does not follow redirects so its authentication Token cannot be forwarded to an unintended host.

## Deploy an Agent

Open **Node Management → Add Node**:

1. Enter one node name, or up to 50 names in batch mode.
2. Select systemd, Docker, or foreground mode; architecture; Agent release; command TTL; and download source.
3. Select **Direct** or **Proxy or bastion**. For the restricted path, set an Agent-specific Master URL, proxy URL, or private CA path.
4. Copy the generated command to the target host and run it with root privileges.

Systemd is recommended for host-file backup and restore because the Agent needs access to arbitrary local paths. A Docker Agent sees only explicitly mounted paths; recreate it with read-only backup-source mounts and separately scoped writable restore destinations before assigning file tasks.

The URL-based command downloads a one-time installer and verifies its marker before execution. The wizard binds the selected Agent URL, explicit proxy, and private CA to that download command as well as to the installed Agent configuration. If the install endpoint is still unreachable, use the separately displayed embedded command. The embedded command contains the long-lived node Token and must be handled as a secret.

The installer:

1. Detects `linux/amd64` or `linux/arm64`.
2. Downloads the selected Release archive through the explicit proxy when configured, otherwise using the host's normal direct/environment-proxy route, and verifies its SHA-256 sidecar when the release provides one.
3. Writes `/etc/backupx-agent/config.yaml` and `/etc/backupx-agent/agent.token` with mode `0600`.
4. Keeps the Token out of the systemd unit and Docker environment metadata.
5. Starts the Agent and checks `/api/v1/agent/self` for up to 30 seconds.
6. Returns non-zero with systemd or Docker diagnostics when the node does not become online.

Older releases without checksum sidecars remain installable with a warning. New releases should always publish and verify the sidecar.

### Installed systemd configuration

```yaml title="/etc/backupx-agent/config.yaml"
master: "https://backup.example.com"
tokenFile: "/etc/backupx-agent/agent.token"
heartbeatInterval: "15s"
pollInterval: "5s"
tempDir: "/var/lib/backupx-agent/tmp"
proxyUrl: ""
caCertFile: ""
```

```ini title="/etc/systemd/system/backupx-agent.service"
[Unit]
Description=BackupX Agent
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=10

[Service]
Type=simple
ExecStart=/opt/backupx-agent/backupx agent --config /etc/backupx-agent/config.yaml
Restart=on-failure
RestartSec=10s
TimeoutStopSec=30s
UMask=0077
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

The Agent runs as root because file backup and restore paths may belong to arbitrary system users. Restrict who can create tasks and who can modify the root-owned Agent configuration.

## SSH bastion example

Prefer a SOCKS tunnel when the internal Master uses HTTPS: its hostname and certificate validation remain unchanged.

Create a dedicated SSH account and pre-provision its private key plus a verified `known_hosts` file. Then create:

```sshconfig title="/etc/backupx-agent/ssh_config"
Host backupx-bastion
    HostName bastion.example.com
    User backupx-tunnel
    IdentityFile /etc/backupx-agent/tunnel_ed25519
    IdentitiesOnly yes
    BatchMode yes
    UserKnownHostsFile /etc/backupx-agent/known_hosts
    StrictHostKeyChecking yes
    DynamicForward 127.0.0.1:1080
    ExitOnForwardFailure yes
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

```ini title="/etc/systemd/system/backupx-agent-tunnel.service"
[Unit]
Description=BackupX Agent SSH tunnel
After=network-online.target
Wants=network-online.target
Before=backupx-agent.service

[Service]
Type=simple
ExecStart=/usr/bin/ssh -NT -F /etc/backupx-agent/ssh_config backupx-bastion
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Add a drop-in so the Agent fails closed when the tunnel is unavailable:

```ini title="/etc/systemd/system/backupx-agent.service.d/tunnel.conf"
[Unit]
Requires=backupx-agent-tunnel.service
After=backupx-agent-tunnel.service
```

Reload and start both units:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now backupx-agent-tunnel backupx-agent
```

In the wizard, keep the internal HTTPS Master URL and set the proxy to `socks5h://127.0.0.1:1080`. Verify the bastion host key out-of-band before enabling the service.

## Central storage data paths

| Destination | Data path |
| --- | --- |
| S3, WebDAV, FTP, cloud drive, or another network backend | Agent streams directly to the destination |
| `local_disk` with **Relay remote backups through Master** enabled | Agent streams through the authenticated Master API; Master writes to its local mount |

The relay does not create a second complete temporary copy on the Master. Restore uses the reverse streaming path. Nginx request buffering must be disabled for this behavior to remain streaming.

## Operations

```bash
sudo systemctl status backupx-agent
sudo journalctl -u backupx-agent -n 100 --no-pager
sudo /opt/backupx-agent/backupx agent --config /etc/backupx-agent/config.yaml
```

Rotate a node Token from its action menu. Update `/etc/backupx-agent/agent.token` on the node and restart the service during the 24-hour overlap window.

Monitor these Prometheus metrics:

- `backupx_agent_command_queue_depth`
- `backupx_agent_command_running`
- `backupx_agent_command_timeout_total`
- `backupx_node_online`

## CLI reference

```text
backupx agent --help
  -master string       Master URL
  -token string        Agent authentication token
  -token-file string   Read the Agent Token from a file
  -config string       YAML configuration path
  -temp-dir string     Local temporary directory
  -proxy-url string    HTTP(S) or SOCKS5(H) proxy
  -ca-cert string      PEM CA certificate used to verify the Master
  -insecure-tls        Skip TLS verification (testing only)
```

Environment variables: `BACKUPX_AGENT_MASTER`, `BACKUPX_AGENT_TOKEN`, `BACKUPX_AGENT_TOKEN_FILE`, `BACKUPX_AGENT_HEARTBEAT`, `BACKUPX_AGENT_POLL`, `BACKUPX_AGENT_TEMP_DIR`, `BACKUPX_AGENT_PROXY_URL`, `BACKUPX_AGENT_CA_CERT_FILE`, and `BACKUPX_AGENT_INSECURE_TLS`.

## Known limitations

- The Master is single-active because it uses embedded SQLite.
- Encrypted backups are Master-only because Agents do not hold the Master encryption key.
- Remote directory browsing is a synchronous queue RPC with a 15-second timeout.
- Claimed commands that stop reporting progress are timed out according to the Master command monitor.
