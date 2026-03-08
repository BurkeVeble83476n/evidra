package store

import (
	"testing"
)

func TestBenchmarkRunID_Generated(t *testing.T) {
	t.Parallel()
	run := BenchmarkRun{}
	if run.ID != "" {
		t.Fatal("expected empty ID before save")
	}
}
