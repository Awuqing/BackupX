// Package aliyun provides an Aliyun OSS storage factory that delegates to the S3-compatible engine.
// Aliyun OSS is fully S3-compatible; we auto-assemble the endpoint from the user-provided region.
package aliyun

import (
	"context"
	"fmt"
	"strings"

	"backupx/server/internal/storage"
	"backupx/server/internal/storage/s3"
)

// Config is the user-facing configuration for Aliyun OSS.
type Config struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey  string `json:"secretAccessKey"`
	Endpoint        string `json:"endpoint"`        // optional override
	InternalNetwork bool   `json:"internalNetwork"` // use -internal endpoint
}

// Factory creates Aliyun OSS providers by composing the S3 engine.
type Factory struct {
	s3Factory s3.Factory
}

func NewFactory() Factory {
	return Factory{s3Factory: s3.NewFactory()}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeAliyunOSS }
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
			return nil, fmt.Errorf("aliyun oss region is required")
		}
		suffix := "aliyuncs.com"
		if cfg.InternalNetwork {
			endpoint = fmt.Sprintf("https://oss-%s-internal.%s", region, suffix)
		} else {
			endpoint = fmt.Sprintf("https://oss-%s.%s", region, suffix)
		}
	}

	// Delegate to S3 engine with assembled endpoint.
	s3Config := map[string]any{
		"endpoint":        endpoint,
		"region":          cfg.Region,
		"bucket":          cfg.Bucket,
		"accessKeyId":     cfg.AccessKeyID,
		"secretAccessKey": cfg.SecretAccessKey,
		"forcePathStyle":  false, // Aliyun OSS uses virtual-hosted style
	}
	return f.s3Factory.New(ctx, s3Config)
}
