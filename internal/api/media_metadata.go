package api

import (
	"fmt"
	"strings"
	"time"

	"era/booru/internal/db"
)

type metadataDate struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type metadataRecord struct {
	ID           string         `json:"id"`
	OriginalName *string        `json:"original_name,omitempty"`
	UploadedAt   *time.Time     `json:"uploaded_at,omitempty"`
	Tags         []string       `json:"tags"`
	NegativeTags []string       `json:"negative_tags"`
	Dates        []metadataDate `json:"dates"`
}

func (item metadataRecord) metadata(version int) (db.MediaMetadata, error) {
	out := db.MediaMetadata{ID: item.ID, OriginalName: item.OriginalName, UploadedAt: item.UploadedAt, Tags: normalizeTags(item.Tags)}
	if strings.TrimSpace(item.ID) == "" {
		return out, fmt.Errorf("missing media ID")
	}
	if version >= 3 {
		if item.NegativeTags == nil {
			return out, fmt.Errorf("missing negative_tags for %s", item.ID)
		}
		out.NegativeTags = normalizeTags(item.NegativeTags)
		positive := make(map[string]bool, len(out.Tags))
		for _, tag := range out.Tags {
			positive[tag] = true
		}
		for _, tag := range out.NegativeTags {
			if tag == "tagme" || positive[tag] {
				return out, fmt.Errorf("invalid rejected tag %q for %s", tag, item.ID)
			}
		}
	}
	seenDates := make(map[string]bool, len(item.Dates))
	for _, date := range item.Dates {
		if strings.TrimSpace(date.Name) == "" || seenDates[date.Name] {
			return out, fmt.Errorf("invalid or duplicate date name for %s", item.ID)
		}
		seenDates[date.Name] = true
		value, err := time.Parse(time.RFC3339Nano, date.Value)
		if err != nil {
			value, err = time.Parse("2006-01-02", date.Value)
		}
		if err != nil {
			return out, fmt.Errorf("invalid date %q for %s", date.Value, item.ID)
		}
		out.Dates = append(out.Dates, db.DateValue{Name: date.Name, Value: value})
	}
	return out, nil
}
