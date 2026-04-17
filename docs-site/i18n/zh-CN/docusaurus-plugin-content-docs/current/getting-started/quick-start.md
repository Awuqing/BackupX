---
sidebar_position: 2
title: 快速开始
description: 部署 BackupX、添加存储目标、创建第一个备份任务。
---

# 快速开始

完成 [安装](./installation) 后，花五分钟跑通第一个备份。

## 1. 打开控制台

浏览器访问 `http://your-server:8340`。首次打开会引导创建管理员账户。

## 2. 添加存储目标

进入 **存储目标 → 添加**，选择类型并填写凭证：

| 类型 | 需要填写 |
|------|---------|
| 阿里云 OSS | Region + AccessKey ID/Secret + Bucket |
| 腾讯云 COS | Region + SecretId/SecretKey + Bucket（格式 `name-appid`） |
| 七牛云 Kodo | Region + AccessKey/SecretKey + Bucket |
| S3 兼容 | Endpoint + AccessKey + Bucket |
| Google Drive | Client ID/Secret → 点击「授权」完成 OAuth |
| WebDAV | 服务器地址 + 用户名/密码 |
| FTP | 主机 + 端口 + 用户名/密码 |
| 本地磁盘 | 目标目录路径 |
| SFTP / Azure / Dropbox / OneDrive 等 | 选择对应类型后填写必填项，高级配置默认折叠 |

:::tip
国内云厂商只需填 Region 和 AccessKey，系统自动组装 Endpoint。Rclone 类型的配置项按"必填 / 可选"分层展示，高级选项默认折叠。
:::

添加后点击 **测试连接** 确认配置正确。

## 3. 创建备份任务

进入 **备份任务 → 新建**，三步完成：

1. **基础信息** — 任务名称、备份类型、Cron 表达式（留空则仅手动执行）
2. **源配置** — 文件备份选择源路径（支持多个），数据库备份填写连接信息
3. **存储与策略** — 选择存储目标（支持多个）、压缩策略、保留天数、是否加密

保存后可点击 **立即执行** 测试，**备份记录** 页面实时查看执行日志。

:::note
删除备份任务时会自动清理远端存储上的备份文件，但保留备份记录以供审计追溯。
:::

## 4. 配置通知（可选）

**通知配置** 页面支持邮件、Webhook、Telegram 三种方式，可分别配置成功/失败时是否推送。

## 继续阅读

- 了解 [备份类型](/docs/features/backup-types) 和 [存储后端](/docs/features/storage-backends)
- 使用 SAP HANA？参考 [SAP HANA 支持](/docs/features/sap-hana)
- 管理多台服务器？参考 [多节点集群](/docs/features/multi-node)
