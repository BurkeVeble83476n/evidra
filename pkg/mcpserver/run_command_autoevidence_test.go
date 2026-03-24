package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra/internal/testutil"
	"samebits.com/evidra/pkg/evidence"
)

func TestRunCommand_MutationAutoEvidenceWritesPrescribeAndReport(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "kubectl"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	handler := &runCommandHandler{
		service: &MCPService{
			evidencePath: dir,
			signer:       testutil.TestSigner(t),
		},
		allowedPrefixes: defaultAllowedPrefixes,
		blockedSubs:     defaultBlockedSubcommands,
	}

	out := handler.execute(t.Context(), RunCommandInput{Command: "kubectl apply -f deploy.yaml"})
	if !out.OK {
		t.Fatalf("run_command ok=false: %+v", out)
	}

	entries, err := evidence.ReadAllEntriesAtPath(dir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if entries[0].Type != evidence.EntryTypePrescribe {
		t.Fatalf("first entry type = %q, want %q", entries[0].Type, evidence.EntryTypePrescribe)
	}
	if entries[1].Type != evidence.EntryTypeReport {
		t.Fatalf("second entry type = %q, want %q", entries[1].Type, evidence.EntryTypeReport)
	}

	var report evidence.ReportPayload
	if err := json.Unmarshal(entries[1].Payload, &report); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if report.Verdict != evidence.VerdictSuccess {
		t.Fatalf("report verdict = %q, want %q", report.Verdict, evidence.VerdictSuccess)
	}
	if report.ExitCode == nil || *report.ExitCode != 0 {
		t.Fatalf("report exit code = %v, want 0", report.ExitCode)
	}
}

func TestRunCommand_MutationAutoEvidenceCapturesFailureExitCode(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "kubectl"), "#!/bin/sh\nexit 7\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	handler := &runCommandHandler{
		service: &MCPService{
			evidencePath: dir,
			signer:       testutil.TestSigner(t),
		},
		allowedPrefixes: defaultAllowedPrefixes,
		blockedSubs:     defaultBlockedSubcommands,
	}

	out := handler.execute(t.Context(), RunCommandInput{Command: "kubectl apply -f deploy.yaml"})
	if out.OK {
		t.Fatalf("run_command ok=true, want false: %+v", out)
	}
	if out.ExitCode != 7 {
		t.Fatalf("exit_code = %d, want 7", out.ExitCode)
	}

	entries, err := evidence.ReadAllEntriesAtPath(dir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}

	var report evidence.ReportPayload
	if err := json.Unmarshal(entries[1].Payload, &report); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if report.Verdict != evidence.VerdictFailure {
		t.Fatalf("report verdict = %q, want %q", report.Verdict, evidence.VerdictFailure)
	}
	if report.ExitCode == nil || *report.ExitCode != 7 {
		t.Fatalf("report exit code = %v, want 7", report.ExitCode)
	}
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
