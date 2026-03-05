package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBenchmarkCommand_HelpAvailable(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"benchmark", "--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, errBuf.String())
	}

	s := out.String()
	if !strings.Contains(s, "evidra benchmark <subcommand>") {
		t.Fatalf("help missing command header: %s", s)
	}
	for _, sub := range []string{"run", "list", "validate", "record", "compare", "version"} {
		if !strings.Contains(s, sub) {
			t.Fatalf("help missing subcommand %q: %s", sub, s)
		}
	}
}

func TestBenchmarkCommand_RunNotImplementedByDefault(t *testing.T) {
	t.Setenv(benchmarkFeatureEnv, "")

	var out, errBuf bytes.Buffer
	code := run([]string{"benchmark", "run"}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0 stdout=%s", out.String())
	}
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "not yet implemented") {
		t.Fatalf("stderr missing not-yet-implemented message: %s", stderr)
	}
	if !strings.Contains(stderr, benchmarkRoadmapURL) {
		t.Fatalf("stderr missing roadmap link: %s", stderr)
	}
}

func TestBenchmarkCommand_RunPreviewStubWhenFeatureEnabled(t *testing.T) {
	t.Setenv(benchmarkFeatureEnv, "1")

	var out, errBuf bytes.Buffer
	code := run([]string{"benchmark", "run"}, &out, &errBuf)
	if code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
	}
	if !strings.Contains(errBuf.String(), "preview stub") {
		t.Fatalf("stderr missing preview stub message: %s", errBuf.String())
	}
}
