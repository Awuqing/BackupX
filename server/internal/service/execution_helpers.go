package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"backupx/server/pkg/compress"
	backupcrypto "backupx/server/pkg/crypto"
)

// 本文件集中放置「备份执行 / 恢复 / 验证 / 复制」四个执行服务共享的执行期辅助逻辑。
//
// 历史上这些函数（解密存储配置创建 provider、按后缀解密解压归档、判定远程节点、
// 跨节点 local_disk 保护、构建任务执行规格）在四个服务里各复制了一份，差异仅在
// 字段名与少量错误码/日志文案。重复实现既增加维护成本，也容易出现"改了一处忘了
// 另一处"的不一致缺陷。这里抽取为单一实现，各服务通过薄封装方法委托调用，调用方
// 无需改动。

// fileSHA256 计算文件内容的 SHA-256（小写 hex），与备份上传时记录到
// BackupRecord.Checksum 的格式一致，用于恢复/复制前的完整性校验。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyArtifactChecksum 校验下载到本地的备份对象与记录的 SHA-256 是否一致。
// expected 为空时跳过（兼容早期未记录 checksum 的备份）；不一致返回结构化错误，
// 调用方应据此中止恢复，避免还原已损坏或被篡改的数据。
func verifyArtifactChecksum(path, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	actual, err := fileSHA256(path)
	if err != nil {
		return apperror.Internal("BACKUP_CHECKSUM_READ_FAILED", "无法读取备份文件计算校验和", err)
	}
	if !strings.EqualFold(actual, expected) {
		// 包装错误同样使用中文并附上期望/实际哈希：apperror.Error() 会优先返回包装错误，
		// 而恢复记录的 ErrorMessage 取自 err.Error()，需保证对用户可读。
		return apperror.BadRequest("BACKUP_CHECKSUM_MISMATCH",
			"备份文件完整性校验失败：SHA-256 不匹配，文件可能已损坏或被篡改",
			fmt.Errorf("备份文件完整性校验失败：SHA-256 不匹配（期望 %s，实际 %s），文件可能已损坏或被篡改", expected, actual))
	}
	return nil
}

// resolveStorageProvider 查询存储目标、解密其配置并创建 provider。
func resolveStorageProvider(ctx context.Context, targets repository.StorageTargetRepository, registry *storage.Registry, cipher *codec.ConfigCipher, targetID uint) (storage.StorageProvider, error) {
	target, err := targets.FindByID(ctx, targetID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if target == nil {
		return nil, apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", "关联的存储目标不存在", nil)
	}
	configMap := map[string]any{}
	if err := cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		return nil, apperror.Internal("BACKUP_STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	return registry.Create(ctx, target.Type, configMap)
}

// prepareBackupArtifact 按文件后缀依次解密(.enc)与解压(.gz)，返回最终可读路径。
// logger 可为 nil（此时静默执行）。
func prepareBackupArtifact(cipher *codec.ConfigCipher, artifactPath string, logger *backup.ExecutionLogger) (string, error) {
	current := artifactPath
	if strings.HasSuffix(strings.ToLower(current), ".enc") {
		if logger != nil {
			logger.Infof("检测到加密后缀，开始解密")
		}
		decrypted, err := backupcrypto.DecryptFile(cipher.Key(), current)
		if err != nil {
			return "", err
		}
		current = decrypted
	}
	if strings.HasSuffix(strings.ToLower(current), ".gz") {
		if logger != nil {
			logger.Infof("检测到 gzip 压缩，开始解压")
		}
		decompressed, err := compress.GunzipFile(current)
		if err != nil {
			return "", err
		}
		current = decompressed
	}
	return current, nil
}

// resolveRemoteExecutionNode 返回远程（非本机）节点指针，用于判定任务应下发给
// Agent 还是在 Master 本地执行。clusterEnabled 通常为「该服务是否注入了 Agent
// 下发能力」。本机 / 未启用集群 / nodeID=0 / 未找到时返回 nil（走本地执行）。
func resolveRemoteExecutionNode(ctx context.Context, nodeRepo repository.NodeRepository, clusterEnabled bool, nodeID uint) *model.Node {
	if nodeRepo == nil || !clusterEnabled || nodeID == 0 {
		return nil
	}
	node, err := nodeRepo.FindByID(ctx, nodeID)
	if err != nil || node == nil || node.IsLocal {
		return nil
	}
	return node
}

// validateCrossNodeLocalDisk 跨节点 local_disk 保护：若备份记录归属某远程节点，
// 且其存储目标是 local_disk（数据位于该节点本地磁盘），Master 无法跨节点访问，
// 直接返回错误。errCode/opName 由各服务定制，以给出贴合场景的提示文案。
func validateCrossNodeLocalDisk(ctx context.Context, nodeRepo repository.NodeRepository, targets repository.StorageTargetRepository, record *model.BackupRecord, errCode, opName string) error {
	if record == nil || record.NodeID == 0 || nodeRepo == nil {
		return nil
	}
	node, err := nodeRepo.FindByID(ctx, record.NodeID)
	if err != nil || node == nil || node.IsLocal {
		return nil
	}
	target, err := targets.FindByID(ctx, record.StorageTargetID)
	if err != nil || target == nil {
		return nil
	}
	if strings.EqualFold(target.Type, "local_disk") {
		return apperror.BadRequest(errCode,
			fmt.Sprintf("备份位于节点 %s 的本地磁盘（local_disk），Master 无法跨节点%s。", node.Name, opName),
			nil)
	}
	return nil
}

// buildBackupTaskSpec 由备份任务构建执行规格：解析排除规则/源路径、解密 DB 密码、
// 套用 ExtraConfig（SAP HANA 等类型特有字段）。被备份执行与恢复服务共享。
func buildBackupTaskSpec(cipher *codec.ConfigCipher, task *model.BackupTask, startedAt time.Time, tempDir string) (backup.TaskSpec, error) {
	excludePatterns := []string{}
	if strings.TrimSpace(task.ExcludePatterns) != "" {
		if err := json.Unmarshal([]byte(task.ExcludePatterns), &excludePatterns); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析排除规则", err)
		}
	}
	password := ""
	if strings.TrimSpace(task.DBPasswordCiphertext) != "" {
		plain, err := cipher.Decrypt(task.DBPasswordCiphertext)
		if err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECRYPT_FAILED", "无法解密数据库密码", err)
		}
		password = string(plain)
	}
	sourcePaths := []string{}
	if strings.TrimSpace(task.SourcePaths) != "" {
		if err := json.Unmarshal([]byte(task.SourcePaths), &sourcePaths); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析源路径配置", err)
		}
	}
	dbSpec := backup.DatabaseSpec{
		Host:     task.DBHost,
		Port:     task.DBPort,
		User:     task.DBUser,
		Password: password,
		Names:    []string{task.DBName},
		Path:     task.DBPath,
	}
	// 解析 ExtraConfig 填充类型特有字段（目前主要用于 SAP HANA）
	if strings.TrimSpace(task.ExtraConfig) != "" {
		extra := map[string]any{}
		if err := json.Unmarshal([]byte(task.ExtraConfig), &extra); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析扩展配置", err)
		}
		applyHANAExtraConfig(&dbSpec, extra)
	}
	return backup.TaskSpec{
		ID:              task.ID,
		Name:            task.Name,
		Type:            task.Type,
		SourcePath:      task.SourcePath,
		SourcePaths:     sourcePaths,
		ExcludePatterns: excludePatterns,
		StorageTargetID: task.StorageTargetID,
		Compression:     task.Compression,
		Encrypt:         task.Encrypt,
		RetentionDays:   task.RetentionDays,
		MaxBackups:      task.MaxBackups,
		StartedAt:       startedAt,
		TempDir:         tempDir,
		Database:        dbSpec,
	}, nil
}
