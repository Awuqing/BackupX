# 安全策略 / Security Policy

## 支持范围 / Supported Versions

安全修复面向最新稳定版本和 `main` 分支。旧版本不会单独维护安全补丁；升级前请先阅读对应 Release Notes 和升级恢复文档。

Security fixes target the latest stable release and the `main` branch. Older versions do not receive separate security patches. Review the release notes and upgrade documentation before updating.

| Version | Status |
|---------|--------|
| Latest stable release | Supported |
| `main` | Development support |
| Older releases | Unsupported |

## 报告漏洞 / Reporting a Vulnerability

请勿通过公开 Issue、Discussion 或 Pull Request 披露安全漏洞。使用 GitHub 的[私密漏洞报告入口](https://github.com/Awuqing/BackupX/security/advisories/new)提交报告。

Do not disclose vulnerabilities in a public Issue, Discussion, or Pull Request. Submit the report through GitHub's [private vulnerability reporting form](https://github.com/Awuqing/BackupX/security/advisories/new).

报告应包含：

1. 受影响版本或提交；
2. 漏洞描述、攻击前提和影响范围；
3. 最小复现步骤或验证代码；
4. 已知缓解措施；
5. 希望使用的署名信息。

不要上传真实密钥、Token、备份数据、数据库或包含客户信息的日志。必要时请先脱敏，并使用最小化测试数据。

Do not upload real credentials, tokens, backup data, databases, or logs containing customer information. Redact sensitive values and use minimal test data.

## 处理流程 / Response Process

- 维护者会尽快确认报告并进行初步分级；
- 修复期间请保持细节私密，避免影响仍未升级的部署；
- 修复发布后会在安全公告或 Release Notes 中说明受影响范围、缓解措施和升级版本；
- 披露时间由报告者与维护者协调确定。

The maintainer will acknowledge and triage the report as soon as practical. Details should remain private until a fix and coordinated disclosure are ready.
