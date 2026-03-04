package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-benchmark/pkg/evidence"
)

const testCanonicalAction = `{"tool":"terraform","operation":"apply","operation_class":"mutate","scope_class":"production","resource_count":1,"resource_shape_hash":"sha256:test"}`

func TestRunPrescribe_ScannerReportParseError(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "artifact.json")
	badSarif := filepath.Join(tmp, "bad.sarif")

	if err := os.WriteFile(artifact, []byte(`{"noop":true}`), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := os.WriteFile(badSarif, []byte(`not json`), 0o644); err != nil {
		t.Fatalf("write bad sarif: %v", err)
	}

	args := []string{
		"prescribe",
		"--tool", "terraform",
		"--artifact", artifact,
		"--canonical-action", testCanonicalAction,
		"--scanner-report", badSarif,
		"--evidence-dir", tmp,
	}

	var out, errBuf bytes.Buffer
	code := run(args, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(errBuf.String(), "parse scanner report") {
		t.Fatalf("stderr missing parse scanner report: %s", errBuf.String())
	}
}

func TestRunPrescribe_ScannerReportCountsWrittenFindings(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "artifact.json")
	if err := os.WriteFile(artifact, []byte(`{"noop":true}`), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	scannerReport, err := filepath.Abs("../../tests/testdata/sarif_trivy.json")
	if err != nil {
		t.Fatalf("resolve scanner report path: %v", err)
	}

	args := []string{
		"prescribe",
		"--tool", "terraform",
		"--artifact", artifact,
		"--canonical-action", testCanonicalAction,
		"--scanner-report", scannerReport,
		"--evidence-dir", tmp,
	}

	var out, errBuf bytes.Buffer
	code := run(args, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errBuf.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	count, ok := result["findings_count"].(float64)
	if !ok {
		t.Fatalf("findings_count missing or non-number: %#v", result["findings_count"])
	}
	if int(count) != 1 {
		t.Fatalf("findings_count = %d, want 1", int(count))
	}

	entries, err := evidence.ReadAllEntriesAtPath(tmp)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}

	findingCount := 0
	for _, e := range entries {
		if e.Type != evidence.EntryTypeFinding {
			continue
		}
		findingCount++
		if e.Actor.ID != "cli" {
			t.Fatalf("finding actor id = %q, want cli", e.Actor.ID)
		}
	}

	if findingCount != 1 {
		t.Fatalf("finding entry count = %d, want 1", findingCount)
	}
}
