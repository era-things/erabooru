package workers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"era/booru/ent"
	"era/booru/ent/media"
	"era/booru/internal/config"
	"era/booru/internal/db"
	"era/booru/internal/minio"
	"era/booru/internal/processing"
	"era/booru/internal/queue"
	"era/booru/internal/search"

	mc "github.com/minio/minio-go/v7"
	"github.com/riverqueue/river"
)

// ProcessWorker processes uploaded media objects.
type ProcessWorker struct {
	river.WorkerDefaults[queue.ProcessArgs]
	Minio *minio.Client
	DB    *ent.Client
	Cfg   *config.Config
	Queue *river.Client[*sql.Tx]
}

func (w *ProcessWorker) Work(ctx context.Context, job *river.Job[queue.ProcessArgs]) error {
	bucket := job.Args.Bucket
	if bucket == "" {
		bucket = w.Minio.Bucket
	}
	if strings.HasPrefix(job.Args.ContentType, "video/") {
		id, err := w.processVideo(ctx, bucket, job.Args.Key)
		if err != nil {
			return err
		}
		return queue.Enqueue(ctx, w.Queue, queue.IndexArgs{ID: id})
	}
	id, err := w.processImage(ctx, bucket, job.Args.Key)
	if err != nil {
		return err
	}
	return queue.Enqueue(ctx, w.Queue, queue.IndexArgs{ID: id})
}

func (w *ProcessWorker) processImage(ctx context.Context, bucket, key string) (string, error) {
	rc, err := w.Minio.GetObject(ctx, bucket, key, mc.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	meta, err := processing.GetMetadata(data)
	if err != nil {
		return "", err
	}

	mediaObj, err := w.DB.Media.Create().
		SetID(key).
		SetFormat(meta.Format).
		SetWidth(int16(meta.Width)).
		SetHeight(int16(meta.Height)).
		Save(ctx)
	if err != nil {
		return "", err
	}

	dt, err := db.FindOrCreateDate(ctx, w.DB, "upload")
	if err != nil {
		return "", err
	}
	if _, err := w.DB.MediaDate.Create().
		SetMediaID(mediaObj.ID).
		SetDateID(dt.ID).
		SetValue(time.Now().UTC()).
		Save(ctx); err != nil {
		return "", err
	}

	tagme, err := db.FindOrCreateTag(ctx, w.DB, "tagme")
	if err == nil {
		_, _ = w.DB.Media.UpdateOneID(mediaObj.ID).AddTagIDs(tagme.ID).Save(ctx)
	}

	return mediaObj.ID, nil
}

func (w *ProcessWorker) processVideo(ctx context.Context, bucket, key string) (string, error) {
	scheme := "http://"
	if w.Cfg.MinioSSL {
		scheme = "https://"
	}
	src := fmt.Sprintf("%s%s/%s/%s", scheme, w.Cfg.MinioInternalEndpoint, strings.TrimPrefix(bucket, "/"), strings.TrimPrefix(key, "/"))

	probeOut, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", "-show_format", src).Output()
	if err != nil {
		return "", err
	}
	var probe struct {
		Streams []struct {
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probeOut, &probe); err != nil {
		return "", err
	}
	width, height, duration := 0, 0, 0
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			if s.Width > width {
				width = s.Width
			}
			if s.Height > height {
				height = s.Height
			}
			if s.Duration != "" {
				if f, err := strconv.ParseFloat(s.Duration, 64); err == nil {
					if int(f+0.5) > duration {
						duration = int(f + 0.5)
					}
				}
			}
		}
	}
	if duration == 0 && probe.Format.Duration != "" {
		if f, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			duration = int(f + 0.5)
		}
	}
	format := probe.Format.FormatName
	for f := range config.SupportedVideoFormats {
		if strings.Contains(format, f) {
			format = f
			break
		}
	}

	cmd := exec.Command("ffmpeg",
		"-ss", "00:00:02", "-i", src, "-y", "-loglevel", "error",
		"-vframes", "1", "-vf", "scale=320:-2",
		"-q:v", "3", "-f", "image2", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	previewKey := key
	if _, err = w.Minio.PutPreviewJpeg(ctx, previewKey, stdout); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}

	obj, err := w.Minio.GetObject(ctx, bucket, key, mc.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		return "", err
	}
	_ = hex.EncodeToString(h.Sum(nil))

	mediaObj, err := w.DB.Media.Create().
		SetID(key).
		SetFormat(format).
		SetWidth(int16(width)).
		SetHeight(int16(height)).
		SetDuration(int16(duration)).
		Save(ctx)
	if err != nil {
		return "", err
	}

	dt, err := db.FindOrCreateDate(ctx, w.DB, "upload")
	if err != nil {
		return "", err
	}
	if _, err := w.DB.MediaDate.Create().
		SetMediaID(mediaObj.ID).
		SetDateID(dt.ID).
		SetValue(time.Now().UTC()).
		Save(ctx); err != nil {
		return "", err
	}
	tagme, err := db.FindOrCreateTag(ctx, w.DB, "tagme")
	if err == nil {
		_, _ = w.DB.Media.UpdateOneID(mediaObj.ID).AddTagIDs(tagme.ID).Save(ctx)
	}
	return mediaObj.ID, nil
}

// IndexWorker updates the Bleve index for a media item.
type IndexWorker struct {
	river.WorkerDefaults[queue.IndexArgs]
	DB *ent.Client
}

func (w *IndexWorker) Work(ctx context.Context, job *river.Job[queue.IndexArgs]) error {
	mobj, err := w.DB.Media.Query().Where(media.IDEQ(job.Args.ID)).
		WithTags().
		WithDates(func(q *ent.DateQuery) { q.WithMediaDates() }).
		Only(ctx)
	if ent.IsNotFound(err) {
		return search.DeleteMedia(job.Args.ID)
	}
	if err != nil {
		return err
	}
	return search.IndexMedia(mobj)
}
