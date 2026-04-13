package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func mustResolveSchema(t *testing.T, raw string) *jsonschema.Resolved {
	t.Helper()

	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	return resolved
}

func TestStructuredToolResultValidated_RejectsMissingRequiredField(t *testing.T) {
	t.Parallel()

	resolved := mustResolveSchema(t, `{
		"type": "object",
		"required": ["ok", "summary"],
		"properties": {
			"ok": {"type": "boolean"},
			"summary": {"type": "string"}
		},
		"additionalProperties": false
	}`)

	_, err := structuredToolResultValidated(struct {
		OK bool `json:"ok"`
	}{OK: true}, resolved)
	if err == nil {
		t.Fatal("expected schema validation error, got nil")
	}
	if !strings.Contains(err.Error(), "summary") {
		t.Fatalf("error = %q, want mention of missing summary", err.Error())
	}
}

func TestStructuredToolResultValidated_AcceptsConditionalErrorShape(t *testing.T) {
	t.Parallel()

	resolved := mustResolveSchema(t, `{
		"type": "object",
		"required": ["ok"],
		"properties": {
			"ok": {"type": "boolean"},
			"report_id": {"type": "string"},
			"error": {
				"type": "object",
				"required": ["code", "message"],
				"properties": {
					"code": {"type": "string"},
					"message": {"type": "string"}
				},
				"additionalProperties": false
			}
		},
		"allOf": [
			{
				"if": {"properties": {"ok": {"const": true}}},
				"then": {"required": ["report_id"]},
				"else": {"required": ["error"]}
			}
		],
		"additionalProperties": false
	}`)

	result, err := structuredToolResultValidated(struct {
		OK    bool     `json:"ok"`
		Error *ErrInfo `json:"error,omitempty"`
	}{
		OK:    false,
		Error: &ErrInfo{Code: "not_found", Message: "missing prescription"},
	}, resolved)
	if err != nil {
		t.Fatalf("structuredToolResultValidated: %v", err)
	}
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("Content is empty")
	}
}
