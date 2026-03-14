package experiments

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunArtifact_CleanOutDirWithoutExplicitOutDir(t *testing.T) {
	root := repoRootForExperiments(t)
	chdirTemp(t)

	outRoot := filepath.Join(DefaultArtifactOutRoot)
	if err := os.MkdirAll(filepath.Join(outRoot, "old-run"), 0o755); err != nil {
		t.Fatalf("mkdir old run: %v", err)
	}
	stale := filepath.Join(outRoot, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	err := RunArtifact(context.Background(), ArtifactRunOptions{
		ModelID:          "test/model",
		Provider:         "test",
		PromptFile:       filepath.Join(root, DefaultPromptFile),
		CasesDir:         filepath.Join(root, DefaultArtifactCasesDir),
		Agent:            "dry-run",
		Repeats:          1,
		MaxCases:         1,
		CleanOutDir:      true,
		DelayBetweenRuns: 0,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("RunArtifact: %v", err)
	}

	if _, err := os.Stat(stale); err == nil {
		t.Fatalf("stale file still exists: %s", stale)
	}
	if _, err := os.Stat(filepath.Join(outRoot, "old-run")); err == nil {
		t.Fatalf("old run directory still exists")
	}
}

func repoRootForExperiments(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func chdirTemp(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%s): %v", tmp, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
