# MCP Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add structured output schemas to all MCP tools, resource links in `report` results, a scorecard resource, and the registry description field.

**Architecture:** Single PR on top of go-sdk v1.5.0. All changes are in `pkg/mcpserver/`. Output schemas are hand-authored JSON files embedded via Go's `//go:embed`. Resource handlers follow the existing `readResourceEvent` pattern already in `server.go`. No new packages.

**Tech Stack:** Go 1.24, `github.com/modelcontextprotocol/go-sdk/mcp` v1.5.0, stdlib `encoding/json`, `//go:embed`

**Design spec:** `docs/superpowers/specs/2026-04-13-mcp-features-design.md`

---

## Prerequisites

Before starting: merge PR #20 (`go-sdk` 1.4.1 → 1.5.0) and run `go test ./...` to confirm a clean baseline.

---

## File map

| File | Change |
|------|--------|
| `pkg/mcpserver/schemas/prescribe.output.schema.json` | **Create** — output schema for `prescribe_full` and `prescribe_smart` |
| `pkg/mcpserver/schemas/report.output.schema.json` | **Create** — output schema for `report` |
| `pkg/mcpserver/schemas/collect_diagnostics.output.schema.json` | **Create** — output schema for `collect_diagnostics` |
| `pkg/mcpserver/schema_embed.go` | **Modify** — embed 3 new schema files |
| `pkg/mcpserver/deferred_protocol_tools.go` | **Modify** — wire `OutputSchema` to `prescribe_smart` and `report`; add resource link to report result |
| `pkg/mcpserver/server.go` | **Modify** — wire `OutputSchema` to `prescribe_full`; add scorecard resource handlers; add `Description` to `Implementation` |
| `pkg/mcpserver/collect_diagnostics.go` | **Modify** — wire `OutputSchema` to `collect_diagnostics` |
| `pkg/mcpserver/server_test.go` | **Modify** — update deferred schema test; add scorecard resource test |

---

## Task 1: Output schema JSON files

**Files:**
- Create: `pkg/mcpserver/schemas/prescribe.output.schema.json`
- Create: `pkg/mcpserver/schemas/report.output.schema.json`
- Create: `pkg/mcpserver/schemas/collect_diagnostics.output.schema.json`

- [ ] **Step 1: Write the failing test**

Add this test to `pkg/mcpserver/server_test.go` (after the existing `TestRunCommandTool_HasOutputSchemaAndExamples` test):

```go
func TestAllTools_HaveOutputSchema(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Options{
		Name:         "test",
		Version:      "0.0.1",
		EvidencePath: t.TempDir(),
		Signer:       testutil.TestSigner(t),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	toolDefs := make(map[string]*mcp.Tool, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolDefs[tool.Name] = tool
	}

	for _, name := range []string{"prescribe_smart", "report", "run_command", "collect_diagnostics"} {
		tool := toolDefs[name]
		if tool == nil {
			t.Fatalf("tool %q missing from tools/list", name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %q is missing OutputSchema", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test -run TestAllTools_HaveOutputSchema ./pkg/mcpserver/ -v -count=1
```

Expected: FAIL — `tool "prescribe_smart" is missing OutputSchema` and `tool "collect_diagnostics" is missing OutputSchema`

- [ ] **Step 3: Also update the existing deferred protocol test**

In `TestNewServer_DefaultToolSurfaceUsesDeferredProtocolSchemas`, find the block that checks `prescribe_smart` and `report` have no output schema (around line 271) and invert it:

```go
// Old — delete these lines:
if tool.OutputSchema != nil {
    t.Fatalf("%s should not advertise an output schema by default", name)
}

// New — replace with:
if tool.OutputSchema == nil {
    t.Fatalf("%s should advertise an output schema", name)
}
```

- [ ] **Step 4: Create `schemas/prescribe.output.schema.json`**

```json
{
  "type": "object",
  "required": ["ok", "prescription_id", "artifact_digest", "intent_digest", "resource_shape_hash", "resource_count", "operation_class", "scope_class", "canon_version"],
  "properties": {
    "ok": { "type": "boolean" },
    "prescription_id": { "type": "string" },
    "risk_inputs": {
      "type": "array",
      "items": { "type": "object" }
    },
    "effective_risk": { "type": "string" },
    "artifact_digest": { "type": "string" },
    "intent_digest": { "type": "string" },
    "resource_shape_hash": { "type": "string" },
    "resource_count": { "type": "integer" },
    "operation_class": { "type": "string" },
    "scope_class": { "type": "string" },
    "canon_version": { "type": "string" },
    "retry_count": { "type": "integer" },
    "error": {
      "type": "object",
      "required": ["code", "message"],
      "properties": {
        "code": { "type": "string" },
        "message": { "type": "string" }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 5: Create `schemas/report.output.schema.json`**

```json
{
  "type": "object",
  "required": ["ok", "report_id", "prescription_id", "verdict", "score", "score_band", "scoring_profile_id", "signal_summary", "basis", "confidence"],
  "properties": {
    "ok": { "type": "boolean" },
    "report_id": { "type": "string" },
    "prescription_id": { "type": "string" },
    "exit_code": { "type": "integer" },
    "verdict": { "type": "string" },
    "decision_context": { "type": "object" },
    "score": { "type": "number" },
    "score_band": { "type": "string" },
    "scoring_profile_id": { "type": "string" },
    "signal_summary": {
      "type": "object",
      "additionalProperties": { "type": "integer" }
    },
    "basis": {
      "type": "object",
      "properties": {
        "assessment_mode": { "type": "string" },
        "sufficient": { "type": "boolean" },
        "total_operations": { "type": "integer" },
        "sufficient_threshold": { "type": "integer" },
        "preview_min_operations": { "type": "integer" }
      },
      "additionalProperties": false
    },
    "confidence": { "type": "string" },
    "error": {
      "type": "object",
      "required": ["code", "message"],
      "properties": {
        "code": { "type": "string" },
        "message": { "type": "string" }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}
```

- [ ] **Step 6: Create `schemas/collect_diagnostics.output.schema.json`**

```json
{
  "type": "object",
  "required": ["ok", "summary", "commands"],
  "properties": {
    "ok": { "type": "boolean" },
    "summary": { "type": "string" },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["source", "summary"],
        "properties": {
          "source": { "type": "string" },
          "summary": { "type": "string" }
        },
        "additionalProperties": false
      }
    },
    "commands": {
      "type": "array",
      "items": { "type": "string" }
    },
    "error": { "type": "string" }
  },
  "additionalProperties": false
}
```

- [ ] **Step 7: Embed the new schema files in `schema_embed.go`**

Add three embed declarations to `pkg/mcpserver/schema_embed.go`:

```go
//go:embed schemas/prescribe.output.schema.json
var prescribeOutputSchemaBytes []byte

//go:embed schemas/report.output.schema.json
var reportOutputSchemaBytes []byte

//go:embed schemas/collect_diagnostics.output.schema.json
var collectDiagnosticsOutputSchemaBytes []byte
```

The full file becomes:

```go
package mcpserver

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed schemas/get_event.schema.json
var getEventSchemaBytes []byte

//go:embed schemas/get_event.output.schema.json
var getEventOutputSchemaBytes []byte

//go:embed schemas/run_command.output.schema.json
var runCommandOutputSchemaBytes []byte

//go:embed schemas/prescribe.output.schema.json
var prescribeOutputSchemaBytes []byte

//go:embed schemas/report.output.schema.json
var reportOutputSchemaBytes []byte

//go:embed schemas/collect_diagnostics.output.schema.json
var collectDiagnosticsOutputSchemaBytes []byte

func loadSchema(raw []byte, name string) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse embedded MCP schema %s: %w", name, err)
	}
	return schema, nil
}
```

- [ ] **Step 8: Wire OutputSchema to `prescribe_smart` and `report` in `deferred_protocol_tools.go`**

Load both output schemas at the top of `registerDeferredProtocolTools`, then add `OutputSchema` to each tool definition.

Replace the current function with:

```go
func registerDeferredProtocolTools(server *mcp.Server, svc *MCPService, registry *toolRegistry) error {
	prescribeOutputSchema, err := loadSchema(prescribeOutputSchemaBytes, "schemas/prescribe.output.schema.json")
	if err != nil {
		return err
	}
	reportOutputSchema, err := loadSchema(reportOutputSchemaBytes, "schemas/report.output.schema.json")
	if err != nil {
		return err
	}

	smartDef, err := execcontract.PrescribeSmartToolDefinition()
	if err != nil {
		return err
	}
	registry.register("prescribe_smart", toolEntry{
		description: smartDef.Description,
		inputSchema: smartDef.Parameters,
	})
	server.AddTool(&mcp.Tool{
		Name:        "prescribe_smart",
		Title:       "Record Smart Infrastructure Intent",
		Description: smartDef.Description + " Use describe_tool for the full schema if you need explicit control.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Prescribe Smart",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema:  minimalObjectSchema,
		OutputSchema: prescribeOutputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input PrescribeSmartInput
		if err := decodeDeferredInput(req, &input); err != nil {
			return nil, err
		}
		out := svc.PrescribeCtx(ctx, input.toPrescribeInput())
		return structuredToolResult(out)
	})

	reportDef, err := execcontract.ReportToolDefinition()
	if err != nil {
		return err
	}
	registry.register("report", toolEntry{
		description: reportDef.Description,
		inputSchema: reportDef.Parameters,
	})
	server.AddTool(&mcp.Tool{
		Name:        "report",
		Title:       "Report Operation Result",
		Description: reportDef.Description + " Use describe_tool for the full schema if you need explicit control.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Report",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema:  minimalObjectSchema,
		OutputSchema: reportOutputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input ReportInput
		if err := decodeDeferredInput(req, &input); err != nil {
			return nil, err
		}
		out := svc.ReportCtx(ctx, input)
		return structuredToolResult(out)
	})

	return nil
}
```

- [ ] **Step 9: Wire OutputSchema to `prescribe_full` in `server.go`**

In `NewServerWithCleanup`, load the prescribe output schema once (reuse `prescribeOutputSchemaBytes`) and add `OutputSchema` to the `prescribe_full` tool definition. Find the `prescribe_full` tool registration block and update it:

```go
prescribeOutputSchema, err := loadSchema(prescribeOutputSchemaBytes, "schemas/prescribe.output.schema.json")
if err != nil {
    return nil, nil, err
}

// ... (prescribeFullDef loading is already there) ...

mcp.AddTool(server, &mcp.Tool{
    Name:        "prescribe_full",
    Title:       "Record Full Infrastructure Intent",
    Description: prescribeFullDef.Description,
    Annotations: &mcp.ToolAnnotations{
        Title:           "Prescribe Full",
        ReadOnlyHint:    false,
        IdempotentHint:  false,
        DestructiveHint: boolPtr(false),
        OpenWorldHint:   boolPtr(false),
    },
    InputSchema:  prescribeFullDef.Parameters,
    OutputSchema: prescribeOutputSchema,
}, prescribeFull.Handle)
```

- [ ] **Step 10: Wire OutputSchema to `collect_diagnostics` in `collect_diagnostics.go`**

In `RegisterCollectDiagnostics`, load the schema and add `OutputSchema`:

```go
func RegisterCollectDiagnostics(server *mcp.Server, svc *MCPService, kubeconfigPath string, actorID string) {
	outputSchema, err := loadSchema(collectDiagnosticsOutputSchemaBytes, "schemas/collect_diagnostics.output.schema.json")
	if err != nil {
		panic(err) // same pattern as runCommandOutputSchema()
	}

	runCommand := &runCommandHandler{
		service:         svc,
		kubeconfigPath:  kubeconfigPath,
		actorID:         actorID,
		allowedPrefixes: defaultAllowedPrefixes,
		blockedSubs:     defaultBlockedSubcommands,
	}
	handler := &collectDiagnosticsHandler{
		run: func(ctx context.Context, command string) RunCommandOutput {
			return runCommand.execute(ctx, RunCommandInput{Command: command})
		},
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "collect_diagnostics",
		Title:       "Collect Kubernetes Diagnostics",
		Description: "Run a fixed Kubernetes diagnosis sequence for one workload: get pods, describe the workload, inspect recent events, and fetch logs when a failing pod needs more context.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Collect Diagnostics",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(true),
		},
		InputSchema:  collectDiagnosticsInputSchema,
		OutputSchema: outputSchema,
	}, handler.Handle)
}
```

- [ ] **Step 11: Run tests to verify they pass**

```
go test -run "TestAllTools_HaveOutputSchema|TestNewServer_DefaultToolSurfaceUsesDeferredProtocolSchemas" ./pkg/mcpserver/ -v -count=1
```

Expected: both PASS

- [ ] **Step 12: Run full package tests**

```
go test ./pkg/mcpserver/ -v -count=1
```

Expected: all PASS

- [ ] **Step 13: Format**

```
gofmt -w pkg/mcpserver/
```

- [ ] **Step 14: Commit**

```bash
git add pkg/mcpserver/schemas/prescribe.output.schema.json \
        pkg/mcpserver/schemas/report.output.schema.json \
        pkg/mcpserver/schemas/collect_diagnostics.output.schema.json \
        pkg/mcpserver/schema_embed.go \
        pkg/mcpserver/deferred_protocol_tools.go \
        pkg/mcpserver/server.go \
        pkg/mcpserver/collect_diagnostics.go \
        pkg/mcpserver/server_test.go
git commit -m "$(cat <<'EOF'
feat(mcp): add outputSchema declarations to all tools

prescribe_smart, report, prescribe_full, and collect_diagnostics now
advertise output schemas so agents receive validated, schema-declared
JSON in structuredContent.

Signed-off-by: Vitaliy Ryumshyn <vitas@samebits.com>
EOF
)"
```

---

## Task 2: Resource link in `report` output

**Files:**
- Modify: `pkg/mcpserver/deferred_protocol_tools.go`
- Modify: `pkg/mcpserver/server_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/mcpserver/server_test.go`:

```go
func TestReportTool_IncludesResourceLinkOnSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := &MCPService{
		evidencePath: dir,
		signer:       testutil.TestSigner(t),
	}
	svc.lifecycle = svc.newLifecycleService()

	// First prescribe so we have a valid prescription_id.
	presc := svc.Prescribe(PrescribeInput{
		Actor:       InputActor{Type: "agent", ID: "test", Origin: "mcp"},
		Tool:        "kubectl",
		Operation:   "apply",
		RawArtifact: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: prod",
	})
	if !presc.OK {
		t.Fatalf("prescribe: %v", presc.Error)
	}

	server, err := NewServer(Options{
		EvidencePath: dir,
		Signer:       testutil.TestSigner(t),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	input, _ := json.Marshal(ReportInput{
		PrescriptionID: presc.PrescriptionID,
		Verdict:        evidence.VerdictSuccess,
		Actor:          InputActor{Type: "agent", ID: "test", Origin: "mcp"},
	})
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "report",
		Arguments: input,
	})
	if err != nil {
		t.Fatalf("CallTool report: %v", err)
	}

	var hasResourceLink bool
	for _, c := range result.Content {
		if rl, ok := c.(*mcp.ResourceLink); ok {
			if strings.HasPrefix(rl.URI, "evidra://event/") {
				hasResourceLink = true
			}
		}
	}
	if !hasResourceLink {
		t.Fatalf("report result missing resource_link to evidra://event/; content: %v", result.Content)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test -run TestReportTool_IncludesResourceLinkOnSuccess ./pkg/mcpserver/ -v -count=1
```

Expected: FAIL — no resource link in content

- [ ] **Step 3: Modify the report handler in `deferred_protocol_tools.go`**

Replace the report handler function (the anonymous func passed to `server.AddTool`) with:

```go
func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    var input ReportInput
    if err := decodeDeferredInput(req, &input); err != nil {
        return nil, err
    }
    out := svc.ReportCtx(ctx, input)
    result, err := structuredToolResult(out)
    if err != nil {
        return nil, err
    }
    if out.OK && out.ReportID != "" {
        result.Content = append(result.Content, &mcp.ResourceLink{
            URI:      "evidra://event/" + out.ReportID,
            Name:     out.ReportID,
            Title:    "Evidence Entry",
            MIMEType: "application/json",
        })
    }
    return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test -run TestReportTool_IncludesResourceLinkOnSuccess ./pkg/mcpserver/ -v -count=1
```

Expected: PASS

- [ ] **Step 5: Run full package tests**

```
go test ./pkg/mcpserver/ -v -count=1
```

Expected: all PASS

- [ ] **Step 6: Format**

```
gofmt -w pkg/mcpserver/deferred_protocol_tools.go pkg/mcpserver/server_test.go
```

- [ ] **Step 7: Commit**

```bash
git add pkg/mcpserver/deferred_protocol_tools.go pkg/mcpserver/server_test.go
git commit -m "$(cat <<'EOF'
feat(mcp): include resource_link in report tool result

After a successful report, the tool result now includes a resource_link
pointing to evidra://event/{report_id} so agents can follow directly to
the evidence entry without a separate get_event call.

Signed-off-by: Vitaliy Ryumshyn <vitas@samebits.com>
EOF
)"
```

---

## Task 3: Scorecard resource

**Files:**
- Modify: `pkg/mcpserver/server.go`
- Modify: `pkg/mcpserver/server_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/mcpserver/server_test.go`:

```go
func TestScorecardResource_ReturnsSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := &MCPService{
		evidencePath: dir,
		signer:       testutil.TestSigner(t),
	}
	svc.lifecycle = svc.newLifecycleService()

	// Write a prescribe+report pair so there is evidence to score.
	presc := svc.Prescribe(PrescribeInput{
		Actor:     InputActor{Type: "agent", ID: "test", Origin: "mcp"},
		Tool:      "kubectl",
		Operation: "apply",
		RawArtifact: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: prod",
	})
	if !presc.OK {
		t.Fatalf("prescribe: %v", presc.Error)
	}
	ec := 0
	report := svc.Report(ReportInput{
		PrescriptionID: presc.PrescriptionID,
		Verdict:        evidence.VerdictSuccess,
		ExitCode:       &ec,
		Actor:          InputActor{Type: "agent", ID: "test", Origin: "mcp"},
	})
	if !report.OK {
		t.Fatalf("report: %v", report.Error)
	}

	server, err := NewServer(Options{
		EvidencePath: dir,
		Signer:       testutil.TestSigner(t),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "evidra://scorecard/current",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Fatal("scorecard resource returned empty contents")
	}
	var snapshot map[string]any
	if len(result.Contents) == 0 || result.Contents[0].Text == "" {
		t.Fatal("scorecard resource returned empty text content")
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &snapshot); err != nil {
		t.Fatalf("unmarshal scorecard: %v", err)
	}
	if _, ok := snapshot["score"]; !ok {
		t.Errorf("scorecard missing 'score' field: %v", snapshot)
	}
	if _, ok := snapshot["score_band"]; !ok {
		t.Errorf("scorecard missing 'score_band' field: %v", snapshot)
	}
}

func TestScorecardResource_NoEvidenceReturnsZeroSnapshot(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Options{
		EvidencePath: t.TempDir(),
		Signer:       testutil.TestSigner(t),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Wait() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "evidra://scorecard/current",
	})
	if err != nil {
		t.Fatalf("ReadResource with empty evidence dir: %v", err)
	}
	if len(result.Contents) == 0 || result.Contents[0].Text == "" {
		t.Fatal("expected non-empty scorecard even with no evidence")
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &snapshot); err != nil {
		t.Fatalf("unmarshal zero scorecard: %v", err)
	}
	// With no evidence, score should be 0.
	if score, ok := snapshot["score"]; !ok || score.(float64) != 0 {
		t.Errorf("expected score=0 with no evidence, got %v", snapshot["score"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test -run "TestScorecardResource" ./pkg/mcpserver/ -v -count=1
```

Expected: FAIL — resource not found

- [ ] **Step 3: Add `readResourceScorecard` to `MCPService` in `server.go`**

Add this method after `readResourceManifest`:

```go
func (s *MCPService) readResourceScorecard(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if s.evidencePath == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	sessionID := strings.TrimPrefix(req.Params.URI, "evidra://scorecard/")
	if sessionID == req.Params.URI {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	if sessionID == "current" {
		sessionID = ""
	}
	snapshot, err := s.sessionSnapshot(sessionID)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(b),
		}},
	}, nil
}
```

- [ ] **Step 4: Register scorecard resources in `NewServerWithCleanup`**

After the existing `server.AddResource` call for `evidra-evidence-manifest`, add:

```go
server.AddResource(&mcp.Resource{
    Name:        "evidra-scorecard-current",
    Title:       "Current Scorecard",
    Description: "Current assessment snapshot — score, score_band, signal_summary, and confidence for this evidence session.",
    MIMEType:    "application/json",
    URI:         "evidra://scorecard/current",
}, svc.readResourceScorecard)
server.AddResourceTemplate(&mcp.ResourceTemplate{
    Name:        "evidra-scorecard",
    Title:       "Scorecard by Session",
    Description: "Assessment snapshot for a specific session ID.",
    MIMEType:    "application/json",
    URITemplate: "evidra://scorecard/{session_id}",
}, svc.readResourceScorecard)
```

- [ ] **Step 5: Run tests to verify they pass**

```
go test -run "TestScorecardResource" ./pkg/mcpserver/ -v -count=1
```

Expected: both PASS

- [ ] **Step 6: Run full package tests**

```
go test ./pkg/mcpserver/ -v -count=1
```

Expected: all PASS

- [ ] **Step 7: Format**

```
gofmt -w pkg/mcpserver/server.go pkg/mcpserver/server_test.go
```

- [ ] **Step 8: Commit**

```bash
git add pkg/mcpserver/server.go pkg/mcpserver/server_test.go
git commit -m "$(cat <<'EOF'
feat(mcp): expose scorecard as MCP resource

Adds evidra://scorecard/current (static) and evidra://scorecard/{session_id}
(template) resources. Agents can read the current assessment snapshot —
score, score_band, signal_summary, confidence — without a tool call.

Signed-off-by: Vitaliy Ryumshyn <vitas@samebits.com>
EOF
)"
```

---

## Task 4: Implementation description field

**Files:**
- Modify: `pkg/mcpserver/server.go`

- [ ] **Step 1: Update `NewServerWithCleanup`**

Find the `mcp.NewServer` call and add `Description`:

```go
server := mcp.NewServer(
    &mcp.Implementation{
        Name:        opts.Name,
        Version:     opts.Version,
        Description: "Flight recorder for AI infrastructure agents",
    },
    &mcp.ServerOptions{
        Instructions: initializeInstructions,
    },
)
```

- [ ] **Step 2: Run full tests**

```
go test ./pkg/mcpserver/ -v -count=1
```

Expected: all PASS (no test needs updating — this is additive metadata)

- [ ] **Step 3: Format and commit**

```bash
gofmt -w pkg/mcpserver/server.go
git add pkg/mcpserver/server.go
git commit -m "$(cat <<'EOF'
feat(mcp): add description to MCP server Implementation

Makes Evidra discoverable in the MCP Registry with a human-readable
description field.

Signed-off-by: Vitaliy Ryumshyn <vitas@samebits.com>
EOF
)"
```

---

## Final verification

- [ ] **Run all tests in the repo**

```
go test ./... -count=1
```

Expected: all PASS

- [ ] **Run race detector**

```
go test -race ./pkg/mcpserver/ -count=1
```

Expected: no race conditions detected

- [ ] **Tidy**

```
go mod tidy
```
