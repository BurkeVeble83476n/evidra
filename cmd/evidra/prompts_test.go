package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptsHelp(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"prompts", "--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "generate") {
		t.Fatalf("help missing generate subcommand: %s", out.String())
	}
	if !strings.Contains(out.String(), "verify") {
		t.Fatalf("help missing verify subcommand: %s", out.String())
	}
}
