---
sidebar_position: 5
title: Notifications
description: Email, webhook, and Telegram notifications on backup success or failure.
---

# Notifications

BackupX supports three notification channels. Configure per-channel rules for success-only, failure-only, or both.

## Email (SMTP)

| Field | Notes |
|-------|-------|
| SMTP host / port | e.g. `smtp.gmail.com:587` |
| Username / password | App-specific password recommended |
| From address | Used in `From:` header |
| Recipients | Comma-separated list |
| Use TLS / StartTLS | Match your SMTP provider |

## Webhook

Send a JSON POST to an arbitrary URL. Body shape:

```json
{
  "event": "backup_result",
  "task": {"id": 1, "name": "web-files", "type": "file"},
  "record": {"id": 42, "status": "success", "fileSize": 1048576, "durationSeconds": 12},
  "error": ""
}
```

Useful for custom workflows: Slack incoming webhook, PagerDuty, your own API, etc.

## Telegram

| Field | Notes |
|-------|-------|
| Bot token | From [@BotFather](https://t.me/BotFather) |
| Chat ID | Numeric — obtain via `/start` + bot's `getUpdates` |

## Event rules

Each notification configuration can be scoped to:

- **Success only** — quiet during normal runs, pings on first failure
- **Failure only** — recommended for loud channels
- **Both** — useful during initial setup to verify notifications flow
