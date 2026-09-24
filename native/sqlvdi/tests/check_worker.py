"""Run against main.cpp linked with mock_vdi.cpp (not a SQL Server acceptance test)."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

worker = str(Path(sys.argv[1]).resolve())
for scenario in ("success", "legacy", "restore", "abort", "unknown_command", "cancel", "create_fail"):
    with tempfile.TemporaryDirectory(prefix="sqlvdi-test-") as folder:
        path = Path(folder) / "数据库备份.bak"
        log = Path(folder) / "protocol.log"
        if scenario == "restore":
            path.write_bytes(b"backup")
        env = dict(os.environ, BACKUPX_TEST_VDI_CASE=scenario, BACKUPX_TEST_VDI_LOG=str(log))
        p = subprocess.Popen([worker, "restore" if scenario == "restore" else "backup", "BackupX-test", str(path), ""],
                             stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
        try:
            if scenario != "create_fail":
                assert p.stdout.readline() == b"BACKUPX_SQLVDI_READY\n"
            if scenario == "cancel":
                p.stdin.close()
            status = p.wait(timeout=5)
            error = p.stderr.read().decode()
            protocol = log.read_text() if log.exists() else ""
            if scenario in ("success", "legacy", "restore"):
                assert status == 0, (scenario, error)
                assert path.read_bytes() == b"backup"
                assert "close" in protocol and "abort" not in protocol
                assert ("complete" in protocol) == (scenario != "legacy")
            else:
                assert status != 0, scenario
                if scenario != "create_fail":
                    assert "abort\nclose" in protocol, (scenario, protocol)
            print(f"PASS {scenario}")
        finally:
            if p.poll() is None:
                p.kill()
                p.wait()
            for stream in (p.stdin, p.stdout, p.stderr):
                stream.close()
