package search

import (
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
	if _, err := decodeCursor[mediaCursor]("not-base64!"); err == nil {
		t.Fatal("expected invalid cursor error")
	}
}
