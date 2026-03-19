package api

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestOpenAPIIngestRoutesDocumentContracts(t *testing.T) {
	t.Parallel()

	spec := loadOpenAPISpec(t)

	assertPathResponses(t, spec, "/v1/evidence/ingest/prescribe", []string{"200", "202", "400", "401", "500", "503"})
	assertPathResponses(t, spec, "/v1/evidence/ingest/report", []string{"200", "202", "400", "401", "404", "500", "503"})

	assertSchemaOneOfRefs(t, spec, "IngestPrescribeRequest", []string{"IngestPrescribeTypedRequest", "IngestPrescribeOverrideRequest"})
	assertSchemaOneOfRefs(t, spec, "IngestReportRequest", []string{"IngestReportNonDeclinedRequest", "IngestReportDeclinedRequest", "IngestReportOverrideRequest"})
	assertSchemaExists(t, spec, "IngestPrescribeTypedRequest")
	assertSchemaExists(t, spec, "IngestPrescribeOverrideRequest")
	assertSchemaExists(t, spec, "IngestReportNonDeclinedRequest")
	assertSchemaExists(t, spec, "IngestReportDeclinedRequest")
	assertSchemaExists(t, spec, "IngestReportOverrideRequest")

	assertPrescribeExamples(t, spec)
	assertReportExamples(t, spec)
}

func loadOpenAPISpec(t *testing.T) *yaml.Node {
	t.Helper()

	path := filepath.Join("..", "..", "cmd", "evidra-api", "static", "openapi.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi spec: %v", err)
	}

	var spec yaml.Node
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse openapi spec: %v", err)
	}
	if len(spec.Content) == 0 {
		t.Fatal("openapi spec is empty")
	}
	return &spec
}

func assertPathResponses(t *testing.T, spec *yaml.Node, path string, want []string) {
	t.Helper()

	pathNode := findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path)
	postNode := findMappingValue(t, pathNode, "post")
	responses := findMappingValue(t, postNode, "responses")

	for _, code := range want {
		if findMappingValueOptional(responses, code) == nil {
			t.Fatalf("path %s missing response code %s", path, code)
		}
	}
}

func assertSchemaExists(t *testing.T, spec *yaml.Node, name string) {
	t.Helper()

	schemas := findMappingValue(t, findMappingValue(t, spec.Content[0], "components"), "schemas")
	if findMappingValueOptional(schemas, name) == nil {
		t.Fatalf("missing schema %s", name)
	}
}

func assertSchemaOneOfRefs(t *testing.T, spec *yaml.Node, name string, want []string) {
	t.Helper()

	schemas := findMappingValue(t, findMappingValue(t, spec.Content[0], "components"), "schemas")
	schema := findMappingValue(t, schemas, name)
	oneOf := findMappingValue(t, schema, "oneOf")
	if len(oneOf.Content) != len(want) {
		t.Fatalf("schema %s oneOf len = %d, want %d", name, len(oneOf.Content), len(want))
	}
	for i, ref := range want {
		item := oneOf.Content[i]
		got := findMappingValue(t, item, "$ref")
		if got.Value != "#/components/schemas/"+ref {
			t.Fatalf("schema %s oneOf[%d] ref = %q, want %q", name, i, got.Value, "#/components/schemas/"+ref)
		}
	}
}

func assertPrescribeExamples(t *testing.T, spec *yaml.Node) {
	t.Helper()

	examples := findOperationExamples(t, spec, "/v1/evidence/ingest/prescribe")
	typed := decodeExampleMap(t, findMappingValue(t, examples, "typed"))
	override := decodeExampleMap(t, findMappingValue(t, examples, "override"))

	if typed["payload_override"] != nil {
		t.Fatal("prescribe typed example should not set payload_override")
	}
	if _, ok := typed["canonical_action"]; !ok {
		t.Fatal("prescribe typed example missing canonical_action")
	}
	if _, ok := typed["smart_target"]; ok {
		t.Fatal("prescribe typed example should not set smart_target when canonical_action is present")
	}

	overrideBody := mustMap(t, override["payload_override"], "prescribe override payload_override")
	if _, ok := overrideBody["canonical_action"]; !ok {
		t.Fatal("prescribe override payload_override missing canonical_action")
	}
	if _, ok := override["canonical_action"]; ok {
		t.Fatal("prescribe override example should not set top-level canonical_action")
	}
	if _, ok := override["smart_target"]; ok {
		t.Fatal("prescribe override example should not set top-level smart_target")
	}
}

func assertReportExamples(t *testing.T, spec *yaml.Node) {
	t.Helper()

	examples := findOperationExamples(t, spec, "/v1/evidence/ingest/report")
	typed := decodeExampleMap(t, findMappingValue(t, examples, "typed"))
	override := decodeExampleMap(t, findMappingValue(t, examples, "override"))

	if typed["payload_override"] != nil {
		t.Fatal("report typed example should not set payload_override")
	}
	if _, ok := typed["prescription_id"]; !ok {
		t.Fatal("report typed example missing prescription_id")
	}
	if _, ok := typed["verdict"]; !ok {
		t.Fatal("report typed example missing verdict")
	}
	if typedVerdict, _ := typed["verdict"].(string); typedVerdict == "declined" {
		if _, ok := typed["decision_context"]; !ok {
			t.Fatal("declined report typed example missing decision_context")
		}
	} else {
		if _, ok := typed["exit_code"]; !ok {
			t.Fatal("non-declined report typed example missing exit_code")
		}
	}

	overrideBody := mustMap(t, override["payload_override"], "report override payload_override")
	if _, ok := overrideBody["prescription_id"]; !ok {
		t.Fatal("report override payload_override missing prescription_id")
	}
	if _, ok := overrideBody["verdict"]; !ok {
		t.Fatal("report override payload_override missing verdict")
	}
	if verdict, _ := overrideBody["verdict"].(string); verdict == "declined" {
		if _, ok := overrideBody["decision_context"]; !ok {
			t.Fatal("declined report override payload_override missing decision_context")
		}
		if _, ok := overrideBody["exit_code"]; ok {
			t.Fatal("declined report override payload_override should not set exit_code")
		}
	} else {
		if _, ok := overrideBody["exit_code"]; !ok {
			t.Fatal("non-declined report override payload_override missing exit_code")
		}
	}
	if _, ok := override["prescription_id"]; ok {
		t.Fatal("report override example should not set top-level prescription_id")
	}
	if _, ok := override["verdict"]; ok {
		t.Fatal("report override example should not set top-level verdict")
	}
	if _, ok := override["exit_code"]; ok {
		t.Fatal("report override example should not set top-level exit_code")
	}
	if _, ok := override["decision_context"]; ok {
		t.Fatal("report override example should not set top-level decision_context")
	}
}

func findOperationExamples(t *testing.T, spec *yaml.Node, path string) *yaml.Node {
	t.Helper()

	pathNode := findMappingValue(t, findMappingValue(t, spec.Content[0], "paths"), path)
	postNode := findMappingValue(t, pathNode, "post")
	requestBody := findMappingValue(t, postNode, "requestBody")
	content := findMappingValue(t, requestBody, "content")
	applicationJSON := findMappingValue(t, content, "application/json")
	return findMappingValue(t, applicationJSON, "examples")
}

func findMappingValue(t *testing.T, node *yaml.Node, key string) *yaml.Node {
	t.Helper()

	value := findMappingValueOptional(node, key)
	if value == nil {
		t.Fatalf("missing key %q", key)
	}
	return value
}

func findMappingValueOptional(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func decodeExampleMap(t *testing.T, node *yaml.Node) map[string]any {
	t.Helper()

	if node == nil {
		t.Fatal("example node is nil")
	}
	var out map[string]any
	if err := node.Decode(&out); err != nil {
		t.Fatalf("decode example: %v", err)
	}
	if value, ok := out["value"].(map[string]any); ok {
		return value
	}
	return out
}

func mustMap(t *testing.T, value any, context string) map[string]any {
	t.Helper()

	if value == nil {
		t.Fatalf("%s is nil", context)
	}
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want map[string]any", context, value)
	}
	return m
}

func TestOpenAPIIngestRoutesServeCurrentSpec(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "evidra-api", "static", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi spec: %v", err)
	}
	if !bytes.Contains(raw, []byte("/v1/evidence/ingest/prescribe")) || !bytes.Contains(raw, []byte("/v1/evidence/ingest/report")) {
		t.Fatal("openapi spec missing ingest paths")
	}
}
