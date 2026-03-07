package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunDetectorsList(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"detectors", "list"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf.String())
	}

	var payload struct {
		Count int `json:"count"`
		Items []struct {
			Tag string `json:"tag"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Count < 20 {
		t.Fatalf("count=%d, want >=20", payload.Count)
	}
	found := false
	for _, item := range payload.Items {
		if item.Tag == "k8s.privileged_container" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected k8s.privileged_container in detector list")
	}
}

func TestRunDetectorsUnknownSubcommand(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	code := run([]string{"detectors", "unknown"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
}
