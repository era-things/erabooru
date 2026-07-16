package embedworker

import "testing"

func TestVideoSampleCountIsBounded(t *testing.T) {
	tests := []struct {
		duration int
		want     int
	}{
		{duration: 3, want: 5},
		{duration: 30, want: 8},
		{duration: 120, want: 12},
		{duration: 3600, want: 16},
	}
	for _, test := range tests {
		if got := videoSampleCount(test.duration); got != test.want {
			t.Errorf("duration %d: expected %d samples, got %d", test.duration, test.want, got)
		}
	}
}
