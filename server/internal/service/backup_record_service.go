package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

type BackupRecordListInput struct {
	TaskID   *uint
	Status   string
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

type BackupRecordSummary struct {
	ID                uint       `json:"id"`
	TaskID            uint       `json:"taskId"`
	TaskName          string     `json:"taskName"`
	StorageTargetID   uint       `json:"storageTargetId"`
	StorageTargetName string     `json:"storageTargetName"`
	Status            string     `json:"status"`
	FileName          string     `json:"fileName"`
	FileSize          int64      `json:"fileSize"`
	Checksum          string     `json:"checksum"`
	StoragePath       string     `json:"storagePath"`
	DurationSeconds   int        `json:"durationSeconds"`
	ErrorMessage      string     `json:"errorMessage"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	Locked            bool       `json:"locked"`
	BackupKind        string     `json:"backupKind"`
}

type BackupRecordDetail struct {
	BackupRecordSummary
	LogContent           string                    `json:"logContent"`
	LogEvents            []backup.LogEvent         `json:"logEvents,omitempty"`
	StorageUploadResults []StorageUploadResultItem `json:"storageUploadResults,omitempty"`
}

type BackupRecordService struct {
	records   repository.BackupRecordRepository
	execution *BackupExecutionService
	logHub    *backup.LogHub
}

func NewBackupRecordService(records repository.BackupRecordRepository, execution *BackupExecutionService, logHub *backup.LogHub) *BackupRecordService {
	return &BackupRecordService{records: records, execution: execution, logHub: logHub}
}

func (s *BackupRecordService) List(ctx context.Context, input BackupRecordListInput) ([]BackupRecordSummary, error) {
	items, err := s.records.List(ctx, repository.BackupRecordListOptions{TaskID: input.TaskID, Status: strings.TrimSpace(input.Status), DateFrom: input.DateFrom, DateTo: input.DateTo, Limit: input.Limit, Offset: input.Offset})
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_LIST_FAILED", "无法获取备份记录列表", err)
	}
	result := make([]BackupRecordSummary, 0, len(items))
	for _, item := range items {
		result = append(result, toBackupRecordSummary(&item))
	}
	return result, nil
}

func (s *BackupRecordService) Get(ctx context.Context, id uint) (*BackupRecordDetail, error) {
	item, err := s.records.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录详情", err)
	}
	if item == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", err)
	}
	return toBackupRecordDetail(item, s.logHub), nil
}

// BackupContentEntry 描述备份内单个条目（文件或目录），用于内容浏览。
type BackupContentEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}

// BackupRecordContents 是一次备份的内容清单视图。
type BackupRecordContents struct {
	RecordID    uint                 `json:"recordId"`
	Total       int                  `json:"total"`
	Truncated   bool                 `json:"truncated"`
	BasedOnFull uint                 `json:"basedOnFull,omitempty"` // 差异记录时，清单取自该基线全量
	Entries     []BackupContentEntry `json:"entries"`
}

const backupContentsMaxEntries = 10000

// ListContents 返回某备份记录的文件清单（仅文件类型的新全量备份会记录清单）。
// 差异记录回退到其基线全量的清单，近似展示恢复后的目录结构。无清单时返回明确错误。
func (s *BackupRecordService) ListContents(ctx context.Context, id uint) (*BackupRecordContents, error) {
	item, err := s.records.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录", err)
	}
	if item == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", nil)
	}
	manifestJSON := item.Manifest
	basedOnFull := uint(0)
	if strings.TrimSpace(manifestJSON) == "" && item.BaseRecordID != 0 {
		if base, baseErr := s.records.FindByID(ctx, item.BaseRecordID); baseErr == nil && base != nil {
			manifestJSON = base.Manifest
			basedOnFull = base.ID
		}
	}
	if strings.TrimSpace(manifestJSON) == "" {
		return nil, apperror.New(422, "BACKUP_CONTENTS_UNAVAILABLE", "该备份未记录文件清单（仅文件类型的新全量备份支持内容浏览），请重新执行一次全量备份后再试。", nil)
	}
	manifest, decErr := backup.DecodeManifest([]byte(manifestJSON))
	if decErr != nil {
		return nil, apperror.Internal("BACKUP_CONTENTS_DECODE_FAILED", "解析备份清单失败", decErr)
	}
	entries := manifest.Entries
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	total := len(entries)
	truncated := false
	if total > backupContentsMaxEntries {
		entries = entries[:backupContentsMaxEntries]
		truncated = true
	}
	result := &BackupRecordContents{RecordID: item.ID, Total: total, Truncated: truncated, BasedOnFull: basedOnFull, Entries: make([]BackupContentEntry, 0, len(entries))}
	for _, e := range entries {
		result.Entries = append(result.Entries, BackupContentEntry{Path: e.Path, Size: e.Size, IsDir: e.IsDir})
	}
	return result, nil
}

func (s *BackupRecordService) SubscribeLogs(ctx context.Context, id uint, buffer int) (<-chan backup.LogEvent, func(), error) {
	item, err := s.records.FindByID(ctx, id)
	if err != nil {
		return nil, nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录详情", err)
	}
	if item == nil {
		return nil, nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", err)
	}
	channel, cancel := s.logHub.Subscribe(id, buffer)
	return channel, cancel, nil
}

func (s *BackupRecordService) Download(ctx context.Context, id uint) (*DownloadedArtifact, error) {
	return s.execution.DownloadRecord(ctx, id)
}

func (s *BackupRecordService) Restore(ctx context.Context, id uint) error {
	return s.execution.RestoreRecord(ctx, id)
}

func (s *BackupRecordService) Delete(ctx context.Context, id uint) error {
	return s.execution.DeleteRecord(ctx, id)
}

// SetLock 设置或解除备份记录的保留锁定（法律保留）。
// 锁定后该记录免于保留期/数量自动清理，且禁止手动删除，直至显式解锁。
func (s *BackupRecordService) SetLock(ctx context.Context, id uint, locked bool) (*BackupRecordDetail, error) {
	item, err := s.records.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录详情", err)
	}
	if item == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", nil)
	}
	if item.Locked != locked {
		item.Locked = locked
		if err := s.records.Update(ctx, item); err != nil {
			return nil, apperror.Internal("BACKUP_RECORD_LOCK_FAILED", "无法更新备份锁定状态", err)
		}
	}
	return toBackupRecordDetail(item, s.logHub), nil
}

func toBackupRecordSummary(item *model.BackupRecord) BackupRecordSummary {
	return BackupRecordSummary{
		ID:                item.ID,
		TaskID:            item.TaskID,
		TaskName:          item.Task.Name,
		StorageTargetID:   item.StorageTargetID,
		StorageTargetName: item.StorageTarget.Name,
		Status:            item.Status,
		FileName:          item.FileName,
		FileSize:          item.FileSize,
		Checksum:          item.Checksum,
		StoragePath:       item.StoragePath,
		DurationSeconds:   item.DurationSeconds,
		ErrorMessage:      item.ErrorMessage,
		StartedAt:         item.StartedAt,
		CompletedAt:       item.CompletedAt,
		Locked:            item.Locked,
		BackupKind:        item.BackupKind,
	}
}

func toBackupRecordDetail(item *model.BackupRecord, logHub *backup.LogHub) *BackupRecordDetail {
	detail := &BackupRecordDetail{BackupRecordSummary: toBackupRecordSummary(item), LogContent: item.LogContent}
	if item.Status == "running" && logHub != nil {
		events := logHub.Snapshot(item.ID)
		detail.LogEvents = events
		if len(events) > 0 {
			lines := make([]string, 0, len(events))
			for _, event := range events {
				lines = append(lines, event.Message)
			}
			detail.LogContent = strings.Join(lines, "\n")
		}
	}
	// 解析多目标上传结果
	if strings.TrimSpace(item.StorageUploadResults) != "" {
		var uploadResults []StorageUploadResultItem
		if err := json.Unmarshal([]byte(item.StorageUploadResults), &uploadResults); err == nil {
			detail.StorageUploadResults = uploadResults
		}
	}
	return detail
}
