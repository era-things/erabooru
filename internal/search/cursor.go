package search

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type mediaCursor struct {
	UploadedAt time.Time `json:"uploaded_at"`
	ID         string    `json:"id"`
}

type vectorCursor struct {
	Distance float64 `json:"distance"`
	ID       string  `json:"id"`
}

func encodeCursor(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor[T any](raw string) (T, error) {
	var value T
	if raw == "" {
		return value, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return value, fmt.Errorf("%w: %w", ErrInvalidCursor, err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("%w: %w", ErrInvalidCursor, err)
	}
	return value, nil
}
