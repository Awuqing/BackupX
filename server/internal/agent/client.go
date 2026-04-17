package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MasterClient 是 Agent 调用 Master HTTP API 的封装。
type MasterClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewMasterClient 构造 Master 客户端。
func NewMasterClient(baseURL, token string, insecureTLS bool) *MasterClient {
	transport := &http.Transport{}
	if insecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &MasterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: transport,
		},
	}
}

// HeartbeatRequest Agent 上报心跳的请求
type HeartbeatRequest struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname,omitempty"`
	IPAddress    string `json:"ipAddress,omitempty"`
	AgentVersion string `json:"agentVersion,omitempty"`
	OS           string `json:"os,omitempty"`
	Arch         string `json:"arch,omitempty"`
}

// HeartbeatResponse Master 返回的心跳响应
type HeartbeatResponse struct {
	Status string `json:"status"`
	NodeID uint   `json:"nodeId"`
	Name   string `json:"name"`
}

// Heartbeat 上报心跳并获取节点元信息
func (c *MasterClient) Heartbeat(ctx context.Context, req HeartbeatRequest) (*HeartbeatResponse, error) {
	var resp HeartbeatResponse
	if err := c.do(ctx, http.MethodPost, "/api/agent/heartbeat", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CommandPayload 与 service.AgentCommandPayload 对齐
type CommandPayload struct {
	ID      uint            `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PollCommandResponse 轮询响应：无命令时 Command 为 nil
type PollCommandResponse struct {
	Command *CommandPayload `json:"command"`
}

// PollCommand 拉取下一条待执行命令
func (c *MasterClient) PollCommand(ctx context.Context) (*CommandPayload, error) {
	var resp PollCommandResponse
	if err := c.do(ctx, http.MethodPost, "/api/agent/commands/poll", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Command, nil
}

// SubmitCommandResult 上报命令执行结果
func (c *MasterClient) SubmitCommandResult(ctx context.Context, cmdID uint, success bool, errorMsg string, result any) error {
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		resultJSON = data
	}
	payload := map[string]any{
		"success":      success,
		"errorMessage": errorMsg,
	}
	if resultJSON != nil {
		payload["result"] = resultJSON
	}
	path := fmt.Sprintf("/api/agent/commands/%d/result", cmdID)
	return c.do(ctx, http.MethodPost, path, payload, nil)
}

// TaskSpec 与 service.AgentTaskSpec 对齐
type TaskSpec struct {
	TaskID          uint                  `json:"taskId"`
	Name            string                `json:"name"`
	Type            string                `json:"type"`
	SourcePath      string                `json:"sourcePath"`
	SourcePaths     string                `json:"sourcePaths"`
	ExcludePatterns string                `json:"excludePatterns"`
	DBHost          string                `json:"dbHost"`
	DBPort          int                   `json:"dbPort"`
	DBUser          string                `json:"dbUser"`
	DBPassword      string                `json:"dbPassword"`
	DBName          string                `json:"dbName"`
	DBPath          string                `json:"dbPath"`
	ExtraConfig     string                `json:"extraConfig"`
	Compression     string                `json:"compression"`
	Encrypt         bool                  `json:"encrypt"`
	StorageTargets  []StorageTargetConfig `json:"storageTargets"`
}

// StorageTargetConfig 与 service.AgentStorageTargetConfig 对齐
type StorageTargetConfig struct {
	ID     uint            `json:"id"`
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// GetTaskSpec 拉取任务规格
func (c *MasterClient) GetTaskSpec(ctx context.Context, taskID uint) (*TaskSpec, error) {
	var spec TaskSpec
	path := fmt.Sprintf("/api/agent/tasks/%d", taskID)
	if err := c.do(ctx, http.MethodGet, path, nil, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// RecordUpdate 与 service.AgentRecordUpdate 对齐
type RecordUpdate struct {
	Status       string `json:"status,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
	StoragePath  string `json:"storagePath,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	LogAppend    string `json:"logAppend,omitempty"`
}

// UpdateRecord 上报备份记录的状态/日志
func (c *MasterClient) UpdateRecord(ctx context.Context, recordID uint, update RecordUpdate) error {
	path := fmt.Sprintf("/api/agent/records/%d", recordID)
	return c.do(ctx, http.MethodPost, path, update, nil)
}

// do 是通用 HTTP 调用。所有 Agent API 都统一走 JSON + X-Agent-Token。
func (c *MasterClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-Agent-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: http %d: %s", method, path, resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	// BackupX API 统一封装成 {code, data, message} 形式，需要解出 data 字段
	var envelope struct {
		Code    string          `json:"code"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Data != nil {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
		return nil
	}
	// 兼容直接返回对象的情况
	return json.Unmarshal(data, out)
}
