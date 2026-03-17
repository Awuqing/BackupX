package service

import (
	"context"
	"path/filepath"
	"syscall"
	"time"

	"backupx/server/internal/config"
)

type SystemInfo struct {
	Version       string `json:"version"`
	Mode          string `json:"mode"`
	StartedAt     string `json:"startedAt"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	DatabasePath  string `json:"databasePath"`
	DiskTotal     int64  `json:"diskTotal"`
	DiskFree      int64  `json:"diskFree"`
	DiskUsed      int64  `json:"diskUsed"`
}

type SystemService struct {
	cfg       config.Config
	version   string
	startedAt time.Time
}

func NewSystemService(cfg config.Config, version string, startedAt time.Time) *SystemService {
	return &SystemService{cfg: cfg, version: version, startedAt: startedAt}
}

func (s *SystemService) GetInfo(_ context.Context) *SystemInfo {
	now := time.Now().UTC()
	info := &SystemInfo{
		Version:       s.version,
		Mode:          s.cfg.Server.Mode,
		StartedAt:     s.startedAt.Format(time.RFC3339),
		UptimeSeconds: int64(now.Sub(s.startedAt).Seconds()),
		DatabasePath:  s.cfg.Database.Path,
	}
	dir := filepath.Dir(s.cfg.Database.Path)
	if dir == "" {
		dir = "."
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err == nil {
		info.DiskTotal = int64(stat.Blocks) * int64(stat.Bsize)
		info.DiskFree = int64(stat.Bavail) * int64(stat.Bsize)
		info.DiskUsed = info.DiskTotal - info.DiskFree
	}
	return info
}
