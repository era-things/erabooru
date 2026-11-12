package api

import (
	"testing"
	"time"
)

func TestMetadataRecordValidation(t *testing.T) {
	for _, test := range []struct {
		name    string
		version int
		item    metadataRecord
		valid   bool
	}{
		{"legacy dates", 2, metadataRecord{ID: "media", Dates: []metadataDate{{"upload", "2021-01-02"}}}, true},
		{"timestamp", 3, metadataRecord{ID: "media", NegativeTags: []string{}, Dates: []metadataDate{{"upload", "2021-01-02T03:04:05.123456Z"}}}, true},
		{"missing negatives", 3, metadataRecord{ID: "media"}, false},
		{"conflicting feedback", 3, metadataRecord{ID: "media", Tags: []string{"cat"}, NegativeTags: []string{" cat "}}, false},
		{"rejected system tag", 3, metadataRecord{ID: "media", NegativeTags: []string{"tagme"}}, false},
		{"missing ID", 3, metadataRecord{NegativeTags: []string{}}, false},
		{"invalid date", 3, metadataRecord{ID: "media", NegativeTags: []string{}, Dates: []metadataDate{{"upload", "bad"}}}, false},
		{"duplicate dates", 3, metadataRecord{ID: "media", NegativeTags: []string{}, Dates: []metadataDate{{"upload", "2021-01-02T00:00:00Z"}, {"upload", "2021-01-02T00:00:00Z"}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.item.metadata(test.version)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, err=%v", test.valid, err)
			}
		})
	}
	legacy, err := (metadataRecord{ID: "media"}).metadata(2)
	if err != nil || legacy.NegativeTags != nil {
		t.Fatalf("legacy import must preserve feedback: %+v, %v", legacy, err)
	}
	item, err := (metadataRecord{ID: "media", NegativeTags: []string{" cat ", "cat"}, Dates: []metadataDate{{"upload", "2021-01-02T03:04:05.123456Z"}}}).metadata(3)
	if err != nil || len(item.NegativeTags) != 1 || item.Dates[0].Value.Nanosecond() != 123456000 || item.Dates[0].Value.Hour() != 3 {
		t.Fatalf("normalization or precision lost: %+v, %v", item, err)
	}
	if !item.Dates[0].Value.Equal(time.Date(2021, 1, 2, 3, 4, 5, 123456000, time.UTC)) {
		t.Fatal("date changed")
	}
}
