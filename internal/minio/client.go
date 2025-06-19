package minio

import (
	"context"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"path"
	"time"

	"era/booru/internal/config"

	mc "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps the MinIO SDK client and configured bucket.
type Client struct {
	*mc.Client
	Bucket        string
	PreviewBucket string
}

// New creates a MinIO client using values from configuration.
func New(cfg *config.Config) (*Client, error) {
	cli, err := mc.New(cfg.MinioInternalEndpoint, &mc.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioUser, cfg.MinioPassword, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return nil, err
	}

	c := &Client{Client: cli, Bucket: cfg.MinioBucket, PreviewBucket: cfg.PreviewBucket}

	ctx := context.Background()

	// Ensure the main bucket exists
	exists, err := cli.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.MinioBucket, mc.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	// Ensure the preview bucket exists
	exists, err = cli.BucketExists(ctx, cfg.PreviewBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := cli.MakeBucket(ctx, cfg.PreviewBucket, mc.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Client) PutPreviewJpeg(ctx context.Context, object string, reader io.Reader) (mc.UploadInfo, error) {
	return c.Client.PutObject(ctx, c.PreviewBucket, object, reader, -1, mc.PutObjectOptions{
		ContentType: "image/jpeg",
	})
}

// PresignedPut returns a presigned URL for uploading an object.
func (c *Client) PresignedPut(ctx context.Context, cfg *config.Config, object string, expiry time.Duration) (string, error) {
	u, err := c.PresignedPutObject(ctx, c.Bucket, object, expiry)
	if err != nil {
		return "", err
	}

	// Rewrite for browser upload
	u.Host = cfg.MinioPublicHost
	u.Path = path.Join(cfg.MinioPublicPrefix, u.Path) // safe join
	return u.String(), nil
}

// PresignedGet returns a presigned URL for downloading an object.
func (c *Client) PresignedGet(ctx context.Context, cfg *config.Config, object string, expiry time.Duration) (string, error) {
	u, err := c.PresignedGetObject(ctx, c.Bucket, object, expiry, nil)
	if err != nil {
		return "", err
	}

	// Rewrite for browser download
	u.Host = cfg.MinioPublicHost
	u.Path = path.Join(cfg.MinioPublicPrefix, u.Path) // safe join
	return u.String(), nil
}

// Watch listens for new object created events and triggers analysis.
func (c *Client) Watch(ctx context.Context, onObject func(ctx context.Context, object string)) {
	ch := c.ListenBucketNotification(ctx, c.Bucket, "", "", []string{"s3:ObjectCreated:*"})
	for notification := range ch {
		if notification.Err != nil {
			log.Printf("notification error: %v", notification.Err)
			continue
		}
		for _, rec := range notification.Records {
			go onObject(ctx, rec.S3.Object.Key)
		}
	}
}

func (c *Client) WatchPictures(ctx context.Context, onObject func(ctx context.Context, object string)) {
	c.Watch(ctx, func(ctx context.Context, object string) {
		switch path.Ext(object) {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			log.Printf("new picture: %s", object)
		default:
			log.Printf("skipping non-image object: %s", object)
			return
		}

		onObject(ctx, object)
	})
}
