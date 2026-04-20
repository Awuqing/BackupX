package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// ClusterVersionMonitor 检查集群中 Agent 版本与 Master 的兼容性。
// 产出两类告警：
//  1. Agent 版本落后 Master（major 或 minor 不一致）→ 建议升级
//  2. Agent 版本为空/异常 → Agent 未正确上报
//
// 触发频率：随节点在线监控 15s/次的同频扫描，但每节点 24h 内只告警一次。
type ClusterVersionMonitor struct {
	nodeRepo        repository.NodeRepository
	eventDispatcher EventDispatcher
	masterVersion   string
	mu              sync.Mutex
	notified        map[uint]time.Time
}

func NewClusterVersionMonitor(nodeRepo repository.NodeRepository, masterVersion string) *ClusterVersionMonitor {
	return &ClusterVersionMonitor{
		nodeRepo:      nodeRepo,
		masterVersion: masterVersion,
		notified:      map[uint]time.Time{},
	}
}

func (m *ClusterVersionMonitor) SetEventDispatcher(dispatcher EventDispatcher) {
	m.eventDispatcher = dispatcher
}

// Start 启动后台扫描。ctx 取消时退出。
// scanInterval 建议 30 分钟；resetInterval 建议 24 小时。
func (m *ClusterVersionMonitor) Start(ctx context.Context, scanInterval, resetInterval time.Duration) {
	if scanInterval <= 0 {
		scanInterval = 30 * time.Minute
	}
	if resetInterval <= 0 {
		resetInterval = 24 * time.Hour
	}
	// 启动立即跑一次，让控制台尽快看到
	go func() {
		m.scan(ctx, resetInterval)
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.scan(ctx, resetInterval)
			}
		}
	}()
}

func (m *ClusterVersionMonitor) scan(ctx context.Context, resetInterval time.Duration) {
	nodes, err := m.nodeRepo.List(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	// 清理已不在集群中的节点
	activeIDs := map[uint]bool{}
	for _, n := range nodes {
		activeIDs[n.ID] = true
	}
	for id := range m.notified {
		if !activeIDs[id] {
			delete(m.notified, id)
		}
	}

	for _, node := range nodes {
		// 仅监控已连接过的远程节点（在线 or 曾在线）
		if node.IsLocal {
			continue
		}
		if strings.TrimSpace(node.AgentVer) == "" {
			continue
		}
		if isVersionOutdated(node.AgentVer, m.masterVersion) {
			if last, seen := m.notified[node.ID]; seen && now.Sub(last) < resetInterval {
				continue
			}
			if m.eventDispatcher != nil {
				title := "BackupX Agent 版本落后"
				body := fmt.Sprintf("节点：%s\nAgent 版本：%s\nMaster 版本：%s\n建议升级 Agent 以获得完整兼容性。",
					node.Name, node.AgentVer, m.masterVersion)
				fields := map[string]any{
					"nodeId":        node.ID,
					"nodeName":      node.Name,
					"agentVersion":  node.AgentVer,
					"masterVersion": m.masterVersion,
				}
				_ = m.eventDispatcher.DispatchEvent(ctx, model.NotificationEventAgentOutdated, title, body, fields)
			}
			m.notified[node.ID] = now
		} else {
			delete(m.notified, node.ID) // 升级后不再告警
		}
	}
}

// isVersionOutdated 简单比较 major.minor。
//
// 规则：
//   - master 或 agent 为 "dev" / 空 → 返回 false（不告警）
//   - 都是形如 x.y[.z] 时，agent 的 major.minor < master 视为落后
//   - 解析失败也返回 false（保守策略）
//
// 该策略放宽 patch 级差异，避免小版本发布造成集群大量告警。
func isVersionOutdated(agent, master string) bool {
	a := strings.TrimPrefix(strings.TrimSpace(agent), "v")
	m := strings.TrimPrefix(strings.TrimSpace(master), "v")
	if a == "" || m == "" || a == "dev" || m == "dev" {
		return false
	}
	aMajor, aMinor, ok := splitMajorMinor(a)
	if !ok {
		return false
	}
	mMajor, mMinor, ok := splitMajorMinor(m)
	if !ok {
		return false
	}
	if aMajor < mMajor {
		return true
	}
	if aMajor == mMajor && aMinor < mMinor {
		return true
	}
	return false
}

func splitMajorMinor(v string) (int, int, bool) {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, ok := atoi(parts[0])
	if !ok {
		return 0, 0, false
	}
	minor, ok := atoi(parts[1])
	if !ok {
		return 0, 0, false
	}
	return major, minor, true
}

func atoi(s string) (int, bool) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}
