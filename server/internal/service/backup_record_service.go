package service

import (
	"context"
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
	StoragePath       string     `json:"storagePath"`
	DurationSeconds   int        `json:"durationSeconds"`
	ErrorMessage      string     `json:"errorMessage"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
}

type BackupRecordDetail struct {
	BackupRecordSummary
	LogContent string            `json:"logContent"`
	LogEvents  []backup.LogEvent `json:"logEvents,omitempty"`
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
		StoragePath:       item.StoragePath,
		DurationSeconds:   item.DurationSeconds,
		ErrorMessage:      item.ErrorMessage,
		StartedAt:         item.StartedAt,
		CompletedAt:       item.CompletedAt,
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
	return detail
}
