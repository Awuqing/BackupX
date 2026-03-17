// Package qiniu provides a Qiniu Cloud Kodo storage factory that delegates to the S3-compatible engine.
// Qiniu Kodo is S3-compatible; we auto-assemble the endpoint from the user-provided region.
package qiniu

import (
	"context"
	"fmt"
	"strings"

	"backupx/server/internal/storage"
	"backupx/server/internal/storage/s3"
)

// Config is the user-facing configuration for Qiniu Kodo.
type Config struct {
	Region          string `json:"region"` // e.g. z0, z1, z2, na0, as0
	Bucket          string `json:"bucket"`
	AccessKey       string `json:"accessKeyId"`
	SecretKey       string `json:"secretAccessKey"`
	Endpoint        string `json:"endpoint"` // optional override
}

// regionEndpoints maps Qiniu storage region codes to their S3-compatible endpoints.
var regionEndpoints = map[string]string{
	"z0":  "https://s3-cn-east-1.qiniucs.com",
	"cn-east-2": "https://s3-cn-east-2.qiniucs.com",
	"z1":  "https://s3-cn-north-1.qiniucs.com",
	"z2":  "https://s3-cn-south-1.qiniucs.com",
	"na0": "https://s3-us-north-1.qiniucs.com",
	"as0": "https://s3-ap-southeast-1.qiniucs.com",
}

// Factory creates Qiniu Kodo providers by composing the S3 engine.
type Factory struct {
	s3Factory s3.Factory
}

func NewFactory() Factory {
	return Factory{s3Factory: s3.NewFactory()}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeQiniuKodo }
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
			return nil, fmt.Errorf("qiniu kodo region is required")
		}
		var ok bool
		endpoint, ok = regionEndpoints[region]
		if !ok {
			return nil, fmt.Errorf("unsupported qiniu region: %s (supported: z0, cn-east-2, z1, z2, na0, as0)", region)
		}
	}

	s3Config := map[string]any{
		"endpoint":        endpoint,
		"region":          cfg.Region,
		"bucket":          cfg.Bucket,
		"accessKeyId":     cfg.AccessKey,
		"secretAccessKey": cfg.SecretKey,
		"forcePathStyle":  true, // Qiniu S3-compatible uses path-style
	}
	return f.s3Factory.New(ctx, s3Config)
}
