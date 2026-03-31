package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type ProviderType = string

const (
	ProviderTypeLocalDisk   ProviderType = "local_disk"
	ProviderTypeGoogleDrive ProviderType = "google_drive"
	ProviderTypeS3          ProviderType = "s3"
	ProviderTypeWebDAV      ProviderType = "webdav"
	ProviderTypeAliyunOSS   ProviderType = "aliyun_oss"
	ProviderTypeTencentCOS  ProviderType = "tencent_cos"
	ProviderTypeQiniuKodo   ProviderType = "qiniu_kodo"
	ProviderTypeFTP         ProviderType = "ftp"
	ProviderTypeRclone      ProviderType = "rclone"
)

const (
	TypeLocalDisk   = string(ProviderTypeLocalDisk)
	TypeGoogleDrive = string(ProviderTypeGoogleDrive)
	TypeS3          = string(ProviderTypeS3)
	TypeWebDAV      = string(ProviderTypeWebDAV)
	TypeAliyunOSS   = string(ProviderTypeAliyunOSS)
	TypeTencentCOS  = string(ProviderTypeTencentCOS)
	TypeQiniuKodo   = string(ProviderTypeQiniuKodo)
	TypeFTP         = string(ProviderTypeFTP)
)

type ObjectInfo struct {
	Key       string    `json:"key"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type StorageProvider interface {
	Type() ProviderType
	TestConnection(context.Context) error
	Upload(ctx context.Context, objectKey string, reader io.Reader, size int64, metadata map[string]string) error
	Download(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectKey string) error
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

type ProviderFactory interface {
	Type() ProviderType
}

// StorageAbout 是可选能力接口，支持查询远端存储空间。
// 并非所有后端都支持（如 S3/FTP 不支持），通过 type assertion 检测。
type StorageAbout interface {
	About(ctx context.Context) (*StorageUsageInfo, error)
}

// StorageUsageInfo 描述远端存储的空间使用情况。
type StorageUsageInfo struct {
	Total   *int64 `json:"total,omitempty"`   // 总空间（字节）
	Used    *int64 `json:"used,omitempty"`    // 已用空间
	Free    *int64 `json:"free,omitempty"`    // 可用空间
	Objects *int64 `json:"objects,omitempty"` // 对象数量
}

func DecodeConfig[T any](raw map[string]any) (T, error) {
	var cfg T
	encoded, err := json.Marshal(raw)
	if err != nil {
		return cfg, fmt.Errorf("marshal config: %w", err)
	}
	if err := json.Unmarshal(encoded, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func DecodeRawConfig[T any](raw json.RawMessage) (T, error) {
	var cfg T
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func ParseProviderType(value string) ProviderType {
	return strings.TrimSpace(value)
}

type LocalDiskConfig struct {
	BasePath string `json:"basePath"`
}

type S3Config struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	ForcePathStyle  bool   `json:"forcePathStyle"`
}

type WebDAVConfig struct {
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`
	BasePath string `json:"basePath"`
}

type GoogleDriveConfig struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectUri"`
	RedirectURL  string `json:"redirectUrl"`
	RefreshToken string `json:"refreshToken"`
	FolderID     string `json:"folderId"`
}

func (cfg GoogleDriveConfig) Normalize() GoogleDriveConfig {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.RedirectURI = strings.TrimSpace(cfg.RedirectURI)
	cfg.RedirectURL = strings.TrimSpace(cfg.RedirectURL)
	cfg.RefreshToken = strings.TrimSpace(cfg.RefreshToken)
	cfg.FolderID = strings.TrimSpace(cfg.FolderID)
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = cfg.RedirectURL
	}
	if cfg.RedirectURL == "" {
		cfg.RedirectURL = cfg.RedirectURI
	}
	return cfg
}

type FTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	BasePath string `json:"basePath"`
	UseTLS   bool   `json:"useTLS"`
}

