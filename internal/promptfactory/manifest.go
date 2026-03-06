package promptfactory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func BuildManifest(contractVersion string, files []RenderedFile) Manifest {
	m := Manifest{
		ContractVersion: contractVersion,
		Files:           map[string]string{},
	}
	for _, f := range files {
		m.Files[f.ActiveRel] = hashContent(f.Content)
		m.Files[f.OutputRel] = hashContent(f.Content)
	}
	return m
}

func WriteManifest(rootDir string, manifest Manifest) error {
	if manifest.ContractVersion == "" {
		return fmt.Errorf("manifest contract version is required")
	}
	path := manifestPath(rootDir, manifest.ContractVersion)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir manifest dir: %w", err)
	}

	stable := struct {
		ContractVersion string            `json:"contract_version"`
		GeneratedAt     string            `json:"generated_at"`
		Files           map[string]string `json:"files"`
	}{
		ContractVersion: manifest.ContractVersion,
		GeneratedAt:     manifest.GeneratedAt,
		Files:           map[string]string{},
	}

	keys := make([]string, 0, len(manifest.Files))
	for k := range manifest.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		stable.Files[k] = manifest.Files[k]
	}

	b, err := json.MarshalIndent(stable, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func ReadManifest(rootDir, contractVersion string) (Manifest, error) {
	path := manifestPath(rootDir, contractVersion)
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if m.ContractVersion == "" {
		m.ContractVersion = contractVersion
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return m, nil
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

func manifestPath(rootDir, contractVersion string) string {
	return filepath.Join(rootDir, "prompts", "manifests", contractVersion+".json")
}
