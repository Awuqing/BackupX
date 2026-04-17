---
sidebar_position: 5
title: 通知
description: 备份成功或失败时通过邮件、Webhook、Telegram 推送通知。
---

# 通知

BackupX 支持三种通知渠道，可为每个渠道单独配置成功/失败事件是否推送。

## 邮件（SMTP）

| 字段 | 说明 |
|------|------|
| SMTP 主机 / 端口 | 如 `smtp.gmail.com:587` |
| 用户名 / 密码 | 建议使用专用应用密码 |
| 发件人地址 | 邮件 `From:` 头 |
| 收件人列表 | 英文逗号分隔 |
| 使用 TLS / StartTLS | 按 SMTP 提供方要求选择 |

## Webhook

向任意 URL 发送 JSON POST，请求体结构：

```json
{
  "event": "backup_result",
  "task": {"id": 1, "name": "web-files", "type": "file"},
  "record": {"id": 42, "status": "success", "fileSize": 1048576, "durationSeconds": 12},
  "error": ""
}
```

适合自定义场景：Slack incoming webhook、PagerDuty、自建 API 等。

## Telegram

| 字段 | 说明 |
|------|------|
| Bot Token | 在 [@BotFather](https://t.me/BotFather) 创建 |
| Chat ID | 数字型，可通过 `/start` 后调 Bot 的 `getUpdates` 获取 |

## 事件规则

每个通知配置可以指定触发范围：

- **仅成功** — 正常运行时静默
- **仅失败** — 适合高噪敏感通道
- **全部** — 初始化配置时用于验证链路
