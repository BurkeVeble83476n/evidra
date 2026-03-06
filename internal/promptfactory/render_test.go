package promptfactory

import (
	"reflect"
	"strings"
	"testing"
)

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()

	bundle, err := LoadBundle(repoRoot(t), "v1.0.1")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	a, err := RenderFiles(repoRoot(t), bundle)
	if err != nil {
		t.Fatalf("RenderFiles a: %v", err)
	}
	b, err := RenderFiles(repoRoot(t), bundle)
	if err != nil {
		t.Fatalf("RenderFiles b: %v", err)
	}

	if !reflect.DeepEqual(a, b) {
		t.Fatal("non-deterministic render")
	}
}

func TestRenderFiles_ExpectedTargets(t *testing.T) {
	t.Parallel()

	bundle, err := LoadBundle(repoRoot(t), "v1.0.1")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	files, err := RenderFiles(repoRoot(t), bundle)
	if err != nil {
		t.Fatalf("RenderFiles: %v", err)
	}

	wantIDs := map[string]bool{
		"mcp.initialize":         true,
		"mcp.prescribe":          true,
		"mcp.report":             true,
		"mcp.get_event":          true,
		"mcp.agent_contract":     true,
		"litellm.system":         true,
		"litellm.agent_contract": true,
	}
	if len(files) != len(wantIDs) {
		t.Fatalf("rendered files = %d, want %d", len(files), len(wantIDs))
	}
	for _, f := range files {
		if !wantIDs[f.ID] {
			t.Fatalf("unexpected file id: %s", f.ID)
		}
		if !strings.Contains(f.Content, "contract: v1.0.1") {
			t.Fatalf("%s missing contract header", f.ID)
		}
	}
}
