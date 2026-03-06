package promptfactory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundle(t *testing.T) {
	t.Parallel()

	bundle, err := LoadBundle(repoRoot(t), "v1.0.1")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if bundle.Contract.Version != "v1.0.1" {
		t.Fatalf("contract version = %q, want v1.0.1", bundle.Contract.Version)
	}
	if len(bundle.Contract.Invariants) == 0 {
		t.Fatal("expected invariants")
	}
	if len(bundle.Classification.MutateExamples) == 0 {
		t.Fatal("expected mutate examples")
	}
}

func TestLoadBundle_ContractVersionMismatch(t *testing.T) {
	t.Parallel()

	_, err := LoadBundle(repoRoot(t), "v9.9.9")
	if err == nil {
		t.Fatal("expected error")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
