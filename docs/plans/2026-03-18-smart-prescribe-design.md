# Smart Prescribe — Implementation Design

**Date:** 2026-03-18
**Status:** Proven in bench harness, ready for evidra implementation
**Target:** `pkg/mcpserver/server.go` in evidra repo

## Proof of Concept

Gemini 2.5 Flash — which scored 0% on full prescribe (can't format
the 10+ field schema) — follows smart prescribe perfectly on first try.
Tested in bench harness: `pkg/agent/smart_prescribe.go`.

## What Changes

The existing `evidra_prescribe` MCP tool accepts BOTH schemas.
Auto-detects based on which fields are present. No new tool name.

### Current (Full Prescribe)
```json
{
  "tool": "kubectl",
  "operation": "apply",
  "raw_artifact": "apiVersion: apps/v1\nkind: Deployment\n...(50+ lines)",
  "actor": {"type": "agent", "id": "bench", "origin": "mcp", "skill_version": "1.0.1"},
  "environment": "production",
  "scope_dimensions": {"cluster": "kind-evidra", "namespace": "bench"},
  "canonical_action": {"resource_identity": [...], "operation_class": "mutate"}
}
```
~200-500 tokens. Only Claude and GPT-5.2 can format this correctly.

### New (Smart Prescribe)
```json
{
  "tool": "kubectl",
  "operation": "apply",
  "resource": "deployment/web",
  "namespace": "bench"
}
```
~30 tokens. Any model can do this.

### Same Response
Both modes return the same response:
```json
{
  "ok": true,
  "prescription_id": "rx-01JQ...",
  "effective_risk": "medium",
  "risk_inputs": [...]
}
```

### Same Report
No change to `evidra_report` — works identically for both modes.

## Detection Logic

In `pkg/mcpserver/server.go`, the prescribe handler checks:

```go
func (s *Server) handlePrescribe(ctx context.Context, input PrescribeInput) (PrescribeOutput, error) {
    if input.RawArtifact != "" {
        // Full mode — existing path
        return s.fullPrescribe(ctx, input)
    }
    // Smart mode — infer what we can, skip what we can't
    return s.smartPrescribe(ctx, input)
}
```

## Smart Prescribe Implementation

```go
func (s *Server) smartPrescribe(ctx context.Context, input PrescribeInput) (PrescribeOutput, error) {
    // 1. Generate prescription ID (same as full mode)
    prescriptionID := generatePrescriptionID()

    // 2. Build canonical action from tool + operation
    opClass := classifyOperation(input.Tool, input.Operation) // mutate/destroy/read

    // 3. Build minimal scope from namespace
    scopeClass := "unknown"
    if input.Namespace != "" {
        scopeClass = resolveScopeFromNamespace(input.Namespace)
    }

    // 4. Risk assessment from tool + operation + scope (no artifact)
    risk := assessRiskFromOperation(input.Tool, input.Operation, opClass, scopeClass)
    // kubectl delete in production → high
    // kubectl apply in staging → medium
    // kubectl patch in development → low

    // 5. Write evidence entry (same format, fewer fields populated)
    entry := evidence.Entry{
        Type:           evidence.TypePrescription,
        PrescriptionID: prescriptionID,
        Tool:           input.Tool,
        Operation:      input.Operation,
        OperationClass: opClass,
        ScopeClass:     scopeClass,
        EffectiveRisk:  risk,
        // No artifact digest, no intent digest, no resource identity
        // These fields are empty but the entry is still valid
    }
    s.writeEntry(ctx, entry)

    // 6. Return same response shape
    return PrescribeOutput{
        OK:             true,
        PrescriptionID: prescriptionID,
        EffectiveRisk:  risk,
        Mode:           "smart", // new field — tells client which mode was used
    }, nil
}
```

## Tool Schema Update

The MCP tool definition needs updated parameter descriptions to show
both modes are accepted:

```json
{
  "name": "evidra_prescribe",
  "description": "Record intent BEFORE an infrastructure mutation. Two modes: send raw_artifact for full risk analysis, or just tool+operation+resource for lightweight recording.",
  "parameters": {
    "type": "object",
    "required": ["tool", "operation"],
    "properties": {
      "tool":             {"type": "string", "description": "Infrastructure tool (kubectl, helm, terraform)"},
      "operation":        {"type": "string", "description": "Operation (apply, delete, patch, upgrade)"},
      "resource":         {"type": "string", "description": "Target resource (e.g. deployment/web)"},
      "namespace":        {"type": "string", "description": "Kubernetes namespace"},
      "raw_artifact":     {"type": "string", "description": "Full YAML artifact (optional — enables drift detection)"},
      "actor":            {"type": "object", "description": "Actor metadata (optional)"},
      "scope_dimensions": {"type": "object", "description": "Scope metadata (optional)"}
    }
  }
}
```

Key: `raw_artifact` moves from required to optional. If present → full mode.
If absent → smart mode. Schema is backward compatible.

## Risk Assessment Without Artifact

Full mode analyzes the YAML artifact for risk. Smart mode infers risk
from tool + operation + scope:

```go
var operationRisk = map[string]map[string]string{
    "kubectl": {
        "delete": "high",
        "apply":  "medium",
        "patch":  "medium",
        "create": "low",
        "scale":  "low",
    },
    "helm": {
        "uninstall": "high",
        "upgrade":   "medium",
        "install":   "medium",
        "rollback":  "medium",
    },
    "terraform": {
        "destroy": "critical",
        "apply":   "high",
        "import":  "medium",
    },
}

func assessRiskFromOperation(tool, operation, opClass, scopeClass string) string {
    // Start with operation-based risk
    risk := operationRisk[tool][operation]
    if risk == "" {
        risk = "medium"
    }
    // Elevate for production scope
    if scopeClass == "production" && risk == "medium" {
        risk = "high"
    }
    return risk
}
```

## Scorecard Compatibility

Which signals work with smart prescribe evidence?

| Signal | Full | Smart | Why |
|--------|------|-------|-----|
| protocol_violation | Yes | Yes | Checks prescribe/report pairing |
| retry_loop | Yes | Yes | Same tool+operation repeated |
| repair_loop | Yes | Yes | Delete+create patterns |
| thrashing | Yes | Yes | Rapid apply/delete cycles |
| blast_radius | Yes | Partial | No resource count from artifact |
| artifact_drift | Yes | No | No artifact hash to compare |
| risk_escalation | Yes | Yes | Risk level tracked per prescribe |
| new_scope | Yes | Yes | Namespace/scope tracked |

6 of 8 signals work fully. 1 partial, 1 missing. Good enough for
most use cases. Full mode only needed for artifact drift detection.

## Files to Modify

```
pkg/mcpserver/server.go          — add smart prescribe path
pkg/mcpserver/server_test.go     — test both modes
pkg/execcontract/contracts.go    — make raw_artifact optional in schema
pkg/execcontract/schemas/prescribe.schema.json — update JSON schema
```

## Skill Prompt Update

The evidra contract skill needs a note about smart prescribe:

```markdown
## Smart Prescribe (recommended)

For each infrastructure mutation, call prescribe with:
- tool: the CLI tool (kubectl, helm, terraform)
- operation: the subcommand (apply, delete, patch)
- resource: target resource (deployment/web, configmap/app)
- namespace: the namespace

You do NOT need to send the full YAML artifact.
```

## Migration

1. Implement in `pkg/mcpserver/server.go` — auto-detect mode
2. Update JSON schema — make raw_artifact optional
3. Update skill prompt — recommend smart mode
4. Test with bench harness — verify Gemini Flash compliance
5. Update docs — explain both modes
6. Release — backward compatible, existing agents work unchanged

## Testing Plan

```bash
# Smart mode — new
infra-bench run --scenario missing-configmap --smart-prescribe --model gemini-2.5-flash

# Full mode — existing, must still work
infra-bench run --scenario missing-configmap --system-prompt-file contract.md --evidra-bin evidra --model gpt-5.2

# Proxy mode — existing, must still work
infra-bench run --scenario missing-configmap --proxy-mode --model qwen-plus
```

All three modes coexist. The evidra binary auto-detects. No breaking changes.
