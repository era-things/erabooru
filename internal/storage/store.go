package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/xxh3"
)

var ErrExists = errors.New("media already exists")

type Store struct {
	originals string
	previews  string
	incoming  string
}

func New(root string) (*Store, error) {
	s := &Store{
		originals: filepath.Join(root, "originals"),
		previews:  filepath.Join(root, "previews"),
		incoming:  filepath.Join(root, ".incoming"),
	}
	for _, dir := range []string{s.originals, s.previews, s.incoming} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) OriginalPath(id string) string { return filepath.Join(s.originals, id) }
func (s *Store) PreviewPath(id string) string  { return filepath.Join(s.previews, id) }
func (s *Store) OriginalsDir() string          { return s.originals }
func (s *Store) PreviewsDir() string           { return s.previews }

func (s *Store) PutOriginal(ctx context.Context, id string, src io.Reader) error {
	if _, err := os.Stat(s.OriginalPath(id)); err == nil {
		return ErrExists
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.writeAtomic(ctx, s.incoming, s.OriginalPath(id), src)
}

// StagedOriginal keeps incoming bytes private until the database publication
// lock is held. Close must be called even after Install.
type StagedOriginal struct {
	ID    string
	Size  int64
	path  string
	store *Store
}

func (s *Store) StageOriginal(ctx context.Context, src io.Reader) (_ *StagedOriginal, retErr error) {
	tmp, err := os.CreateTemp(s.incoming, "upload-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	h := xxh3.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), &contextReader{ctx: ctx, reader: src})
	if err != nil {
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	sum := h.Sum128().Bytes()
	return &StagedOriginal{ID: hex.EncodeToString(sum[:]), Size: size, path: tmp.Name(), store: s}, nil
}
func (o *StagedOriginal) Close() { _ = os.Remove(o.path) }
func (o *StagedOriginal) Install() (bool, error) {
	target := o.store.OriginalPath(o.ID)
	if _, err := os.Stat(target); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.Rename(o.path, target); err != nil {
		if _, statErr := os.Stat(target); statErr == nil {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
func (s *Store) PutHashedOriginal(ctx context.Context, src io.Reader) (string, bool, error) {
	staged, err := s.StageOriginal(ctx, src)
	if err != nil {
		return "", false, err
	}
	defer staged.Close()
	existed, err := staged.Install()
	return staged.ID, existed, err
}

func (s *Store) PutPreview(ctx context.Context, id string, src io.Reader) error {
	if err := os.Remove(s.PreviewPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.writeAtomic(ctx, s.incoming, s.PreviewPath(id), src)
}

func (s *Store) writeAtomic(ctx context.Context, tempDir, target string, src io.Reader) (retErr error) {
	tmp, err := os.CreateTemp(tempDir, ".incoming-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(name)
		}
	}()

	if _, err := io.Copy(tmp, &contextReader{ctx: ctx, reader: src}); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, target)
}

func (s *Store) Remove(id string) error {
	for _, path := range []string{s.OriginalPath(id), s.PreviewPath(id)} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Store) WalkOriginals(fn func(id string) error) error {
	entries, err := os.ReadDir(s.originals)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && !strings.HasPrefix(entry.Name(), ".incoming-") {
			if err := fn(entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
