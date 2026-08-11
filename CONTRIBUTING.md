# Contributing to BackupX

感谢你对 BackupX 的关注！本指南介绍如何搭建开发环境并提交贡献。
Thanks for your interest in contributing to BackupX! This guide covers how to set up your environment and submit changes.

## 开发环境 / Development Setup

### 依赖 / Prerequisites

- **Go** 1.25+（见 `server/go.mod`）
- **Node.js** 24 LTS（见 `.node-version`，CI 与 Docker 使用同一主版本）
- **npm** 11+

### 快速开始 / Quick Start

分别在两个终端启动前后端（后端 :8340，前端 Vite HMR）：

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
npm --prefix web ci
npm --prefix docs-site ci

# 终端 1 —— 后端（默认 http://localhost:8340）
make dev-server

# 终端 2 —— 前端（Vite 热更新，/api 代理到 8340）
make dev-web
```

### 构建 / Building

```bash
make build          # 同时构建后端与前端
make build-server   # 仅后端 → server/bin/backupx
make build-web      # 仅前端 → web/dist
make build-docs     # 仅文档站 → docs-site/build
make docker         # 构建 Docker 镜像
make docker-cn      # 国内镜像源加速构建
```

> 后端会自动托管 `web/dist`（或 `server.web_root` 指定目录），因此本地裸机部署无需额外的反向代理即可访问控制台。

## 测试 / Testing

提交前应执行与 CI 一致的完整验证：

```bash
make verify         # 格式、依赖、静态检查、测试与三端构建
make test           # 仅后端 + 前端测试
make test-server    # 仅后端：cd server && go test ./...
make test-web       # 仅前端：cd web && npm run test（vitest）
make check-docs     # 文档类型检查 + 中英文站点构建
```

新增功能或修复缺陷时，请补充对应测试。文档变更也必须通过严格断链检查，不能只依赖合并后的 Pages 部署结果。

## 提交信息规范 / Commit Messages

本项目采用 **Conventional Commits**，正文用中文撰写：

```
<type>(<scope>): <subject>

<body>
```

| type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `docs` | 文档变更 |
| `style` | 不影响逻辑的格式调整 |
| `refactor` | 重构 |
| `perf` | 性能优化 |
| `test` | 测试相关 |
| `chore` | 构建/依赖/工具链 |

示例：

```
feat(storage): 新增 Wasabi S3 后端支持
fix(cluster): 修复跨节点恢复的终态处理
docs: 补充 CONTRIBUTING 指南
```

## Pull Request 流程

1. **Fork** 仓库并从最新的 `main` 切出特性分支；
2. **开发**功能或修复，必要时补充测试；
3. **自测**：确保 `make verify` 通过；
4. **提交**：使用上述 Conventional Commits（中文）；
5. **推送**并对着 `main` 发起 PR。

### PR 描述建议

- 清晰说明本 PR 做了什么；
- 对新功能/修复，补充动机与背景；
- 关联相关 Issue（如 `Closes #62`）；
- 纯文档 PR 至少应附上 `make check-docs` 的结果。

> 请保持分支基于较新的 `main`：基线过旧的分支容易产生大范围冲突，难以评审与合入。

## 编码规范 / Coding Conventions

- **Go**：遵循现有 handler → service → repository 分层；避免把连续业务流程拆成大量无复用价值的独立函数。所有错误必须处理，日志使用 `zap`，禁止 `fmt.Println`，提交前执行 `gofmt` 与 `go vet`。
- **前端组件**：复用并组合现有组件，不在页面内复制同类交互逻辑，不擅自改变共享组件的基础样式，不引入新的 CSS 框架或 UI 库。
- **前端视觉**：不使用 Emoji、渐变和 `box-shadow`；图标统一使用项目 SVG 图标组件；圆角为 0–4px；不引入独立字体，不用加粗字体制造层级，减少 Card，不使用数据左侧装饰竖线。
- **包管理**：`web/` 与 `docs-site/` 均使用 npm；依赖变更必须同步提交对应的 `package-lock.json`。
- **文档**：英文源文件与 `zh-CN` 翻译应同步维护；新增页面必须加入侧边栏并通过两种语言的生产构建。

## License

向 BackupX 贡献即表示你同意你的贡献以 [Apache License 2.0](LICENSE) 授权。
