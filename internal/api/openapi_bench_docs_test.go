package api

import (
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

	assertPathMissing(t, spec, "/v1/benchmark/run")
	assertPathMissing(t, spec, "/v1/benchmark/runs")
	assertPathMissing(t, spec, "/v1/benchmark/compare")
}

func TestOpenAPIBenchFilterContractDocumentsEvidenceModeSemantics(t *testing.T) {
	t.Parallel()

	spec := loadOpenAPISpec(t)

	wantSnippets := []string{
		"Empty means all runs.",
		"`none` returns baseline runs only.",
		"`evidra` returns all non-`none` runs.",
	}

	assertQueryParameterDescriptionContains(t, spec, "/v1/bench/leaderboard", "get", "evidence_mode", wantSnippets...)
	assertQueryParameterDescriptionContains(t, spec, "/v1/bench/stats", "get", "evidence_mode", wantSnippets...)
	assertQueryParameterDescriptionContains(t, spec, "/v1/bench/compare/models", "get", "evidence_mode", wantSnippets...)
}

func assertPathMissing(t *testing.T, spec *yaml.Node, path string) {
	t.Helper()

	paths := findMappingValue(t, spec.Content[0], "paths")
	if findMappingValueOptional(paths, path) != nil {
		t.Fatalf("path %s should be absent", path)
	}
}
