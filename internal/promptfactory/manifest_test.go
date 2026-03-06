package promptfactory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedSourceContract(t, root)

	_, err := Generate(GenerateOptions{
		RootDir:         root,
		ContractVersion: "v1.0.1",
		WriteActive:     true,
		WriteGenerated:  true,
		WriteManifest:   true,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if err := Verify(root, "v1.0.1"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyDetectsDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedSourceContract(t, root)

	_, err := Generate(GenerateOptions{
		RootDir:         root,
		ContractVersion: "v1.0.1",
		WriteActive:     true,
		WriteGenerated:  true,
		WriteManifest:   true,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	activePath := filepath.Join(root, "prompts", "mcpserver", "tools", "report_description.txt")
	if err := os.WriteFile(activePath, []byte("# contract: v1.0.1\nDRIFT\n"), 0o644); err != nil {
		t.Fatalf("write drifted file: %v", err)
	}

	err = Verify(root, "v1.0.1")
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "drift") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerate_ManifestStableAcrossRuns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedSourceContract(t, root)

	_, err := Generate(GenerateOptions{
		RootDir:         root,
		ContractVersion: "v1.0.1",
		WriteActive:     true,
		WriteGenerated:  true,
		WriteManifest:   true,
	})
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}

	manifestPath := filepath.Join(root, "prompts", "manifests", "v1.0.1.json")
	first, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read first manifest: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	_, err = Generate(GenerateOptions{
		RootDir:         root,
		ContractVersion: "v1.0.1",
		WriteActive:     true,
		WriteGenerated:  true,
		WriteManifest:   true,
	})
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	second, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read second manifest: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("manifest changed across identical generate runs")
	}
}

func seedSourceContract(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(repoRoot(t), "prompts", "source", "contracts", "v1.0.1")
	target := filepath.Join(root, "prompts", "source", "contracts", "v1.0.1")
	if err := copyTree(source, target); err != nil {
		t.Fatalf("copy source contract: %v", err)
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
