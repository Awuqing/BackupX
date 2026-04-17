---
sidebar_position: 2
title: Bare-metal Deployment
description: systemd + Nginx deployment from the prebuilt release tarball or source.
---

# Bare-metal Deployment

## From prebuilt release

```bash
# Download the matching tarball
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-v1.6.0-linux-amd64.tar.gz

# Extract and install
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh
```

The installer performs these steps automatically:

1. Creates a system user `backupx`
2. Copies the binary to `/opt/backupx/`
3. Generates a default `config.yaml` with safe JWT/encryption secrets
4. Installs `backupx.service` (systemd), enabled at boot
5. (Optional) installs an Nginx site file — see [Nginx Reverse Proxy](./nginx)

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
Description=BackupX backup management service
After=network.target

[Service]
Type=simple
User=backupx
WorkingDirectory=/opt/backupx
ExecStart=/opt/backupx/backupx --config /opt/backupx/config.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Typical operations:

```bash
sudo systemctl status backupx
sudo journalctl -u backupx -f    # live logs
sudo systemctl restart backupx
```

## Password reset

If the admin password is lost:

```bash
/opt/backupx/backupx reset-password \
  --username admin \
  --password 'newpass123' \
  --config /opt/backupx/config.yaml
```

Docker equivalent:

```bash
docker exec -it backupx /app/bin/backupx reset-password --username admin --password 'newpass123'
```
