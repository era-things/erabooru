package minio

import (
	"context"
	"era/booru/internal/config"
	"log"
	"time"

	mc "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps the MinIO SDK client and configured bucket.
type Client struct {
	*mc.Client
	Bucket string
}

// New creates a MinIO client using values from configuration.
func New(cfg *config.Config) (*Client, error) {
	cli, err := mc.New(cfg.MinioEndpoint, &mc.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioUser, cfg.MinioPassword, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return nil, err
	}

	c := &Client{Client: cli, Bucket: cfg.MinioBucket}

	ctx := context.Background()
	exists, err := cli.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.MinioBucket, mc.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// PresignedPut returns a presigned URL for uploading an object.
func (c *Client) PresignedPut(ctx context.Context, object string, expiry time.Duration) (string, error) {
	u, err := c.PresignedPutObject(ctx, c.Bucket, object, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Watch listens for new object created events and triggers analysis.
func (c *Client) Watch(ctx context.Context) {
	ch := c.ListenBucketNotification(ctx, c.Bucket, "", "", []string{"s3:ObjectCreated:*"})
	for notification := range ch {
		if notification.Err != nil {
			log.Printf("notification error: %v", notification.Err)
			continue
		}
		for _, rec := range notification.Records {
			go analyze(rec.S3.Object.Key)
		}
	}
}

func analyze(object string) {
	log.Printf("mock analyze of %s", object)
}
