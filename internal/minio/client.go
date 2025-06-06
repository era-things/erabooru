package minio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"path"
	"time"

	"era/booru/ent"
	"era/booru/internal/config"

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
	cli, err := mc.New(cfg.MinioInternalEndpoint, &mc.Options{
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

// Watch listens for new object created events and triggers analysis.
func (c *Client) Watch(ctx context.Context, db *ent.Client) {
	ch := c.ListenBucketNotification(ctx, c.Bucket, "", "", []string{"s3:ObjectCreated:*"})
	for notification := range ch {
		if notification.Err != nil {
			log.Printf("notification error: %v", notification.Err)
			continue
		}
		for _, rec := range notification.Records {
			go c.analyze(ctx, db, rec.S3.Object.Key)
		}
	}
}

func (c *Client) analyze(ctx context.Context, db *ent.Client, object string) {
	rc, err := c.GetObject(ctx, c.Bucket, object, mc.GetObjectOptions{})
	if err != nil {
		log.Printf("get object %s: %v", object, err)
		return
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("read object %s: %v", object, err)
		return
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		log.Printf("decode image %s: %v", object, err)
		return
	}

	if _, err := db.Media.Create().
		SetKey(object).
		SetHash(hash).
		SetFormat(format).
		SetWidth(cfg.Width).
		SetHeight(cfg.Height).
		SetType("image").
		Save(ctx); err != nil {
		log.Printf("create media: %v", err)
	} else {
		log.Printf("saved media %s", object)
	}
}
