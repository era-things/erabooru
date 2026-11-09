package queue

import (
	"slices"
	"testing"

	"github.com/riverqueue/river/rivertype"
)

func TestMediaJobsCanBeReinsertedAfterCompletion(t *testing.T) {
	t.Parallel()

	for _, opts := range []struct {
		name   string
		states []rivertype.JobState
	}{
		{name: "process", states: ProcessArgs{}.InsertOpts().UniqueOpts.ByState},
		{name: "embed", states: EmbedArgs{}.InsertOpts().UniqueOpts.ByState},
	} {
		t.Run(opts.name, func(t *testing.T) {
			if slices.Contains(opts.states, rivertype.JobStateCompleted) {
				t.Fatal("completed jobs must not suppress a new job for re-uploaded media")
			}
			if !slices.Contains(opts.states, rivertype.JobStateRunning) {
				t.Fatal("running jobs should still suppress duplicate work")
			}
		})
	}
}
