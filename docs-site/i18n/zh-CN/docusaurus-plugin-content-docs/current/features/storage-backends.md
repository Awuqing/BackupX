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
| **本地磁盘** | 目标目录（绝对路径）+ 可选的远程 Agent 经 Master 中转 |

新建本地磁盘目标默认开启 **远程备份经 Master 中转**。开启时，配置目录属于 Master，挂载到 Master 的存储服务器可集中接收多台源 Agent 的备份；如果该路径本就属于各 Agent，请关闭此选项。升级前已有目标保持原来的 Agent 本机落盘行为，只有显式开启后才会切换。

## 存储容量与备份配额

只有后端返回总容量和已用字节时，才显示真实存储空间使用率。包括 MinIO 在内的 S3 兼容存储不通过标准 S3 API 或 rclone About 能力提供这些数据；未获取到容量不代表容量为零。

此类目标会显示备份记录统计的大小。可在存储目标中设置 **备份配额**，显示备份配额使用率。这是 BackupX 执行的软限额，不是桶配额，也不是 MinIO 集群磁盘总容量；统计不包含其他应用写入的对象或未被记录的对象版本。集群物理容量与桶容量请使用 MinIO 控制台或监控接口查询。

## Rclone 后端

每一种 [rclone 后端](https://rclone.org/overview/) 都作为一等公民暴露 — SFTP、Azure Blob、Dropbox、OneDrive、Backblaze B2、Wasabi、pCloud、HDFS 等。

- 表单字段分为 **必填** 和 **高级**（高级默认折叠）
- 校验与连接测试复用 rclone 自带的探测

## 一个任务多个目标

一个备份任务可以并行上传到多个存储目标。每个目标获得相同的产物，每目标的状态会单独记录：

- 成功：storage_path + 文件大小
- 失败：错误信息

如果任一目标在重试后仍失败，整条记录的状态为 `failed`，但已成功的目标产物会被保留（不回滚）。
