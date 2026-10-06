package storage

import (
	"encoding/json"
	"fmt"
)

// ConfigForAgent 保留传输配置，MinIO 监控 Token 和容量查询只留在 Master。
func ConfigForAgent(raw []byte) ([]byte, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode agent storage config: %w", err)
	}
	delete(cfg, "minioCapacity")
	delete(cfg, "minioMetricsToken")
	result, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode agent storage config: %w", err)
	}
	return result, nil
}
