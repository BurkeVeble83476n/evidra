package api

import (
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestOpenAPIBenchRoutesDocumentSupportedSurface(t *testing.T) {
	t.Parallel()

	spec := loadOpenAPISpec(t)

	assertPathOperations(t, spec, "/v1/bench/leaderboard", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/scenarios", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/runs", []string{"get", "post"})
	assertPathOperations(t, spec, "/v1/bench/runs/batch", []string{"post"})
	assertPathOperations(t, spec, "/v1/bench/runs/{id}", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/runs/{id}/transcript", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/runs/{id}/tool-calls", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/runs/{id}/timeline", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/runs/{id}/scorecard", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/stats", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/catalog", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/trigger", []string{"post"})
	assertPathOperations(t, spec, "/v1/bench/trigger/{id}", []string{"get"})
	assertPathOperations(t, spec, "/v1/bench/trigger/{id}/progress", []string{"post"})
	assertPathOperations(t, spec, "/v1/runners/register", []string{"post"})
	assertPathOperations(t, spec, "/v1/runners", []string{"get"})
	assertPathOperations(t, spec, "/v1/runners/{id}", []string{"delete"})
	assertPathOperations(t, spec, "/v1/runners/jobs", []string{"get"})
	assertPathOperations(t, spec, "/v1/runners/jobs/{id}/complete", []string{"post"})

	assertRequestBodyRequiredFields(t, spec, "/v1/bench/trigger", "post", []string{"model", "scenarios", "evidence_mode"})
	assertRequestBodyDoesNotRequireField(t, spec, "/v1/bench/trigger", "post", "execution_mode")
	assertRequestBodyPropertyEnumValues(t, spec, "/v1/bench/trigger", "post", "evidence_mode", []string{"none", "smart"})
	assertRequestBodyPropertyEnumValues(t, spec, "/v1/bench/trigger", "post", "execution_mode", []string{"provider", "a2a"})
	assertResponseSchemaHasProperty(t, spec, "/v1/runners/jobs", "get", "200", "evidence_mode")
	assertResponseSchemaHasProperty(t, spec, "/v1/runners/jobs", "get", "200", "execution_mode")

	assertPathMissing(t, spec, "/v1/benchmark/run")
	assertPathMissing(t, spec, "/v1/benchmark/runs")
	assertPathMissing(t, spec, "/v1/benchmark/compare")
}

func TestOpenAPIBenchFilterContractDocumentsEvidenceModeSemantics(t *testing.T) {
	t.Parallel()

	wantSnippets := []string{
		"Empty means all runs.",
		"`none` returns baseline runs only.",
		"`evidra` returns all non-`none` runs.",
		"Non-empty, non-`evidra` values are exact-match filters against stored modes.",
	}

	specs := []*yaml.Node{
		loadOpenAPISpec(t),
		loadOpenAPISpecFromPath(t, filepath.Join("..", "..", "ui", "public", "openapi.yaml")),
	}
	paths := []string{"/v1/bench/leaderboard", "/v1/bench/runs", "/v1/bench/stats", "/v1/bench/compare/models", "/v1/bench/signals"}

	for _, spec := range specs {
		for _, path := range paths {
			assertQueryParameterDescriptionContains(t, spec, path, "get", "evidence_mode", wantSnippets...)
			assertQueryParameterHasNoStaleProxyContract(t, spec, path, "get", "evidence_mode")
		}
	}
}

func assertRequestBodyRequiredFields(t *testing.T, spec *yaml.Node, path, method string, want []string) {
	t.Helper()

	schema := requestBodySchema(t, spec, path, method)
	required := findMappingValueOptional(schema, "required")
	got := map[string]struct{}{}
	if required != nil {
		for _, item := range required.Content {
			got[item.Value] = struct{}{}
		}
	}
	for _, field := range want {
		if _, ok := got[field]; !ok {
			t.Fatalf("%s %s missing required field %s", strings.ToUpper(method), path, field)
		}
	}
}

func assertRequestBodyPropertyEnumValues(t *testing.T, spec *yaml.Node, path, method, property string, want []string) {
	t.Helper()

	schema := requestBodySchema(t, spec, path, method)
	properties := findMappingValue(t, schema, "properties")
	assertSchemaEnumValues(t, findMappingValue(t, properties, property), want)
}

func assertResponseSchemaHasProperty(t *testing.T, spec *yaml.Node, path, method, code, property string) {
	t.Helper()

	schema := responseSchema(t, spec, path, method, code)
	properties := findMappingValue(t, schema, "properties")
	if findMappingValueOptional(properties, property) == nil {
		t.Fatalf("%s %s %s response missing property %s", strings.ToUpper(method), path, code, property)
	}
}

func assertPathMissing(t *testing.T, spec *yaml.Node, path string) {
	t.Helper()

	paths := findMappingValue(t, spec.Content[0], "paths")
	if findMappingValueOptional(paths, path) != nil {
		t.Fatalf("path %s should be absent", path)
	}
}

func requestBodySchema(t *testing.T, spec *yaml.Node, path, method string) *yaml.Node {
	t.Helper()

	op := findMappingValue(t, findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path), method)
	requestBody := findMappingValue(t, op, "requestBody")
	content := findMappingValue(t, requestBody, "content")
	jsonBody := findMappingValue(t, content, "application/json")
	return findMappingValue(t, jsonBody, "schema")
}

func responseSchema(t *testing.T, spec *yaml.Node, path, method, code string) *yaml.Node {
	t.Helper()

	op := findMappingValue(t, findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path), method)
	responses := findMappingValue(t, op, "responses")
	response := findMappingValue(t, responses, code)
	content := findMappingValue(t, response, "content")
	jsonBody := findMappingValue(t, content, "application/json")
	return findMappingValue(t, jsonBody, "schema")
}
