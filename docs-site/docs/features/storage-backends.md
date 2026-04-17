---
sidebar_position: 2
title: Storage Backends
description: 70+ storage backends — built-in cloud providers plus any rclone backend.
---

# Storage Backends

BackupX aims to accept any place you'd want to drop a backup file.

## Built-in providers

| Type | Required fields |
|------|-----------------|
| **Alibaba OSS** | Region + AccessKey ID/Secret + Bucket (endpoint auto-assembled) |
| **Tencent COS** | Region + SecretId/SecretKey + Bucket (format `name-appid`) |
| **Qiniu Kodo** | Region + AccessKey/SecretKey + Bucket |
| **S3-compatible** | Endpoint + AccessKey + Bucket |
| **Google Drive** | Client ID/Secret + OAuth authorization |
| **WebDAV** | URL + username/password |
| **FTP / FTPS** | Host + port + username/password |
| **Local disk** | Target directory (absolute path) |

## Rclone backends

Every [rclone backend](https://rclone.org/overview/) is exposed as a first-class storage type — SFTP, Azure Blob, Dropbox, OneDrive, Backblaze B2, Wasabi, pCloud, HDFS, and many more.

- The form groups fields into **required** and **advanced** (advanced collapsed by default)
- Validation and connection tests reuse rclone's built-in probe

## Multiple targets per task

A backup task can fan out to multiple targets in parallel. All targets receive the same artifact; a per-target status is recorded:

- Success: storage path + size
- Failed: error message

If any target fails after retries, the record status is `failed` but successful targets are preserved (no rollback).
