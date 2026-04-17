---
sidebar_position: 2
title: 存储后端
description: 70+ 存储后端 — 内置云服务商 + 任意 rclone 后端。
---

# 存储后端

BackupX 的目标是接入任何你想放置备份文件的地方。

## 内置后端

| 类型 | 必填字段 |
|------|---------|
| **阿里云 OSS** | Region + AccessKey ID/Secret + Bucket（endpoint 自动组装） |
| **腾讯云 COS** | Region + SecretId/SecretKey + Bucket（格式 `name-appid`） |
| **七牛云 Kodo** | Region + AccessKey/SecretKey + Bucket |
| **S3 兼容** | Endpoint + AccessKey + Bucket |
| **Google Drive** | Client ID/Secret + OAuth 授权 |
| **WebDAV** | 地址 + 用户名/密码 |
| **FTP / FTPS** | 主机 + 端口 + 用户名/密码 |
| **本地磁盘** | 目标目录（绝对路径） |

## Rclone 后端

每一种 [rclone 后端](https://rclone.org/overview/) 都作为一等公民暴露 — SFTP、Azure Blob、Dropbox、OneDrive、Backblaze B2、Wasabi、pCloud、HDFS 等。

- 表单字段分为 **必填** 和 **高级**（高级默认折叠）
- 校验与连接测试复用 rclone 自带的探测

## 一个任务多个目标

一个备份任务可以并行上传到多个存储目标。每个目标获得相同的产物，每目标的状态会单独记录：

- 成功：storage_path + 文件大小
- 失败：错误信息

如果任一目标在重试后仍失败，整条记录的状态为 `failed`，但已成功的目标产物会被保留（不回滚）。
