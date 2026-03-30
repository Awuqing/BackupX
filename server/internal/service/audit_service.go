package service

import (
	"context"
	"fmt"
	"log"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// AuditEntry 是记录审计日志的输入结构
type AuditEntry struct {
	UserID     uint
	Username   string
	Category   string // auth / storage_target / backup_task / backup_record / settings
	Action     string // create / update / delete / login_success / login_failed / ...
	TargetType string
	TargetID   string
	TargetName string
	Detail     string
	ClientIP   string
}

type AuditService struct {
	repo repository.AuditLogRepository
}

func NewAuditService(repo repository.AuditLogRepository) *AuditService {
	return &AuditService{repo: repo}
}

// Record 异步 fire-and-forget 写入审计日志，不阻塞业务逻辑
func (s *AuditService) Record(entry AuditEntry) {
	if s == nil || s.repo == nil {
		return
	}
	go func() {
		record := &model.AuditLog{
			UserID:     entry.UserID,
			Username:   entry.Username,
			Category:   entry.Category,
			Action:     entry.Action,
			TargetType: entry.TargetType,
			TargetID:   entry.TargetID,
			TargetName: entry.TargetName,
			Detail:     entry.Detail,
			ClientIP:   entry.ClientIP,
		}
		if err := s.repo.Create(context.Background(), record); err != nil {
			log.Printf("[audit] failed to write audit log: %v", err)
		}
	}()
}

// List 分页查询审计日志
func (s *AuditService) List(ctx context.Context, category string, limit, offset int) (*repository.AuditLogListResult, error) {
	result, err := s.repo.List(ctx, repository.AuditLogListOptions{
		Category: category,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, apperror.Internal("AUDIT_LOG_LIST_FAILED", fmt.Sprintf("无法获取审计日志列表: %v", err), err)
	}
	return result, nil
}
