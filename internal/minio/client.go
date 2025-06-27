package minio

import (
	"context"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
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
	extra := http.Header{}
	extra.Set("If-None-Match", "*")
	u, err := c.PresignHeader(ctx, http.MethodPut, c.Bucket, object, expiry, nil, extra)
	if err != nil {
		return "", err
	}

	// If MinioPublicHost is empty, return relative URL
	if cfg.MinioPublicHost == "" {
		// Clear scheme and host to make it relative
		u.Scheme = ""
		u.Host = ""
		u.Path = path.Join(cfg.MinioPublicPrefix, u.Path)
		return u.String(), nil
	}

	// Otherwise, use the configured host
	u.Host = cfg.MinioPublicHost
	u.Path = path.Join(cfg.MinioPublicPrefix, u.Path)
	return u.String(), nil
}

// PresignedGet returns a presigned URL for downloading an object.
func (c *Client) PresignedGet(ctx context.Context, cfg *config.Config, object string, expiry time.Duration) (string, error) {
	u, err := c.PresignedGetObject(ctx, c.Bucket, object, expiry, nil)
	if err != nil {
		return "", err
	}

	// If MinioPublicHost is empty, return relative URL
	if cfg.MinioPublicHost == "" {
		// Clear scheme and host to make it relative
		u.Scheme = ""
		u.Host = ""
		u.Path = path.Join(cfg.MinioPublicPrefix, u.Path)
		return u.String(), nil
	}

	// Otherwise, use the configured host
	u.Host = cfg.MinioPublicHost
	u.Path = path.Join(cfg.MinioPublicPrefix, u.Path)
	return u.String(), nil
}

// Watch listens for new object created events and triggers analysis.
func (c *Client) Watch(ctx context.Context, onObject func(ctx context.Context, object string, contentType string)) {
	ch := c.ListenBucketNotification(ctx, c.Bucket, "", "", []string{"s3:ObjectCreated:*"})
	for notification := range ch {
		if notification.Err != nil {
			log.Printf("notification error: %v", notification.Err)
			continue
		}
		for _, rec := range notification.Records {
			go onObject(ctx, rec.S3.Object.Key, rec.S3.Object.ContentType)
		}
	}
}

func (c *Client) WatchPictures(ctx context.Context, onObject func(ctx context.Context, object string)) {
	c.Watch(ctx, func(ctx context.Context, object string, contentType string) {
		if contentType != "" {
			if !strings.HasPrefix(contentType, "image/") {
				log.Printf("skipping non-image object: %s (content-type: %s)", object, contentType)
				return
			}
		}

		log.Printf("new picture: %s", object)
		onObject(ctx, object)
	})
}

func (c *Client) WatchVideos(ctx context.Context, onObject func(ctx context.Context, object string)) {
	c.Watch(ctx, func(ctx context.Context, object string, contentType string) {
		if contentType != "" {
			if !strings.HasPrefix(contentType, "video/") {
				log.Printf("skipping non-video object: %s (content-type: %s)", object, contentType)
				return
			}
		}

		log.Printf("new video: %s", object)
		onObject(ctx, object)
	})
}
