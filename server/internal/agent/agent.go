package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Agent 是 Agent 进程的主控制器。
type Agent struct {
	cfg      *Config
	client   *MasterClient
	executor *Executor
	version  string

	mu      sync.Mutex
	started bool
}

// New 构造 Agent。
func New(cfg *Config, version string) (*Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := NewMasterClient(cfg.Master, cfg.Token, cfg.InsecureSkipTLSVerify)
	executor := NewExecutor(client, cfg.TempDir)
	return &Agent{
		cfg:      cfg,
		client:   client,
		executor: executor,
		version:  version,
	}, nil
}

// Run 启动 Agent 主循环，阻塞直到 ctx 被取消。
func (a *Agent) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return fmt.Errorf("agent already started")
	}
	a.started = true
	a.mu.Unlock()

	hbInterval := parseDuration(a.cfg.HeartbeatInterval, 15*time.Second)
	pollInterval := parseDuration(a.cfg.PollInterval, 5*time.Second)

	// 首次握手：通过一次心跳确认 token 有效
	if err := a.heartbeatOnce(ctx); err != nil {
		return fmt.Errorf("initial heartbeat failed: %w", err)
	}
	log.Printf("[agent] connected to master %s", a.cfg.Master)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.heartbeatLoop(ctx, hbInterval)
	}()
	go func() {
		defer wg.Done()
		a.pollLoop(ctx, pollInterval)
	}()
	wg.Wait()
	return ctx.Err()
}

// heartbeatLoop 定期发送心跳。
func (a *Agent) heartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.heartbeatOnce(ctx); err != nil {
				log.Printf("[agent] heartbeat failed: %v", err)
			}
		}
	}
}

func (a *Agent) heartbeatOnce(ctx context.Context) error {
	hostname, _ := os.Hostname()
	req := HeartbeatRequest{
		Token:        a.cfg.Token,
		Hostname:     hostname,
		IPAddress:    detectLocalIP(),
		AgentVersion: a.version,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
	}
	_, err := a.client.Heartbeat(ctx, req)
	return err
}

// pollLoop 定期拉取并处理待执行命令。
func (a *Agent) pollLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.pollAndHandleOnce(ctx)
		}
	}
}

func (a *Agent) pollAndHandleOnce(ctx context.Context) {
	cmd, err := a.client.PollCommand(ctx)
	if err != nil {
		log.Printf("[agent] poll command failed: %v", err)
		return
	}
	if cmd == nil {
		return
	}
	log.Printf("[agent] received command #%d type=%s", cmd.ID, cmd.Type)
	switch cmd.Type {
	case "run_task":
		a.handleRunTask(ctx, cmd)
	case "list_dir":
		a.handleListDir(ctx, cmd)
	default:
		msg := fmt.Sprintf("unknown command type: %s", cmd.Type)
		log.Printf("[agent] %s", msg)
		_ = a.client.SubmitCommandResult(ctx, cmd.ID, false, msg, nil)
	}
}

// handleRunTask 处理 run_task 命令
func (a *Agent) handleRunTask(ctx context.Context, cmd *CommandPayload) {
	var payload struct {
		TaskID   uint `json:"taskId"`
		RecordID uint `json:"recordId"`
	}
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		_ = a.client.SubmitCommandResult(ctx, cmd.ID, false, "invalid payload: "+err.Error(), nil)
		return
	}
	if err := a.executor.ExecuteRunTask(ctx, payload.TaskID, payload.RecordID); err != nil {
		_ = a.client.SubmitCommandResult(ctx, cmd.ID, false, err.Error(), nil)
		return
	}
	_ = a.client.SubmitCommandResult(ctx, cmd.ID, true, "", map[string]any{
		"taskId":   payload.TaskID,
		"recordId": payload.RecordID,
	})
}

// handleListDir 处理 list_dir 命令（阶段四实现）
func (a *Agent) handleListDir(ctx context.Context, cmd *CommandPayload) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
		_ = a.client.SubmitCommandResult(ctx, cmd.ID, false, "invalid payload: "+err.Error(), nil)
		return
	}
	entries, err := listLocalDir(payload.Path)
	if err != nil {
		_ = a.client.SubmitCommandResult(ctx, cmd.ID, false, err.Error(), nil)
		return
	}
	_ = a.client.SubmitCommandResult(ctx, cmd.ID, true, "", map[string]any{"entries": entries})
}

// 辅助函数

func parseDuration(s string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return fallback
}

func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}
