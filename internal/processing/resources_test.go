package processing

import (
	"runtime"
	"testing"
)

func TestThreadBudgetLeavesCPUCapacity(t *testing.T) {
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	t.Setenv("INFERENCE_CONCURRENCY", "1")
	for _, test := range []struct{ cpus, want int }{{1, 1}, {2, 1}, {4, 3}, {20, 3}} {
		runtime.GOMAXPROCS(test.cpus)
		if got := DefaultInferenceThreads(); got != test.want {
			t.Fatalf("%d CPUs: got %d threads, want %d", test.cpus, got, test.want)
		}
	}
	runtime.GOMAXPROCS(4)
	t.Setenv("INFERENCE_CONCURRENCY", "2")
	if got := DefaultInferenceThreads(); got != 1 {
		t.Fatalf("concurrent sessions exceed budget: %d", got)
	}
	t.Setenv("ONNX_INTRA_THREADS", "8")
	if got := ResourceThreads("ONNX_INTRA_THREADS", DefaultInferenceThreads()); got != 8 {
		t.Fatal("explicit override ignored")
	}
}
