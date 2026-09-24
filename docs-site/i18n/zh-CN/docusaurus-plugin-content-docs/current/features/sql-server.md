---
sidebar_position: 4
title: SQL Server VDI
description: 在 Linux 和 Windows SQL Server 主机通过 VDI 执行原生完整备份。
---

# SQL Server VDI

SQL Server 类型生成原生 `.bak`，执行 `BACKUP DATABASE ... WITH COPY_ONLY, CHECKSUM`。COPY_ONLY 不改变现有差异备份基线。每个任务只处理一个数据库；本次不支持 `tempdb`、差异备份和事务日志备份。

Master 可以部署在其他机器。**执行节点和 `backupx-sqlvdi` 必须位于 SQL Server 所在主机，并能访问同一共享内存环境**。任务必须选择该固定 Agent，或者在 SQL Server 主机运行 Master；不支持节点池和远程数据库地址。原生组件仅处理 VDI 介质，Go 进程使用微软驱动发送 SQL，数据库密码不会传入 shell 或子进程参数。

## 前置条件

- Linux x64 或 Windows x64 SQL Server，启用 TCP 并确认实际端口。
- SQL 登录账号具有 **sysadmin** 服务器角色，这是 VDI 要求。
- Master、Agent 均使用包含此功能的版本；对应平台的原生组件位于执行节点进程的 `PATH`。
- 临时目录有足够空间容纳未压缩 `.bak` 及后续压缩产物。上传、保留策略沿用现有链路，远程 Agent 仍不支持 BackupX 加密。

默认启用 TLS 并校验服务器证书。若本机 SQL Server 使用自签名证书，需显式打开任务中的“信任服务器证书”。主机只允许 `localhost` 或回环 IP。

## Linux 部署

准备 SQL Server 支持的 Linux 发行版、SQL Server 自带的 `/opt/mssql/lib/libsqlvdi.so`、C++17 编译器和 CMake 3.20+。在项目根目录执行：

```bash
cmake -S native/sqlvdi -B .build-tmp/sqlvdi -DCMAKE_BUILD_TYPE=Release
cmake --build .build-tmp/sqlvdi
sudo cmake --install .build-tmp/sqlvdi --prefix /usr/local
```

构建时从固定微软仓库版本下载 SDK 头文件，并验证 SHA-256；离线构建可以用 `SQLVDI_SDK_DIR` 指向相同版本的头文件。微软动态库由 SQL Server 安装提供，不随 BackupX 分发。

推荐 Agent 使用 `mssql` 身份运行，并使用该账号可写的独立临时目录；也可以按[微软规范](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-backup-vdi-specification)设置双向用户组权限。仅在同机运行但权限不匹配，仍会导致 VDI 失败。

默认 BackupX Alpine 镜像不是 VDI 执行环境。Master 可继续使用原有 Docker 部署，Agent 和原生组件应运行于 SQL Server 的兼容环境中。跨容器需要共享 IPC 和正确权限，仅共享备份目录不够；请先核对[微软容器 VDI 要求](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-docker-container-configure#enable-vdi-backup-and-restore-in-containers)。BackupX 不会自动修改容器或 SQL Server 配置。

## Windows 部署

需要 SQL Server 安装并注册的 x64 VDI COM 组件、Visual Studio C++ 构建工具、Windows SDK、CMake 和 Go 1.25+。在项目根目录的 PowerShell 中执行：

```powershell
cmake -S native/sqlvdi -B .build-tmp/sqlvdi-windows -A x64
cmake --build .build-tmp/sqlvdi-windows --config Release
$env:CGO_ENABLED = '0'
go -C server build -o ../.build-tmp/backupx.exe ./cmd/backupx
```

将 `backupx.exe` 和 `.build-tmp/sqlvdi-windows/Release/backupx-sqlvdi.exe` 放入安装目录，并将目录加入 Agent 进程的 `PATH`。运行身份必须能访问实例的 VDI 共享对象和临时目录。

当前一键节点安装器面向 Linux；Windows 使用手动部署。在控制台创建节点，使用其令牌启动：

```powershell
$env:PATH = 'C:\BackupX;' + $env:PATH
$env:BACKUPX_AGENT_MASTER = 'https://backup.example.com'
$env:BACKUPX_AGENT_TOKEN = '<节点令牌>'
$env:BACKUPX_AGENT_TEMP_DIR = 'C:\BackupX\tmp'
C:\BackupX\backupx.exe agent
```

长期运行可交给已有进程托管器或 Windows 任务计划程序。`backupx agent` 是控制台程序，不能直接作为 Windows Service 注册。对于 `SQLEXPRESS` 等命名实例，任务填写“Windows 实例名称”和该实例实际 TCP 端口；不通过 SQL Browser 自动发现端口。默认实例和 Linux 留空实例名称。

## 配置、恢复与验证

1. 选择“SQL Server (VDI)”和数据库所在固定节点。
2. 填写回环地址、TCP 端口、账号密码、一个数据库名称。
3. 按需填写 Windows 实例名和证书选项。
4. 配置存储和计划，先手动执行一次备份。

只有原生组件和 SQL 命令都成功，才会上报备份成功。支持 `VDC_Complete` 时在回应前完成落盘；旧版本在原生组件退出成功前完成落盘。失败或取消不会上传半成品，并清理任务临时目录。取消通过控制管道 EOF 触发 VDI 中止和关闭；原生库无响应时五秒后强制退出。

现有恢复入口下载、解压备份后，通过 VDI 执行 `RESTORE DATABASE ... WITH CHECKSUM`。不会自动使用 `WITH REPLACE`、强制踢掉数据库连接或更改数据库文件位置。数据库被占用、文件冲突或触发 SQL Server 覆盖保护时会失败，需要管理员明确处理。恢复到另一个数据库名或使用 `MOVE` 目前请使用 SQL Server 工具。

此类型暂不支持自动验证演练。传输成功、校验和通过不等于恢复验收完成；上线前应在每种目标系统上备份并恢复一个可丢弃数据库，核对数据，必要时执行 `DBCC CHECKDB`。Windows 交叉编译及模拟 VDI 测试不代表真实 COM、共享内存和数据库已通过兼容性验证。

## 开发验证

原生协议检查将真实组件源码与模拟 VDI 库链接，不需要 SQL Server。准备固定版本的 Linux SDK 头文件到 `.build-tmp/sdk/linux` 后执行：

```bash
c++ -std=c++17 -pthread -I .build-tmp/sdk/linux native/sqlvdi/main.cpp \
  native/sqlvdi/tests/mock_vdi.cpp -o .build-tmp/sqlvdi-worker-test
TMPDIR="$PWD/.build-tmp" python3 native/sqlvdi/tests/check_worker.py .build-tmp/sqlvdi-worker-test
```

参考：[微软 VDI 规范](https://learn.microsoft.com/en-us/sql/relational-databases/backup-restore/vdi-reference/reference-virtual-device-interface)、[Linux SDK](https://github.com/microsoft/sql-server-samples/tree/master/samples/features/sqlvdi-linux)、[Windows SDK](https://github.com/microsoft/sql-server-samples/tree/master/samples/features/sqlvdi)。

### 真实 SQL Server 验收

仅在专用测试实例运行。此测试创建唯一的 `backupx_vdi_it_*` 数据库，写入数据、备份、删除该测试库、恢复、核对数据并执行 `DBCC CHECKDB`，最后删除测试库；不会使用已有业务库。原生组件必须位于 `PATH`。

```bash
export BACKUPX_SQLSERVER_INTEGRATION=1
export BACKUPX_SQLSERVER_TEST_USER=backup_test
# Set BACKUPX_SQLSERVER_TEST_PASSWORD securely in the current process environment.
# Optional: BACKUPX_SQLSERVER_TEST_PORT, BACKUPX_SQLSERVER_TEST_INSTANCE,
# BACKUPX_SQLSERVER_TEST_TRUST_CERTIFICATE=1 for a self-signed local certificate.
TMPDIR="$PWD/.build-tmp" go -C server test ./internal/backup -run '^TestSQLServerVDIRoundTrip$' -v -count=1
```

Windows 请在 PowerShell 用 `$env:变量名` 设置相同环境变量，再运行同一个 `go test` 命令。默认未启用此测试；编译和模拟测试通过不能代替两种真实系统上的验收。
