package embedworker

import "testing"

func TestSampleTimestampDoesNotExceedDuration(t *testing.T) {
	duration := 16
	samples := videoSampleCount(duration)
	if samples != duration {
		t.Fatalf("expected samples to equal duration, got %d", samples)
	}

	last := sampleTimestamp(duration, samples, samples-1)
	if !(last < float64(duration)) {
		t.Fatalf("expected last timestamp to be less than duration: got %.6f >= %d", last, duration)
	}
}

func TestSampleTimestampSpacingForShortVideo(t *testing.T) {
	duration := 3
	samples := videoSampleCount(duration)
	if samples != 5 {
		t.Fatalf("expected 5 samples for short video, got %d", samples)
	}

	for i := 0; i < samples; i++ {
		value := sampleTimestamp(duration, samples, i)
		if value < 0 {
			t.Fatalf("timestamp %d should not be negative: %.6f", i, value)
		}
		if value >= float64(duration) {
			t.Fatalf("timestamp %d should be inside duration: %.6f >= %d", i, value, duration)
		}
	}
}

func TestSampleTimestampZeroDuration(t *testing.T) {
	if ts := sampleTimestamp(0, 5, 0); ts != 0 {
		t.Fatalf("expected zero timestamp for zero duration, got %.6f", ts)
	}
}

func TestFormatTimestampFloorsMilliseconds(t *testing.T) {
	got := formatTimestamp(15.9999)
	expected := "00:00:15.999"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}
