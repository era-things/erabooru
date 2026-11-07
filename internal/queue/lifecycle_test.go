package queue

import (
	"errors"
	"fmt"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestFailureStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		attempt int
		err     error
		want    string
	}{
		{"retry", 1, errors.New("temporary"), "retrying"},
		{"exhausted", 3, errors.New("temporary"), "failed"},
		{"cancelled", 1, river.JobCancel(errors.New("unsupported")), "failed"},
		{"wrapped cancellation", 1, fmt.Errorf("processing: %w", river.JobCancel(errors.New("unsupported"))), "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := FailureStatus(&rivertype.JobRow{Attempt: test.attempt, MaxAttempts: 3}, test.err)
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
