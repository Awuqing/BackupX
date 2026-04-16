package backint

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Config 是 Backint Agent 的运行时配置。
//
// SAP HANA 通过 -p <paramfile> 传入一个参数文件。BackupX Backint Agent 复用 SAP
// 的"#KEY = VALUE"风格（兼容原生 backint 参数文件习惯），不支持 section。
//
// 必填字段：
//   - STORAGE_TYPE：存储类型（s3/webdav/local_disk/...，与 BackupX storage registry 一致）
//   - STORAGE_CONFIG_JSON：存储配置 JSON 文件路径（或直接 STORAGE_CONFIG = <json>）
//
// 可选字段：
//   - PARALLEL_FACTOR：并行度（默认 1）
//   - COMPRESS：是否 gzip 压缩（true/false，默认 false）
//   - LOG_FILE：日志文件路径（默认 stderr）
//   - CATALOG_DB：本地目录数据库路径（默认 ./backint_catalog.db）
//   - KEY_PREFIX：对象键前缀（默认空，最终对象键 = <prefix>/<ebid>）
type Config struct {
	StorageType       string
	StorageConfigJSON string         // 存储配置 JSON 文件路径
	StorageConfigRaw  []byte         // 也支持直接内联（STORAGE_CONFIG）
	StorageConfig     map[string]any // 解析后的存储配置
	ParallelFactor    int
	Compress          bool
	LogFile           string
	CatalogDB         string
	KeyPrefix         string
}

// LoadConfigFile 从文件加载配置。
func LoadConfigFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open backint config: %w", err)
	}
	defer f.Close()
	return ParseConfig(f)
}

// ParseConfig 从 reader 解析配置。
func ParseConfig(r io.Reader) (*Config, error) {
	cfg := &Config{ParallelFactor: 1}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		// 兼容可选的 "#" 前缀（SAP 约定）
		line = strings.TrimPrefix(line, "#")
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		switch strings.ToUpper(key) {
		case "STORAGE_TYPE":
			cfg.StorageType = value
		case "STORAGE_CONFIG_JSON":
			cfg.StorageConfigJSON = value
		case "STORAGE_CONFIG":
			cfg.StorageConfigRaw = []byte(value)
		case "PARALLEL_FACTOR":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid PARALLEL_FACTOR: %q", value)
			}
			cfg.ParallelFactor = n
		case "COMPRESS":
			cfg.Compress = parseBool(value)
		case "LOG_FILE":
			cfg.LogFile = value
		case "CATALOG_DB":
			cfg.CatalogDB = value
		case "KEY_PREFIX":
			cfg.KeyPrefix = strings.Trim(value, "/")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := cfg.finalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) finalize() error {
	if c.StorageType == "" {
		return errors.New("STORAGE_TYPE is required")
	}
	if c.CatalogDB == "" {
		c.CatalogDB = "./backint_catalog.db"
	}
	// 加载存储配置 JSON
	var raw []byte
	switch {
	case c.StorageConfigJSON != "":
		data, err := os.ReadFile(c.StorageConfigJSON)
		if err != nil {
			return fmt.Errorf("read STORAGE_CONFIG_JSON: %w", err)
		}
		raw = data
	case len(c.StorageConfigRaw) > 0:
		raw = c.StorageConfigRaw
	default:
		return errors.New("STORAGE_CONFIG_JSON or STORAGE_CONFIG is required")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse storage config JSON: %w", err)
	}
	c.StorageConfig = m
	return nil
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
