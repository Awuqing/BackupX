// Package tencent provides a Tencent Cloud COS storage factory that delegates to the S3-compatible engine.
// Tencent COS is fully S3-compatible; we auto-assemble the endpoint from region and appId.
package tencent

import (
	"context"
	"fmt"
	"strings"

	"backupx/server/internal/storage"
	"backupx/server/internal/storage/s3"
)

// Config is the user-facing configuration for Tencent COS.
type Config struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"` // format: bucketname-appid
	SecretID        string `json:"accessKeyId"`
	SecretKey       string `json:"secretAccessKey"`
	Endpoint        string `json:"endpoint"` // optional override
}

// Factory creates Tencent COS providers by composing the S3 engine.
type Factory struct {
	s3Factory s3.Factory
}

func NewFactory() Factory {
	return Factory{s3Factory: s3.NewFactory()}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeTencentCOS }
func (Factory) SensitiveFields() []string  { return []string{"accessKeyId", "secretAccessKey"} }

func (f Factory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[Config](rawConfig)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			return nil, fmt.Errorf("tencent cos region is required")
		}
		// Tencent COS S3-compatible endpoint format
		endpoint = fmt.Sprintf("https://cos.%s.myqcloud.com", region)
	}

	s3Config := map[string]any{
		"endpoint":        endpoint,
		"region":          cfg.Region,
		"bucket":          cfg.Bucket,
		"accessKeyId":     cfg.SecretID,
		"secretAccessKey": cfg.SecretKey,
		"forcePathStyle":  false, // COS uses virtual-hosted style
	}
	return f.s3Factory.New(ctx, s3Config)
}
