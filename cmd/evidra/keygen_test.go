package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestCmdKeygen_OutputsKeyPair(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := cmdKeygen(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "EVIDRA_SIGNING_KEY=") {
		t.Error("expected EVIDRA_SIGNING_KEY= in output")
	}
	if !strings.Contains(out, "BEGIN PUBLIC KEY") {
		t.Error("expected PEM public key in output")
	}
	// Extract base64 key and validate it decodes to 32-byte seed
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "EVIDRA_SIGNING_KEY=") {
			b64 := strings.TrimPrefix(line, "EVIDRA_SIGNING_KEY=")
			raw, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				t.Errorf("base64 decode: %v", err)
			}
			if len(raw) != 32 {
				t.Errorf("expected 32-byte seed, got %d bytes", len(raw))
			}
			return
		}
	}
	t.Error("EVIDRA_SIGNING_KEY line not found")
}
