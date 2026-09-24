---
sidebar_position: 4
title: SQL Server VDI
description: Native SQL Server full backups on Linux and Windows through a local VDI worker.
---

# SQL Server VDI

SQL Server tasks create a native `.bak` with `BACKUP DATABASE ... WITH COPY_ONLY, CHECKSUM`. COPY_ONLY preserves an existing differential backup base. Each task handles one database; `tempdb`, differential backups and transaction-log backups are not supported in this implementation.

The Master can run on a different machine. The **execution node and `backupx-sqlvdi` worker must share the SQL Server host and its shared-memory environment**. Bind the task to that specific Agent, or run the Master on the SQL host. A node pool or a remote database hostname is not supported. The worker handles media I/O; BackupX sends SQL through Microsoft's Go driver, without passing credentials to a shell or child-process command line.

## Prerequisites

- SQL Server on Linux x64 or Windows x64, with TCP enabled and a known port.
- A SQL authentication login with the **sysadmin** server role (required by VDI).
- A current BackupX Master and Agent built from this source, plus the platform-specific worker on the execution node's `PATH`.
- Enough local temporary disk space for the uncompressed `.bak` and any subsequent compression. Uploads and retention use the existing BackupX pipeline; remote Agent encryption remains unsupported.

TLS is enabled. Certificate validation is enabled by default. For a local instance with a self-signed certificate, explicitly enable **Trust server certificate** in the task. The connection is restricted to `localhost` or a loopback IP.

## Linux worker

Install SQL Server's supported Linux distribution, its `/opt/mssql/lib/libsqlvdi.so`, a C++17 compiler and CMake 3.20 or newer. From the repository root:

```bash
cmake -S native/sqlvdi -B .build-tmp/sqlvdi -DCMAKE_BUILD_TYPE=Release
cmake --build .build-tmp/sqlvdi
sudo cmake --install .build-tmp/sqlvdi --prefix /usr/local
```

CMake downloads the Microsoft SDK headers from a pinned revision and verifies their SHA-256 hashes. `SQLVDI_SDK_DIR` can point to pre-downloaded copies of the same headers for an offline build. The Microsoft shared library is supplied by SQL Server, not redistributed with BackupX.

Run the Agent as `mssql`, with a writable private temporary directory, or configure the shared-memory group permissions described in the [Microsoft VDI specification](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-backup-vdi-specification). Merely running on the same machine with an unrelated user may fail to open shared memory.

For containers, the usual BackupX Alpine container is **not** a VDI execution environment. Run the Agent and worker alongside SQL Server in its compatible environment; the Master can retain the normal Docker deployment. Separate containers need correctly shared IPC and permissions, not just a shared backup directory. Follow Microsoft's [VDI container requirements](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-docker-container-configure#enable-vdi-backup-and-restore-in-containers) before using this topology. Container deployment is not automatically configured by BackupX.

## Windows worker and Agent

Use the SQL Server x64 VDI COM component installed and registered by SQL Server, Visual Studio's C++ build tools, the Windows SDK, CMake and Go 1.25+. In PowerShell at the repository root:

```powershell
cmake -S native/sqlvdi -B .build-tmp/sqlvdi-windows -A x64
cmake --build .build-tmp/sqlvdi-windows --config Release
$env:CGO_ENABLED = '0'
go -C server build -o ../.build-tmp/backupx.exe ./cmd/backupx
```

Place `backupx.exe` and the worker from `.build-tmp/sqlvdi-windows/Release/backupx-sqlvdi.exe` in the chosen installation directory. Add that directory to the Agent process's `PATH`. Use an OS account that can access the instance's VDI shared objects and the temporary directory.

The existing node installer targets Linux; Windows installation is manual. Create a node in the console and use its token with the existing Agent configuration:

```powershell
$env:PATH = 'C:\BackupX;' + $env:PATH
$env:BACKUPX_AGENT_MASTER = 'https://backup.example.com'
$env:BACKUPX_AGENT_TOKEN = '<node token>'
$env:BACKUPX_AGENT_TEMP_DIR = 'C:\BackupX\tmp'
C:\BackupX\backupx.exe agent
```

For unattended execution, use your existing Windows process supervisor or Task Scheduler. `backupx agent` is a console application, not a Windows Service executable. A named instance such as `SQLEXPRESS` requires **Windows instance name** in the task and its actual TCP port; the port is not discovered through SQL Browser. Leave the instance name empty for the default instance and for Linux.

## Configure and restore

1. Select **SQL Server (VDI)** and the fixed execution node.
2. Enter the loopback host, TCP port, login/password and one database name.
3. Set the optional Windows instance name and certificate option.
4. Select storage targets and the schedule, then run a manual backup first.

Backup success requires both the VDI worker and the SQL command to succeed. Output is flushed before acknowledging `VDC_Complete`; older servers without this command are flushed before worker success. Failed or cancelled backups are not uploaded, and their local temporary directory is removed. Cancellation closes the worker's control pipe, invokes VDI abort/close, and force-stops an unresponsive worker after five seconds.

The existing restore action downloads and decompresses the `.bak`, then streams it through VDI with `RESTORE DATABASE ... WITH CHECKSUM`. It intentionally does not issue `WITH REPLACE`, force-disconnect database clients or rewrite file locations. SQL Server may reject restore because the database is in use, its files conflict or its overwrite protections apply; resolve those conditions explicitly. Restoring under another name or with `MOVE` currently requires SQL Server's own tools.

Automatic verification drills are unavailable for this task type. A successful transfer or checksum is **not a completed restore drill**. Before relying on backups, restore a disposable database on each target OS and check its data (and `DBCC CHECKDB` where appropriate). Windows cross-compilation and mocked VDI tests do not establish real COM/shared-memory or database compatibility.

## Development checks

The native protocol check links the production worker with a fake VDI library; it does not need SQL Server. After downloading the pinned Linux headers into `.build-tmp/sdk/linux`:

```bash
c++ -std=c++17 -pthread -I .build-tmp/sdk/linux native/sqlvdi/main.cpp \
  native/sqlvdi/tests/mock_vdi.cpp -o .build-tmp/sqlvdi-worker-test
TMPDIR="$PWD/.build-tmp" python3 native/sqlvdi/tests/check_worker.py .build-tmp/sqlvdi-worker-test
```

Sources: [Microsoft VDI reference](https://learn.microsoft.com/en-us/sql/relational-databases/backup-restore/vdi-reference/reference-virtual-device-interface), [Linux SDK](https://github.com/microsoft/sql-server-samples/tree/master/samples/features/sqlvdi-linux), [Windows SDK](https://github.com/microsoft/sql-server-samples/tree/master/samples/features/sqlvdi).

### Real SQL Server acceptance test

Run only on a dedicated test instance. This creates a unique `backupx_vdi_it_*` database, seeds it, backs it up, drops that test database, restores it, checks data and runs `DBCC CHECKDB`, then removes the test database. It does not use an existing application database. The real native worker must be on `PATH`.

```bash
export BACKUPX_SQLSERVER_INTEGRATION=1
export BACKUPX_SQLSERVER_TEST_USER=backup_test
# Set BACKUPX_SQLSERVER_TEST_PASSWORD securely in the current process environment.
# Optional: BACKUPX_SQLSERVER_TEST_PORT, BACKUPX_SQLSERVER_TEST_INSTANCE,
# BACKUPX_SQLSERVER_TEST_TRUST_CERTIFICATE=1 for a self-signed local certificate.
TMPDIR="$PWD/.build-tmp" go -C server test ./internal/backup -run '^TestSQLServerVDIRoundTrip$' -v -count=1
```

On Windows, set the same environment variables with PowerShell `$env:NAME` and run the same `go test` command. This test is skipped by default; compilation and mocked tests do not replace running it on each real OS.
