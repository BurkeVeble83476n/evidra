package promptfactory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type GenerateOptions struct {
	RootDir         string
	ContractVersion string
	WriteActive     bool
	WriteGenerated  bool
	WriteManifest   bool
}

func Generate(opts GenerateOptions) ([]RenderedFile, error) {
	if opts.RootDir == "" {
		return nil, fmt.Errorf("rootDir is required")
	}
	if opts.ContractVersion == "" {
		return nil, fmt.Errorf("contractVersion is required")
	}

	bundle, err := LoadBundle(opts.RootDir, opts.ContractVersion)
	if err != nil {
		return nil, err
	}
	files, err := RenderFiles(opts.RootDir, bundle)
	if err != nil {
		return nil, err
	}

	if opts.WriteGenerated {
		if err := writeRenderedFiles(opts.RootDir, files, false, true); err != nil {
			return nil, err
		}
	}
	if opts.WriteActive {
		if err := writeRenderedFiles(opts.RootDir, files, true, false); err != nil {
			return nil, err
		}
	}

	if opts.WriteManifest {
		manifest := BuildManifest(bundle.Contract.Version, files)
		manifest.GeneratedAt = stableGeneratedAt(opts.RootDir, bundle.Contract.Version)
		if err := WriteManifest(opts.RootDir, manifest); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func Verify(rootDir, contractVersion string) error {
	if rootDir == "" {
		return fmt.Errorf("rootDir is required")
	}
	if contractVersion == "" {
		return fmt.Errorf("contractVersion is required")
	}

	bundle, err := LoadBundle(rootDir, contractVersion)
	if err != nil {
		return err
	}
	rendered, err := RenderFiles(rootDir, bundle)
	if err != nil {
		return err
	}
	manifest, err := ReadManifest(rootDir, contractVersion)
	if err != nil {
		return err
	}

	for _, f := range rendered {
		activePath := filepath.Join(rootDir, filepath.FromSlash(f.ActiveRel))
		genPath := filepath.Join(rootDir, filepath.FromSlash(f.OutputRel))

		activeBytes, err := os.ReadFile(activePath)
		if err != nil {
			return fmt.Errorf("read active output %s: %w", f.ActiveRel, err)
		}
		if string(activeBytes) != f.Content {
			return fmt.Errorf("active output drift detected: %s", f.ActiveRel)
		}

		genBytes, err := os.ReadFile(genPath)
		if err != nil {
			return fmt.Errorf("read generated output %s: %w", f.OutputRel, err)
		}
		if string(genBytes) != f.Content {
			return fmt.Errorf("generated output drift detected: %s", f.OutputRel)
		}

		hash := hashContent(f.Content)
		if want := manifest.Files[f.ActiveRel]; want != hash {
			return fmt.Errorf("manifest drift for %s: want=%s got=%s", f.ActiveRel, want, hash)
		}
		if want := manifest.Files[f.OutputRel]; want != hash {
			return fmt.Errorf("manifest drift for %s: want=%s got=%s", f.OutputRel, want, hash)
		}
	}

	return nil
}

func writeRenderedFiles(rootDir string, files []RenderedFile, writeActive, writeGenerated bool) error {
	for _, f := range files {
		if writeGenerated {
			path := filepath.Join(rootDir, filepath.FromSlash(f.OutputRel))
			if err := writeFile(path, f.Content); err != nil {
				return fmt.Errorf("write generated file %s: %w", f.OutputRel, err)
			}
		}
		if writeActive {
			path := filepath.Join(rootDir, filepath.FromSlash(f.ActiveRel))
			if err := writeFile(path, f.Content); err != nil {
				return fmt.Errorf("write active file %s: %w", f.ActiveRel, err)
			}
		}
	}
	return nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func stableGeneratedAt(rootDir, contractVersion string) string {
	if existing, err := ReadManifest(rootDir, contractVersion); err == nil && existing.GeneratedAt != "" {
		return existing.GeneratedAt
	}
	return time.Now().UTC().Format(time.RFC3339)
}
