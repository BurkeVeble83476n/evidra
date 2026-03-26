package api

import (
	"path/filepath"
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

func assertPathMissing(t *testing.T, spec *yaml.Node, path string) {
	t.Helper()

	paths := findMappingValue(t, spec.Content[0], "paths")
	if findMappingValueOptional(paths, path) != nil {
		t.Fatalf("path %s should be absent", path)
	}
}
