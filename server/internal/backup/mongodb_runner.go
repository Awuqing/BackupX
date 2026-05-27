package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MongoDBRunner 通过 mongodump/mongorestore 备份与恢复 MongoDB 数据库。
// 采用 --archive 流式模式（dump 写 stdout、restore 读 stdin），与 MySQLRunner
// 的 mysqldump/mysql 管线保持一致；产物为未压缩的 mongo archive，由备份管线统一压缩/加密。
type MongoDBRunner struct {
	executor CommandExecutor
}

func NewMongoDBRunner(executor CommandExecutor) *MongoDBRunner {
	if executor == nil {
		executor = NewOSCommandExecutor()
	}
	return &MongoDBRunner{executor: executor}
}

func (r *MongoDBRunner) Type() string {
	return "mongodb"
}

func (r *MongoDBRunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	if _, err := r.executor.LookPath("mongodump"); err != nil {
		return nil, fmt.Errorf("未找到 mongodump 命令 (请确保服务器已安装 mongodb-database-tools)")
	}
	startedAt := task.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	tempDir, err := CreateTaskTempDir(task.Name, startedAt)
	if err != nil {
		return nil, err
	}
	fileName := BuildArtifactName(task.Name, startedAt, "archive")
	artifactPath := filepath.Join(tempDir, fileName)
	file, err := os.Create(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("create mongodump archive file: %w", err)
	}
	defer file.Close()

	args := mongoConnArgs(task.Database)
	dbNames := normalizeDatabaseNames(task.Database.Names)
	if len(dbNames) == 1 {
		args = append(args, "--db", dbNames[0])
		writer.WriteLine(fmt.Sprintf("备份数据库: %s", dbNames[0]))
	} else {
		writer.WriteLine("备份全部数据库")
	}
	args = append(args, "--archive") // 归档流式写入 stdout

	writer.WriteLine(fmt.Sprintf("连接到 MongoDB: %s:%d", task.Database.Host, task.Database.Port))
	stderrWriter := newLogLineWriter(writer, "mongodump")
	writer.WriteLine("开始执行 mongodump")
	if err := r.executor.Run(ctx, "mongodump", args, CommandOptions{Stdout: file, Stderr: stderrWriter}); err != nil {
		return nil, fmt.Errorf("run mongodump: %w: %s", err, stderrWriter.collected())
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat mongodump archive: %w", err)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("mongodump 产物为空，请检查数据库连接与权限")
	}
	writer.WriteLine(fmt.Sprintf("MongoDB 导出完成（文件大小: %s）", formatFileSize(info.Size())))
	return &RunResult{ArtifactPath: artifactPath, FileName: fileName, TempDir: tempDir, Size: info.Size(), StorageKey: BuildStorageKey("mongodb", startedAt, fileName)}, nil
}

func (r *MongoDBRunner) Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	if _, err := r.executor.LookPath("mongorestore"); err != nil {
		return fmt.Errorf("未找到 mongorestore 命令 (请确保服务器已安装 mongodb-database-tools)")
	}
	input, err := os.Open(filepath.Clean(artifactPath))
	if err != nil {
		return fmt.Errorf("open mongodb restore archive: %w", err)
	}
	defer input.Close()

	args := mongoConnArgs(task.Database)
	// --drop：恢复前删除同名集合，保证恢复后与归档一致（与 mysql 恢复的整库覆盖语义对齐）。
	args = append(args, "--drop", "--archive")
	stderr := &bytes.Buffer{}
	writer.WriteLine("开始执行 mongorestore")
	if err := r.executor.Run(ctx, "mongorestore", args, CommandOptions{Stdin: input, Stderr: stderr}); err != nil {
		return fmt.Errorf("run mongorestore: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	writer.WriteLine("MongoDB 恢复完成")
	return nil
}

// mongoConnArgs 构造 mongodump/mongorestore 的连接与认证参数。
// 注意：mongodb-database-tools 无类似 MYSQL_PWD 的密码环境变量，密码只能经 --password 传入；
// 认证库默认 admin（绝大多数部署的管理账号所在库）。
func mongoConnArgs(db DatabaseSpec) []string {
	args := make([]string, 0, 8)
	if strings.TrimSpace(db.Host) != "" {
		args = append(args, "--host", db.Host)
	}
	if db.Port > 0 {
		args = append(args, "--port", strconv.Itoa(db.Port))
	}
	if strings.TrimSpace(db.User) != "" {
		args = append(args, "--username", db.User, "--authenticationDatabase", "admin")
		if strings.TrimSpace(db.Password) != "" {
			args = append(args, "--password", db.Password)
		}
	}
	return args
}
