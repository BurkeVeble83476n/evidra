package mcpserver

import (
	"reflect"
	"strings"
	"testing"
)

func TestToolRegistryRegisterAndDescribe(t *testing.T) {
	t.Parallel()

	reg := newToolRegistry()
	reg.register("prescribe_smart", toolEntry{
		description: "Pre-flight risk assessment",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool": map[string]any{"type": "string"},
			},
		},
	})

	got, err := reg.describe("prescribe_smart")
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if !strings.Contains(got, `"properties"`) {
		t.Fatalf("describe output missing schema body: %q", got)
	}
}

func TestToolRegistryListSorted(t *testing.T) {
	t.Parallel()

	reg := newToolRegistry()
	reg.register("report", toolEntry{description: "report", inputSchema: map[string]any{"type": "object"}})
	reg.register("prescribe_smart", toolEntry{description: "smart", inputSchema: map[string]any{"type": "object"}})

	got := reg.list()
	want := []string{"prescribe_smart", "report"}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("list = %v, want %v", got, want)
	}
}

func TestToolRegistryDescribeUnknown(t *testing.T) {
	t.Parallel()

	reg := newToolRegistry()
	_, err := reg.describe("missing")
	if err == nil {
		t.Fatal("expected error")
	}
}
