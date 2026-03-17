package s3

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/storage"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeClient struct{ data map[string]string }

func (c *fakeClient) HeadBucket(context.Context, *awss3.HeadBucketInput, ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error) {
	return &awss3.HeadBucketOutput{}, nil
}

func (c *fakeClient) PutObject(_ context.Context, input *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	body, _ := io.ReadAll(input.Body)
	c.data[awscore.ToString(input.Key)] = string(body)
	return &awss3.PutObjectOutput{}, nil
}

func (c *fakeClient) GetObject(_ context.Context, input *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	return &awss3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(c.data[awscore.ToString(input.Key)]))}, nil
}

func (c *fakeClient) DeleteObject(_ context.Context, input *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	delete(c.data, awscore.ToString(input.Key))
	return &awss3.DeleteObjectOutput{}, nil
}

func (c *fakeClient) ListObjectsV2(_ context.Context, _ *awss3.ListObjectsV2Input, _ ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	now := time.Now().UTC()
	return &awss3.ListObjectsV2Output{Contents: []awss3types.Object{{Key: awscore.String("backup.tar.gz"), Size: awscore.Int64(10), LastModified: &now}}}, nil
}

func TestS3ProviderCRUD(t *testing.T) {
	factory := Factory{newClient: func(cfg storage.S3Config) client {
		return &fakeClient{data: make(map[string]string)}
	}}
	providerAny, err := factory.New(context.Background(), map[string]any{"bucket": "demo", "accessKeyId": "a", "secretAccessKey": "b"})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	provider := providerAny.(*Provider)
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}
	if err := provider.Upload(context.Background(), "backup.tar.gz", bytes.NewBufferString("payload"), 7, nil); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	reader, err := provider.Download(context.Background(), "backup.tar.gz")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "payload" {
		t.Fatalf("unexpected content: %s", string(content))
	}
	items, err := provider.List(context.Background(), "backup")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "backup.tar.gz" {
		t.Fatalf("unexpected list result: %#v", items)
	}
	if err := provider.Delete(context.Background(), "backup.tar.gz"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
