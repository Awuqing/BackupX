---
sidebar_position: 2
title: Bare-metal Deployment
description: Hardened systemd deployment from the prebuilt release tarball or source, with opt-in Nginx.
---

# Bare-metal Deployment

## From prebuilt release

```bash
# Download the matching tarball
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz.sha256
sha256sum -c backupx-linux-amd64.tar.gz.sha256

# Extract and install
tar xzf backupx-linux-amd64.tar.gz && cd backupx-*-linux-amd64
sudo ./install.sh
```

The installer performs these steps automatically:

1. Creates a system user `backupx`
2. Copies the binary to `/opt/backupx/bin/backupx` and the web console to `/opt/backupx/web`
3. Installs the default configuration at `/etc/backupx/config.yaml`
4. Installs `backupx.service` (systemd), enabled at boot
5. Leaves Nginx unchanged unless `INSTALL_NGINX=1` is explicitly requested
6. Verifies the first-setup API before reporting success

The executable and web assets are owned by root; only `/opt/backupx/data` is writable by the `backupx` service account. `/etc/backupx/config.yaml` is installed as `root:backupx` with mode `0640`.

The bundled Nginx template is a starting point and may conflict with an existing default server. Review its hostname and TLS policy first, then opt in:

```bash
sudo INSTALL_NGINX=1 ./install.sh
```

For multi-node clusters, edit `/etc/backupx/config.yaml` after installation and set the Master URL that remote Agents can reach:

```yaml
server:
  external_url: "https://backup.example.com"
```

Restart BackupX after changing it:

```bash
sudo systemctl restart backupx
```

## From source

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
sudo ./deploy/install.sh
```

`make build` compiles:

- `server/bin/backupx` (Go backend, no CGO)
- `web/dist/` (React frontend, `npm run build`)

## systemd

The installed unit:

```ini title="/etc/systemd/system/backupx.service"
[Unit]
Description=BackupX API Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=backupx
Group=backupx
WorkingDirectory=/opt/backupx
ExecStart=/opt/backupx/bin/backupx -config /etc/backupx/config.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
UMask=0027
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Typical operations:

```bash
sudo systemctl status backupx
sudo journalctl -u backupx -f    # live logs
sudo systemctl restart backupx
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
```

Open `http://your-server:8340`, switch to English if desired, and create the first administrator on the **System setup** screen. For a custom listen port, run the installer with a matching `HEALTH_URL`.

For production, expose BackupX through HTTPS or restrict port `8340` at the firewall. The installer does not make firewall changes.

Before replacing a release, snapshot `/etc/backupx`, `/opt/backupx/data`, the installed binary, and web assets while the service is stopped. Follow the versioned procedure in [Upgrade and Recovery](../operations/upgrade-recovery); running an older binary against a database already migrated by a newer release is not a safe rollback.

## Password reset

If the admin password is lost:

```bash
/opt/backupx/bin/backupx reset-password \
  --username admin \
  --password 'newpass123' \
  --config /etc/backupx/config.yaml
```

Docker equivalent:

```bash
docker exec -it backupx /app/bin/backupx reset-password --username admin --password 'newpass123'
```
