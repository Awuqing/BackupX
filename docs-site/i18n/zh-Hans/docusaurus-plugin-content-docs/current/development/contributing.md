---
sidebar_position: 2
title: 贡献指南
description: 如何反馈问题、提出改进、提交 PR。
---

# 贡献指南

BackupX 使用 Apache License 2.0 开源，欢迎提交 Issue 与 Pull Request。

## 报告 Bug

在 [github.com/Awuqing/BackupX/issues](https://github.com/Awuqing/BackupX/issues) 提交 Issue，请附上：

- BackupX 版本（`backupx --version`）
- 部署方式（Docker / 裸机 / 源码）
- 相关的备份任务类型和存储后端
- 复现步骤
- 问题发生时段的 stdout / `backupx.log` 片段

## 提议改动

对于重要功能或重构，建议先开 Issue 对齐方案，避免 PR 大改动后被 Review 回退。

## 提交 PR

1. Fork 仓库，创建主题分支（如 `fix/windows-path-escape`）
2. 执行 `make test` 确认本地全通过
3. 保持每个 PR 只做一件事
4. Commit message 使用中文，格式 `类型: 简要描述`：
   - `功能: 新增审计日志模块`
   - `修复: 目录浏览器无法进入子目录`
   - `重构: 简化存储目标解密逻辑`
   - 类型：`功能` / `修复` / `重构` / `文档` / `构建` / `测试`
5. PR 标题和正文同样使用中文，描述"为什么"和"怎么做"，而非仅仅"做了什么"

## 代码规范

- **Go** — 所有错误必须处理（禁止 `_ = err`），日志使用现有 `zap`，禁止生产路径中出现 `fmt.Println`
- **TypeScript** — 严格模式，禁止隐式 any，遵循现有 ESLint/Prettier 配置
- **Commit 粒度** — 每个 commit 一件事，不要把顺手的小修改和功能代码混在一起
