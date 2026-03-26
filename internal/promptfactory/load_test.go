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

func TestLoadBundle_LatestContractVersion(t *testing.T) {
	t.Parallel()

	bundle, err := LoadBundle(repoRoot(t), "v1.3.0")
	if err != nil {
		t.Fatalf("LoadBundle(v1.3.0): %v", err)
	}
	if bundle.Contract.Version != "v1.3.0" {
		t.Fatalf("contract version = %q, want %q", bundle.Contract.Version, "v1.3.0")
	}
}

func TestLoadBundle_AllowsMissingAuxiliaryContractFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	versionDir := filepath.Join(rootDir, "prompts", "source", "contracts", "v1.0.1")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	repoRootDir := repoRoot(t)
	for _, name := range []string{"CONTRACT.yaml", "CLASSIFICATION.yaml"} {
		src, err := os.ReadFile(filepath.Join(repoRootDir, "prompts", "source", "contracts", "v1.0.1", name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, name), src, 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	bundle, err := LoadBundle(rootDir, "v1.0.1")
	if err != nil {
		t.Fatalf("LoadBundle without OUTPUT_CONTRACTS.yaml: %v", err)
	}
	if bundle.Contract.Version != "v1.0.1" {
		t.Fatalf("contract version = %q, want v1.0.1", bundle.Contract.Version)
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
