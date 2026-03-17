package s3

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"backupx/server/internal/storage"
	awscore "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type client interface {
	HeadBucket(context.Context, *awss3.HeadBucketInput, ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error)
	PutObject(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	DeleteObject(context.Context, *awss3.DeleteObjectInput, ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
	ListObjectsV2(context.Context, *awss3.ListObjectsV2Input, ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
}

type Provider struct {
	client client
	bucket string
}

type Factory struct {
	newClient func(cfg storage.S3Config) client
}

func NewFactory() Factory {
	return Factory{newClient: func(cfg storage.S3Config) client {
		region := strings.TrimSpace(cfg.Region)
		if region == "" {
			region = "us-east-1"
		}
		awsConfig := awscore.Config{
			Region:      region,
			Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		}
		return awss3.NewFromConfig(awsConfig, func(options *awss3.Options) {
			options.UsePathStyle = cfg.ForcePathStyle
			if strings.TrimSpace(cfg.Endpoint) != "" {
				options.BaseEndpoint = awscore.String(strings.TrimRight(cfg.Endpoint, "/"))
			}
		})
	}}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeS3 }
func (Factory) SensitiveFields() []string  { return []string{"accessKeyId", "secretAccessKey"} }

func (f Factory) New(_ context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
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
	newClient := f.newClient
	if newClient == nil {
		factory := NewFactory()
		newClient = factory.newClient
	}
	return &Provider{client: newClient(cfg), bucket: cfg.Bucket}, nil
}

func (p *Provider) Type() storage.ProviderType { return storage.ProviderTypeS3 }

func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: awscore.String(p.bucket)})
	if err != nil {
		return fmt.Errorf("test s3 connection: %w", err)
	}
	return nil
}

func (p *Provider) Upload(ctx context.Context, objectKey string, reader io.Reader, _ int64, metadata map[string]string) error {
	_, err := p.client.PutObject(ctx, &awss3.PutObjectInput{Bucket: awscore.String(p.bucket), Key: awscore.String(objectKey), Body: reader, Metadata: metadata})
	if err != nil {
		return fmt.Errorf("upload s3 object: %w", err)
	}
	return nil
}

func (p *Provider) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	result, err := p.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: awscore.String(p.bucket), Key: awscore.String(objectKey)})
	if err != nil {
		return nil, fmt.Errorf("download s3 object: %w", err)
	}
	return result.Body, nil
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	_, err := p.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: awscore.String(p.bucket), Key: awscore.String(objectKey)})
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func (p *Provider) List(ctx context.Context, prefix string) ([]storage.ObjectInfo, error) {
	result, err := p.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{Bucket: awscore.String(p.bucket), Prefix: awscore.String(prefix)})
	if err != nil {
		return nil, fmt.Errorf("list s3 objects: %w", err)
	}
	items := make([]storage.ObjectInfo, 0, len(result.Contents))
	for _, object := range result.Contents {
		updatedAt := time.Time{}
		if object.LastModified != nil {
			updatedAt = object.LastModified.UTC()
		}
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}
		items = append(items, storage.ObjectInfo{Key: awscore.ToString(object.Key), Size: size, UpdatedAt: updatedAt})
	}
	return items, nil
}
