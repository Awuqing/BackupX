package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"
)

// SQLServerOptions 同时用于 Master 和 Agent，避免节点执行时丢失实例/TLS 配置。
type SQLServerOptions struct {
	InstanceName           string `json:"instanceName"`
	TrustServerCertificate bool   `json:"trustServerCertificate"`
}

// ValidateSQLServerDatabase 限定为执行节点上的一个数据库；VDI 不是远程备份协议。
func ValidateSQLServerDatabase(db DatabaseSpec) (SQLServerOptions, error) {
	var options SQLServerOptions
	if db.ExtraConfig != "" {
		if err := json.Unmarshal([]byte(db.ExtraConfig), &options); err != nil {
			return options, fmt.Errorf("SQL Server 扩展配置不合法: %w", err)
		}
	}
	host := strings.TrimSpace(db.Host)
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return options, fmt.Errorf("SQL Server VDI 必须连接执行节点本机，请使用 localhost 或回环 IP，并将任务绑定到数据库所在节点")
	}
	if db.Port < 1 || db.Port > 65535 || strings.TrimSpace(db.User) == "" {
		return options, fmt.Errorf("SQL Server 用户名和有效 TCP 端口必填")
	}
	if len(db.Names) != 1 || strings.TrimSpace(db.Names[0]) == "" || strings.ContainsAny(db.Names[0], ",\r\n\x00") || len(utf16.Encode([]rune(db.Names[0]))) > 128 {
		return options, fmt.Errorf("SQL Server 每个任务必须指定一个数据库（不支持逗号、换行或空名称，最长 128 个 UTF-16 单元）")
	}
	if strings.EqualFold(db.Names[0], "tempdb") {
		return options, fmt.Errorf("SQL Server 不支持备份 tempdb")
	}
	if len(options.InstanceName) > 16 || strings.ContainsAny(options.InstanceName, "\\/;\r\n\x00") {
		return options, fmt.Errorf("SQL Server 实例名不合法，请仅填写实例名称")
	}
	return options, nil
}

type SQLServerRunner struct {
	helperPath string
	execSQL    func(context.Context, DatabaseSpec, SQLServerOptions, string, string) error
}

func NewSQLServerRunner() *SQLServerRunner {
	return &SQLServerRunner{helperPath: "backupx-sqlvdi", execSQL: executeSQLServer}
}

func (r *SQLServerRunner) Type() string { return "sqlserver" }

func (r *SQLServerRunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (result *RunResult, err error) {
	if _, err = ValidateSQLServerDatabase(task.Database); err != nil {
		return nil, err
	}
	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, "bak")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(tempDir))
		}
	}()
	writer.WriteLine("开始 SQL Server VDI COPY_ONLY 完整备份")
	if err = r.transfer(ctx, task.Database, "backup", artifactPath); err != nil {
		return nil, err
	}
	info, err := os.Stat(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("读取 SQL Server 备份文件: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("SQL Server VDI 未生成有效备份文件")
	}
	startedAt := task.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	writer.WriteLine("SQL Server 备份传输和 SQL 命令均已完成")
	return &RunResult{ArtifactPath: artifactPath, FileName: filepath.Base(artifactPath), TempDir: tempDir, Size: info.Size(), StorageKey: BuildStorageKey(r.Type(), startedAt, filepath.Base(artifactPath))}, nil
}

func (r *SQLServerRunner) Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	info, err := os.Stat(artifactPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("SQL Server 恢复文件不存在、为空或不是普通文件: %v", err)
	}
	writer.WriteLine("开始 SQL Server VDI 恢复（保留 SQL Server 的覆盖保护，不使用 REPLACE）")
	if err := r.transfer(ctx, task.Database, "restore", artifactPath); err != nil {
		return err
	}
	writer.WriteLine("SQL Server 恢复完成")
	return nil
}

func sqlServerQuery(mode, database string) string {
	name := "[" + strings.ReplaceAll(database, "]", "]]") + "]"
	if mode == "backup" {
		return "BACKUP DATABASE " + name + " TO VIRTUAL_DEVICE = @device WITH COPY_ONLY, CHECKSUM"
	}
	return "RESTORE DATABASE " + name + " FROM VIRTUAL_DEVICE = @device WITH CHECKSUM"
}

func executeSQLServer(ctx context.Context, spec DatabaseSpec, options SQLServerOptions, query, device string) (err error) {
	// VDI 数据不经过 SQL 连接。大备份期间 SQL 连接可能长时间没有响应，不能设置读超时。
	values := url.Values{"database": {"master"}, "encrypt": {"true"}, "TrustServerCertificate": {strconv.FormatBool(options.TrustServerCertificate)}, "dial timeout": {"15"}, "app name": {"BackupX VDI"}, "disableretry": {"true"}}
	dsn := url.URL{Scheme: "sqlserver", User: url.UserPassword(spec.User, spec.Password), Host: net.JoinHostPort(strings.TrimSpace(spec.Host), strconv.Itoa(spec.Port)), RawQuery: values.Encode()}
	connector, err := mssql.NewConnector(dsn.String())
	if err != nil {
		return fmt.Errorf("连接 SQL Server 失败: %w", err)
	}
	connector.Dialer = sqlServerDialer{ctx: ctx}
	db := sql.OpenDB(connector)
	defer func() { err = errors.Join(err, db.Close()) }()
	_, err = db.ExecContext(ctx, query, sql.Named("device", device))
	return err
}

// 驱动取消时会等待 SQL Server 的 ATTENTION 回应；关闭本次操作的连接，避免失联时永远等待。
type sqlServerDialer struct{ ctx context.Context }

func (d sqlServerDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	wrapped := &sqlServerConn{Conn: conn}
	wrapped.stop = context.AfterFunc(d.ctx, wrapped.closeSocket)
	return wrapped, nil
}

type sqlServerConn struct {
	net.Conn
	stop      func() bool
	closeOnce sync.Once
	closeErr  error
}

func (c *sqlServerConn) closeSocket() {
	c.closeOnce.Do(func() { c.closeErr = c.Conn.Close() })
}

func (c *sqlServerConn) Close() error {
	c.stop()
	c.closeSocket()
	if errors.Is(c.closeErr, net.ErrClosed) {
		return nil
	}
	return c.closeErr
}

// transfer 等待两个独立结果：VDI 文件传输完成，以及 SQL Server BACKUP/RESTORE 成功。
// stdin 关闭通知原生组件 SignalAbort；WaitDelay 为无响应的原生库提供强制退出上限。
func (r *SQLServerRunner) transfer(ctx context.Context, db DatabaseSpec, mode, artifactPath string) (returnErr error) {
	options, err := ValidateSQLServerDatabase(db)
	if err != nil {
		return err
	}
	helper, err := exec.LookPath(r.helperPath)
	if err != nil {
		return fmt.Errorf("未找到 backupx-sqlvdi；请在 SQL Server 所在节点安装对应平台的 VDI 组件: %w", err)
	}
	device := "BackupX-" + uuid.NewString()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, helper, mode, device, artifactPath, options.InstanceName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建 VDI 控制管道: %w", err)
	}
	var closeOnce sync.Once
	closeControl := func() error {
		var closeErr error
		closeOnce.Do(func() { closeErr = stdin.Close() })
		// exec.Cmd.Wait also closes StdinPipe after a normal worker exit.
		if errors.Is(closeErr, os.ErrClosed) {
			return nil
		}
		return closeErr
	}
	defer func() { returnErr = errors.Join(returnErr, closeControl()) }()
	cmd.Cancel = closeControl
	cmd.WaitDelay = 5 * time.Second
	ready := &vdiReadyWriter{ready: make(chan error, 1)}
	stderr := &vdiErrorWriter{}
	cmd.Stdout, cmd.Stderr = ready, stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 VDI 组件: %w", err)
	}
	helperDone := make(chan error, 1)
	go func() { helperDone <- cmd.Wait() }()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case err = <-ready.ready:
	case err = <-helperDone:
		return fmt.Errorf("VDI 组件未就绪即退出: %v: %s", err, stderr.String())
	case <-timer.C:
		err = fmt.Errorf("VDI 组件初始化超时")
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		cancel()
		return errors.Join(err, <-helperDone)
	}
	sqlDone := make(chan error, 1)
	go func() { sqlDone <- r.execSQL(runCtx, db, options, sqlServerQuery(mode, db.Names[0]), device) }()
	var transferErr, sqlErr error
	for helperDone != nil || sqlDone != nil {
		select {
		case transferErr = <-helperDone:
			helperDone = nil
			if transferErr != nil {
				cancel()
			}
		case sqlErr = <-sqlDone:
			sqlDone = nil
			if sqlErr != nil {
				cancel()
			}
		}
	}
	if err := errors.Join(ctx.Err(), transferErr, sqlErr); err != nil {
		message := fmt.Sprintf("SQL Server VDI %s 失败: %v: %s", mode, err, stderr.String())
		if db.Password != "" {
			message = strings.ReplaceAll(message, db.Password, "********")
		}
		return errors.Join(ctx.Err(), fmt.Errorf("%s", message))
	}
	return nil
}

type vdiReadyWriter struct {
	buffer bytes.Buffer
	ready  chan error
	done   bool
}

func (w *vdiReadyWriter) Write(p []byte) (int, error) {
	if !w.done {
		if w.buffer.Len()+len(p) > 128 {
			w.ready <- fmt.Errorf("VDI 组件就绪协议不合法")
			w.done = true
		} else {
			w.buffer.Write(p)
			if bytes.Contains(p, []byte("\n")) {
				var err error
				if w.buffer.String() != "BACKUPX_SQLVDI_READY\n" {
					err = fmt.Errorf("VDI 组件版本不兼容")
				}
				w.ready <- err
				w.done = true
			}
		}
	}
	return len(p), nil
}

type vdiErrorWriter struct{ bytes.Buffer }

func (w *vdiErrorWriter) Write(p []byte) (int, error) {
	limit := min(len(p), 16384-w.Len())
	if limit > 0 {
		w.Buffer.Write(p[:limit])
	}
	return len(p), nil
}
