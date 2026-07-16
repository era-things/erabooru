package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestStoreLifecycle(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	id, existed, err := store.PutHashedOriginal(ctx, bytes.NewBufferString("hashed"))
	if err != nil || existed || id != "afd9fe2e29bcfac47456c5db865d13ef" {
		t.Fatalf("hashed original: id=%q existed=%v err=%v", id, existed, err)
	}
	duplicateID, existed, err := store.PutHashedOriginal(ctx, bytes.NewBufferString("hashed"))
	if err != nil || !existed || duplicateID != id {
		t.Fatalf("duplicate hashed original: id=%q existed=%v err=%v", duplicateID, existed, err)
	}

	if err := store.PutOriginal(ctx, "media", bytes.NewBufferString("original")); err != nil {
		t.Fatal(err)
	}
	if err := store.PutOriginal(ctx, "media", bytes.NewBufferString("duplicate")); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}

	for _, value := range []string{"first", "replacement"} {
		if err := store.PutPreview(ctx, "media", bytes.NewBufferString(value)); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := os.ReadFile(store.PreviewPath("media"))
	if err != nil {
		t.Fatal(err)
	}
	if string(preview) != "replacement" {
		t.Fatalf("unexpected preview %q", preview)
	}

	if err := store.Remove("media"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.OriginalPath("media")); !os.IsNotExist(err) {
		t.Fatalf("original still exists: %v", err)
	}
}
