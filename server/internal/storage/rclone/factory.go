package rclone

import (
	"context"
	"fmt"
	"strings"

	"backupx/server/internal/storage"

	"github.com/rclone/rclone/fs"
)

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// quoteParam 对 rclone 连接字符串中含特殊字符的值加单引号保护。
func quoteParam(s string) string {
	if s == "" {
		return s
	}
	if !strings.ContainsAny(s, ",:='") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// newFs 创建 rclone fs.Fs 实例并包装为 Provider。
func newFs(ctx context.Context, providerType storage.ProviderType, remote string) (*Provider, error) {
	rfs, err := fs.NewFs(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("create rclone fs for %s: %w", providerType, err)
	}
	return newProvider(providerType, rfs), nil
}

// ---------------------------------------------------------------------------
// LocalDisk
// ---------------------------------------------------------------------------

type LocalDiskFactory struct{}

func NewLocalDiskFactory() LocalDiskFactory { return LocalDiskFactory{} }

func (LocalDiskFactory) Type() storage.ProviderType { return storage.ProviderTypeLocalDisk }
func (LocalDiskFactory) SensitiveFields() []string   { return nil }

func (LocalDiskFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.LocalDiskConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	basePath := strings.TrimSpace(cfg.BasePath)
	if basePath == "" {
		return nil, fmt.Errorf("local disk basePath is required")
	}
	return newFs(ctx, storage.ProviderTypeLocalDisk, basePath)
}

// ---------------------------------------------------------------------------
// S3
// ---------------------------------------------------------------------------

type S3Factory struct{}

func NewS3Factory() S3Factory { return S3Factory{} }

func (S3Factory) Type() storage.ProviderType { return storage.ProviderTypeS3 }
func (S3Factory) SensitiveFields() []string   { return []string{"accessKeyId", "secretAccessKey"} }

func (S3Factory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.S3Config](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("s3 credentials are required")
	}
	return newFs(ctx, storage.ProviderTypeS3, buildS3Remote("Other", cfg.AccessKeyID, cfg.SecretAccessKey, cfg.Endpoint, cfg.Region, cfg.Bucket, cfg.ForcePathStyle))
}

// buildS3Remote 构建 S3 兼容存储的 rclone 连接字符串。
func buildS3Remote(provider, keyID, secret, endpoint, region, bucket string, pathStyle bool) string {
	var b strings.Builder
	b.WriteString(":s3,provider=")
	b.WriteString(quoteParam(provider))
	b.WriteString(",access_key_id=")
	b.WriteString(quoteParam(keyID))
	b.WriteString(",secret_access_key=")
	b.WriteString(quoteParam(secret))
	if strings.TrimSpace(endpoint) != "" {
		b.WriteString(",endpoint=")
		b.WriteString(quoteParam(strings.TrimRight(endpoint, "/")))
	}
	if strings.TrimSpace(region) != "" {
		b.WriteString(",region=")
		b.WriteString(quoteParam(region))
	}
	if pathStyle {
		b.WriteString(",force_path_style=true")
	}
	b.WriteString(",env_auth=false,no_check_bucket=true:")
	b.WriteString(bucket)
	return b.String()
}

// ---------------------------------------------------------------------------
// WebDAV
// ---------------------------------------------------------------------------

type WebDAVFactory struct{}

func NewWebDAVFactory() WebDAVFactory { return WebDAVFactory{} }

func (WebDAVFactory) Type() storage.ProviderType { return storage.ProviderTypeWebDAV }
func (WebDAVFactory) SensitiveFields() []string   { return []string{"username", "password"} }

func (WebDAVFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.WebDAVConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("webdav endpoint is required")
	}
	remote := fmt.Sprintf(":webdav,url=%s,user=%s,pass=%s:%s",
		quoteParam(strings.TrimRight(cfg.Endpoint, "/")),
		quoteParam(cfg.Username),
		quoteParam(cfg.Password),
		strings.TrimSpace(cfg.BasePath))
	return newFs(ctx, storage.ProviderTypeWebDAV, remote)
}

// ---------------------------------------------------------------------------
// Google Drive
// ---------------------------------------------------------------------------

type GoogleDriveFactory struct{}

func NewGoogleDriveFactory() GoogleDriveFactory { return GoogleDriveFactory{} }

func (GoogleDriveFactory) Type() storage.ProviderType { return storage.ProviderTypeGoogleDrive }
func (GoogleDriveFactory) SensitiveFields() []string {
	return []string{"clientId", "clientSecret", "refreshToken"}
}

func (GoogleDriveFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.GoogleDriveConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	cfg = cfg.Normalize()
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("google drive client credentials are required")
	}
	if strings.TrimSpace(cfg.RefreshToken) == "" {
		return nil, fmt.Errorf("google drive refresh token is required")
	}
	// 构造 rclone 所需的 OAuth2 token JSON
	tokenJSON := fmt.Sprintf(`{"access_token":"","token_type":"Bearer","refresh_token":"%s","expiry":"0001-01-01T00:00:00Z"}`,
		strings.ReplaceAll(cfg.RefreshToken, `"`, `\"`))

	var b strings.Builder
	b.WriteString(":drive,client_id=")
	b.WriteString(quoteParam(cfg.ClientID))
	b.WriteString(",client_secret=")
	b.WriteString(quoteParam(cfg.ClientSecret))
	b.WriteString(",token=")
	b.WriteString(quoteParam(tokenJSON))
	if strings.TrimSpace(cfg.FolderID) != "" {
		b.WriteString(",root_folder_id=")
		b.WriteString(quoteParam(cfg.FolderID))
	}
	b.WriteString(":")
	return newFs(ctx, storage.ProviderTypeGoogleDrive, b.String())
}

// ---------------------------------------------------------------------------
// FTP
// ---------------------------------------------------------------------------

type FTPFactory struct{}

func NewFTPFactory() FTPFactory { return FTPFactory{} }

func (FTPFactory) Type() storage.ProviderType { return storage.ProviderTypeFTP }
func (FTPFactory) SensitiveFields() []string   { return []string{"username", "password"} }

func (FTPFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.FTPConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("FTP host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 21
	}
	username := strings.TrimSpace(cfg.Username)
	if username == "" {
		username = "anonymous"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(":ftp,host=%s,port=%d,user=%s,pass=%s",
		quoteParam(cfg.Host), port, quoteParam(username), quoteParam(cfg.Password)))
	if cfg.UseTLS {
		b.WriteString(",tls=true,explicit_tls=true")
	}
	b.WriteString(":")
	basePath := strings.TrimSpace(cfg.BasePath)
	if basePath != "" {
		b.WriteString(basePath)
	}
	return newFs(ctx, storage.ProviderTypeFTP, b.String())
}

// ---------------------------------------------------------------------------
// 阿里云 OSS（委托 S3 引擎）
// ---------------------------------------------------------------------------

type AliyunOSSFactory struct{}

func NewAliyunOSSFactory() AliyunOSSFactory { return AliyunOSSFactory{} }

func (AliyunOSSFactory) Type() storage.ProviderType { return storage.ProviderTypeAliyunOSS }
func (AliyunOSSFactory) SensitiveFields() []string   { return []string{"accessKeyId", "secretAccessKey"} }

// AliyunConfig 是阿里云 OSS 的用户配置。
type AliyunConfig struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	Endpoint        string `json:"endpoint"`
	InternalNetwork bool   `json:"internalNetwork"`
}

func (AliyunOSSFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[AliyunConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			return nil, fmt.Errorf("aliyun oss region is required")
		}
		if cfg.InternalNetwork {
			endpoint = fmt.Sprintf("https://oss-%s-internal.aliyuncs.com", region)
		} else {
			endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", region)
		}
	}
	return newFs(ctx, storage.ProviderTypeAliyunOSS, buildS3Remote("Alibaba", cfg.AccessKeyID, cfg.SecretAccessKey, endpoint, cfg.Region, cfg.Bucket, false))
}

// ---------------------------------------------------------------------------
// 腾讯云 COS（委托 S3 引擎）
// ---------------------------------------------------------------------------

type TencentCOSFactory struct{}

func NewTencentCOSFactory() TencentCOSFactory { return TencentCOSFactory{} }

func (TencentCOSFactory) Type() storage.ProviderType { return storage.ProviderTypeTencentCOS }
func (TencentCOSFactory) SensitiveFields() []string   { return []string{"accessKeyId", "secretAccessKey"} }

// TencentConfig 是腾讯云 COS 的用户配置。
type TencentConfig struct {
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	SecretID  string `json:"accessKeyId"`
	SecretKey string `json:"secretAccessKey"`
	Endpoint  string `json:"endpoint"`
}

func (TencentCOSFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[TencentConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			return nil, fmt.Errorf("tencent cos region is required")
		}
		endpoint = fmt.Sprintf("https://cos.%s.myqcloud.com", region)
	}
	return newFs(ctx, storage.ProviderTypeTencentCOS, buildS3Remote("TencentCOS", cfg.SecretID, cfg.SecretKey, endpoint, cfg.Region, cfg.Bucket, false))
}

// ---------------------------------------------------------------------------
// 七牛云 Kodo（委托 S3 引擎）
// ---------------------------------------------------------------------------

type QiniuKodoFactory struct{}

func NewQiniuKodoFactory() QiniuKodoFactory { return QiniuKodoFactory{} }

func (QiniuKodoFactory) Type() storage.ProviderType { return storage.ProviderTypeQiniuKodo }
func (QiniuKodoFactory) SensitiveFields() []string   { return []string{"accessKeyId", "secretAccessKey"} }

// QiniuConfig 是七牛云 Kodo 的用户配置。
type QiniuConfig struct {
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"accessKeyId"`
	SecretKey string `json:"secretAccessKey"`
	Endpoint  string `json:"endpoint"`
}

// regionEndpoints 映射七牛区域代码到 S3 兼容 endpoint。
var regionEndpoints = map[string]string{
	"z0":        "https://s3-cn-east-1.qiniucs.com",
	"cn-east-2": "https://s3-cn-east-2.qiniucs.com",
	"z1":        "https://s3-cn-north-1.qiniucs.com",
	"z2":        "https://s3-cn-south-1.qiniucs.com",
	"na0":       "https://s3-us-north-1.qiniucs.com",
	"as0":       "https://s3-ap-southeast-1.qiniucs.com",
}

func (QiniuKodoFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[QiniuConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			return nil, fmt.Errorf("qiniu kodo region is required")
		}
		var ok bool
		endpoint, ok = regionEndpoints[region]
		if !ok {
			return nil, fmt.Errorf("unsupported qiniu region: %s (supported: z0, cn-east-2, z1, z2, na0, as0)", region)
		}
	}
	return newFs(ctx, storage.ProviderTypeQiniuKodo, buildS3Remote("Qiniu", cfg.AccessKey, cfg.SecretKey, endpoint, cfg.Region, cfg.Bucket, true))
}

// ---------------------------------------------------------------------------
// 通用 Rclone 后端（支持全部 70+ 后端）
// ---------------------------------------------------------------------------

type RcloneFactory struct{}

func NewRcloneFactory() RcloneFactory { return RcloneFactory{} }

func (RcloneFactory) Type() storage.ProviderType { return storage.ProviderTypeRclone }
func (RcloneFactory) SensitiveFields() []string   { return []string{"pass", "password", "secret_access_key", "client_secret", "token"} }

func (RcloneFactory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	backend, _ := rawConfig["backend"].(string)
	backend = strings.TrimSpace(backend)
	if backend == "" {
		return nil, fmt.Errorf("rclone backend type is required")
	}
	root, _ := rawConfig["root"].(string)
	root = strings.TrimSpace(root)

	// 构建连接字符串：:backend,key1=val1,key2=val2:root
	var b strings.Builder
	b.WriteString(":")
	b.WriteString(backend)
	for key, val := range rawConfig {
		if key == "backend" || key == "root" {
			continue
		}
		strVal := fmt.Sprintf("%v", val)
		if strings.TrimSpace(strVal) == "" {
			continue
		}
		b.WriteString(",")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(quoteParam(strVal))
	}
	b.WriteString(":")
	b.WriteString(root)

	return newFs(ctx, storage.ProviderTypeRclone, b.String())
}

// ListBackends 返回所有可用的 rclone 后端及其配置选项。
func ListBackends() []BackendInfo {
	var backends []BackendInfo
	for _, ri := range fs.Registry {
		if ri.Name == "union" || ri.Name == "crypt" || ri.Name == "chunker" || ri.Name == "compress" || ri.Name == "hasher" || ri.Name == "combine" {
			continue // 跳过组合/加密类后端
		}
		info := BackendInfo{
			Name:        ri.Name,
			Description: ri.Description,
		}
		for _, opt := range ri.Options {
			if opt.Hide != 0 {
				continue
			}
			// 跳过 rclone 为每个后端自动添加的通用选项
			if opt.Name == "description" {
				continue
			}
			info.Options = append(info.Options, BackendOption{
				Key:        opt.Name,
				Label:      opt.Help,
				Required:   opt.Required,
				IsPassword: opt.IsPassword,
			})
		}
		backends = append(backends, info)
	}
	return backends
}

// BackendInfo 描述一个 rclone 后端。
type BackendInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Options     []BackendOption `json:"options"`
}

// BackendOption 描述一个后端配置选项。
type BackendOption struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Required   bool   `json:"required"`
	IsPassword bool   `json:"isPassword"`
}
