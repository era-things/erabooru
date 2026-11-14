package search

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	want := mediaCursor{UploadedAt: time.Date(2026, 7, 16, 1, 2, 3, 4, time.UTC), ID: "media"}
	raw := encodeCursor(want)
	got, err := decodeCursor[mediaCursor](raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UploadedAt.Equal(want.UploadedAt) || got.ID != want.ID {
		t.Fatalf("cursor mismatch: got %+v want %+v", got, want)
	}
}

func TestInvalidCursor(t *testing.T) {
	for _, raw := range []string{"not-base64!", "ew"} {
		_, err := decodeCursor[mediaCursor](raw)
		if !errors.Is(fmt.Errorf("request: %w", err), ErrInvalidCursor) {
			t.Fatalf("expected a typed invalid cursor error, got %v", err)
		}
	}
}
