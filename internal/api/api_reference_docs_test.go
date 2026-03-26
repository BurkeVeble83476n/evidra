package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestMarkdownAPIReference_CoversLiveExternalIngestSurface(t *testing.T) {
	t.Parallel()

	doc := loadMarkdownAPIReference(t)

	requiredSnippets := []string{
		"### `POST /v1/evidence/ingest/prescribe`",
		"### `POST /v1/evidence/ingest/report`",
		"### `HEAD /auth/check`",
		"`{\"status\":\"ok\"}`",
		"#### POST /v1/bench/trigger",
		"#### GET /v1/runners/jobs",
		"#### POST /v1/runners/jobs/{id}/complete",
		"All | Baseline | Evidra",
		"`all|none|evidra`",
		"Requires `model`, `scenarios`, and `evidence_mode` in the request body.",
		"claimed job payload includes `evidence_mode`",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(doc, snippet) {
			t.Fatalf("api reference missing %q", snippet)
		}
	}
	if strings.Contains(doc, "proxy|smart") {
		t.Fatal("api reference still uses proxy|smart wording for top-level bench filters")
	}
}

func TestOpenAPIReference_StaysAlignedWithLiveRoutes(t *testing.T) {
	t.Parallel()

	spec := loadOpenAPISpec(t)

	assertPathOperations(t, spec, "/auth/check", []string{"get", "head"})
	assertQueryParameterDefaults(t, spec, "/v1/evidence/entries", "limit", "100", "1000")
	assertOperationHasQueryParameter(t, spec, "/v1/evidence/entries", "get", "period")
	assertRequestBodyDoesNotRequireField(t, spec, "/v1/keys", "post", "label")
}

func TestMarkdownAPIReference_BenchEvidenceModeContract(t *testing.T) {
	t.Parallel()

	doc := loadMarkdownAPIReference(t)

	required := []string{
		"Query params: `evidence_mode` (`\"\"` = all, `none` = baseline only, `evidra` = non-`none`)",
		"`evidence_mode` follows the bench contract:",
		"- empty means all runs",
		"- `none` returns baseline runs only",
		"- `evidra` returns all non-`none` runs",
		"Both pairwise and matrix modes honor `evidence_mode` with the same contract as leaderboard/runs/stats.",
		"Query params: `evidence_mode` (`\"\"` = all, `none` = baseline only, `evidra` = non-`none`), `since` (RFC3339).",
	}
	for _, snippet := range required {
		if !strings.Contains(doc, snippet) {
			t.Fatalf("markdown api reference missing %q", snippet)
		}
	}
	if strings.Contains(doc, "proxy|smart") || strings.Contains(doc, "default: proxy") {
		t.Fatal("markdown api reference still contains stale proxy-based evidence_mode contract")
	}
}

func loadMarkdownAPIReference(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "docs", "api-reference.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown api reference: %v", err)
	}
	return string(raw)
}

func assertPathOperations(t *testing.T, spec *yaml.Node, path string, want []string) {
	t.Helper()

	pathNode := findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path)
	for _, method := range want {
		if findMappingValueOptional(pathNode, method) == nil {
			t.Fatalf("path %s missing %s operation", path, method)
		}
	}
}

func assertOperationHasQueryParameter(t *testing.T, spec *yaml.Node, path, method, paramName string) {
	t.Helper()

	params := operationParameters(t, spec, path, method)
	for _, param := range params.Content {
		name := findMappingValueOptional(param, "name")
		in := findMappingValueOptional(param, "in")
		if name != nil && in != nil && name.Value == paramName && in.Value == "query" {
			return
		}
	}
	t.Fatalf("%s %s missing query parameter %s", strings.ToUpper(method), path, paramName)
}

func assertQueryParameterDescriptionContains(t *testing.T, spec *yaml.Node, path, method, paramName string, snippets ...string) {
	t.Helper()

	params := operationParameters(t, spec, path, method)
	for _, param := range params.Content {
		name := findMappingValueOptional(param, "name")
		in := findMappingValueOptional(param, "in")
		if name == nil || in == nil || name.Value != paramName || in.Value != "query" {
			continue
		}

		description := findMappingValueOptional(param, "description")
		if description == nil {
			t.Fatalf("%s %s query param %s missing description", strings.ToUpper(method), path, paramName)
		}
		for _, snippet := range snippets {
			if !strings.Contains(description.Value, snippet) {
				t.Fatalf("%s %s query param %s description = %q, want %q", strings.ToUpper(method), path, paramName, description.Value, snippet)
			}
		}
		return
	}

	t.Fatalf("%s %s missing query parameter %s", strings.ToUpper(method), path, paramName)
}

func assertQueryParameterHasNoStaleProxyContract(t *testing.T, spec *yaml.Node, path, method, paramName string) {
	t.Helper()

	params := operationParameters(t, spec, path, method)
	for _, param := range params.Content {
		name := findMappingValueOptional(param, "name")
		in := findMappingValueOptional(param, "in")
		if name == nil || in == nil || name.Value != paramName || in.Value != "query" {
			continue
		}

		schema := findMappingValueOptional(param, "schema")
		if schema == nil {
			t.Fatalf("%s %s query param %s missing schema", strings.ToUpper(method), path, paramName)
		}
		if def := findMappingValueOptional(schema, "default"); def != nil && def.Value == "proxy" {
			t.Fatalf("%s %s query param %s still defaults to proxy", strings.ToUpper(method), path, paramName)
		}
		if enum := findMappingValueOptional(schema, "enum"); enum != nil {
			for _, item := range enum.Content {
				if item.Value == "proxy" || item.Value == "smart" {
					t.Fatalf("%s %s query param %s enum still contains stale value %q", strings.ToUpper(method), path, paramName, item.Value)
				}
			}
		}
		return
	}

	t.Fatalf("%s %s missing query parameter %s", strings.ToUpper(method), path, paramName)
}

func assertQueryParameterDefaults(t *testing.T, spec *yaml.Node, path, paramName, wantDefault, wantMaximum string) {
	t.Helper()

	params := operationParameters(t, spec, path, "get")
	for _, param := range params.Content {
		name := findMappingValueOptional(param, "name")
		in := findMappingValueOptional(param, "in")
		if name == nil || in == nil || name.Value != paramName || in.Value != "query" {
			continue
		}

		schema := findMappingValue(t, param, "schema")
		def := findMappingValueOptional(schema, "default")
		if def == nil || def.Value != wantDefault {
			t.Fatalf("%s query param %s default = %v, want %s", path, paramName, nodeValue(def), wantDefault)
		}
		max := findMappingValueOptional(schema, "maximum")
		if max == nil || max.Value != wantMaximum {
			t.Fatalf("%s query param %s maximum = %v, want %s", path, paramName, nodeValue(max), wantMaximum)
		}
		return
	}

	t.Fatalf("%s missing query param %s", path, paramName)
}

func assertRequestBodyDoesNotRequireField(t *testing.T, spec *yaml.Node, path, method, field string) {
	t.Helper()

	op := findMappingValue(t, findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path), method)
	requestBody := findMappingValue(t, op, "requestBody")
	content := findMappingValue(t, requestBody, "content")
	jsonBody := findMappingValue(t, content, "application/json")
	schema := findMappingValue(t, jsonBody, "schema")
	required := findMappingValueOptional(schema, "required")
	if required == nil {
		return
	}
	for _, item := range required.Content {
		if item.Value == field {
			t.Fatalf("%s %s should not require %s", strings.ToUpper(method), path, field)
		}
	}
}

func operationParameters(t *testing.T, spec *yaml.Node, path, method string) *yaml.Node {
	t.Helper()

	op := findMappingValue(t, findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path), method)
	return findMappingValue(t, op, "parameters")
}

func nodeValue(n *yaml.Node) string {
	if n == nil {
		return "<nil>"
	}
	return n.Value
}
