package rclone

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
)

// TransferConfig 控制 rclone 传输层行为。
type TransferConfig struct {
	LowLevelRetries int    // 底层 HTTP 请求重试次数，0 保持 rclone 默认（10）
	BandwidthLimit  string // 带宽限制，如 "10M"、"1M:500k"（上传:下载），空或 "0" 不限
}

// ConfiguredContext 返回注入了 rclone 传输配置的 context。
// 各 rclone 后端在 fs.NewFs 时读取 context 中的配置，自动应用重试和限速。
func ConfiguredContext(ctx context.Context, cfg TransferConfig) context.Context {
	ctx, ci := fs.AddConfig(ctx)
	if cfg.LowLevelRetries > 0 {
		ci.LowLevelRetries = cfg.LowLevelRetries
	}
	if cfg.BandwidthLimit != "" && cfg.BandwidthLimit != "0" {
		var bwTable fs.BwTimetable
		if err := bwTable.Set(cfg.BandwidthLimit); err == nil {
			ci.BwLimit = bwTable
		}
	}
	return ctx
}

// StartAccounting 初始化 rclone 的传输统计和令牌桶限速系统。
// 应在应用启动时调用一次。
func StartAccounting(ctx context.Context) {
	accounting.Start(ctx)
}
