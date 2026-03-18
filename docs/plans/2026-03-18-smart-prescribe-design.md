# Smart Prescribe Tool Split — Implementation Design

**Date:** 2026-03-18
**Status:** Approved for implementation
**Target:** direct MCP mode in `pkg/mcpserver`, prompt contract generation in `prompts/`, shared lifecycle path unchanged

## Problem

The current direct-mode MCP surface exposes one overloaded `prescribe` tool
that accepts both full-artifact and smart/lightweight inputs. That keeps the
backend simple, but it is the wrong shape for model-facing prompts:

- full mode and smart mode have different goals
- full mode should ask for artifact bytes and detector-oriented context
- smart mode should stay lightweight and target-oriented
- one blended tool description forces the model to choose a payload shape
  inside the prompt, which increases ambiguity and invalid mixed payloads

The skill prompt already wants to explain these as different workflows. The MCP
tool surface should match that.

## Decision

Split the overloaded direct prescribe surface into two MCP tools:

- `prescribe_full`
- `prescribe_smart`

Keep `report` and `get_event` unchanged.

Both prescribe tools route into the same internal lifecycle service and produce
the same output shape, so evidence persistence, scoring, retry tracking, and
report semantics stay unified.

## Non-Goals

- No change to the `report` protocol
- No change to evidence entry schemas
- No change to proxy mode semantics
- No attempt to keep the old `prescribe` tool visible in the default MCP
  surface; doing so would preserve the model-selection ambiguity we are trying
  to remove

## API Shape

### `prescribe_full`

Purpose: artifact-aware direct mode for agents that have the real mutation
artifact and want native detector coverage plus artifact drift detection.

Required fields:

- `tool`
- `operation`
- `raw_artifact`
- `actor.type`
- `actor.id`
- `actor.origin`

Optional fields:

- `canonical_action`
- `environment`
- `scope_dimensions`
- correlation identifiers

Example:

```json
{
  "tool": "kubectl",
  "operation": "apply",
  "raw_artifact": "apiVersion: apps/v1\nkind: Deployment\n...",
  "actor": {
    "type": "agent",
    "id": "bench",
    "origin": "mcp-stdio",
    "skill_version": "1.1.0"
  }
}
```

### `prescribe_smart`

Purpose: lightweight direct mode for agents that know the target operation and
resource but do not have artifact bytes.

Required fields:

- `tool`
- `operation`
- `resource`
- `actor.type`
- `actor.id`
- `actor.origin`

Optional fields:

- `namespace`
- `environment`
- `scope_dimensions`
- correlation identifiers

Example:

```json
{
  "tool": "kubectl",
  "operation": "apply",
  "resource": "deployment/web",
  "namespace": "default",
  "actor": {
    "type": "agent",
    "id": "bench",
    "origin": "mcp-stdio",
    "skill_version": "1.1.0"
  }
}
```

### Shared Response

Both tools return the current `PrescribeOutput` shape:

- `ok`
- `prescription_id`
- `risk_inputs`
- `effective_risk`
- `artifact_digest`
- `intent_digest`
- `resource_shape_hash`
- `operation_class`
- `scope_class`

Full mode will usually populate artifact-derived fields more richly. Smart mode
may leave artifact-specific outputs empty while still returning a valid
prescription.

## Backend Architecture

The MCP layer changes; the lifecycle service does not.

Implementation shape:

1. `prescribe_full` handler validates the full schema and forwards the request
   through the existing artifact-aware path.
2. `prescribe_smart` handler validates the smart schema, synthesizes a minimal
   canonical action from `tool`, `operation`, `resource`, `namespace`,
   `environment`, and `scope_dimensions`, and then forwards that normalized
   request through the same lifecycle service.
3. `report` continues to consume `prescription_id` only; it does not need to
   know which prescribe tool produced it.

This keeps storage, scoring, correlation, and retry semantics on one code path.

## Shared Normalization Rules

### Full Mode

- `raw_artifact` remains required
- if `canonical_action` is absent, existing canonicalization rules apply
- native detector and artifact drift signals remain available

### Smart Mode

- `resource` is required
- `namespace` is optional input but should be copied into canonical scope when
  present
- the adapter synthesizes a minimal `canon.CanonicalAction`
- risk evaluation is matrix-driven when no artifact is present
- artifact drift is unavailable by design

## Tool Registration

`pkg/mcpserver.NewServer` should register these tools:

- `prescribe_full`
- `prescribe_smart`
- `report`
- `get_event`

The previous overloaded `prescribe` tool should be removed from the default MCP
tool list so the model is forced to choose between the explicit full and smart
surfaces.

## Contract And Prompt Versioning

Do not rewrite `v1.0.1` in place.

The current prompt contract version is already embedded in generated assets,
tests, manifests, and `actor.skill_version` guidance. Changing tool semantics
under the same version would make historical prompt provenance misleading.

Instead:

- freeze `prompts/source/contracts/v1.0.1` as-is
- add a new contract version `v1.1.0`
- move the default prompt contract version to `v1.1.0`
- update skill examples to use `skill_version: "1.1.0"`

## Prompt Generation

The prompt factory must generate separate MCP tool descriptions:

- `prompts/generated/v1.1.0/mcpserver/tools/prescribe_full_description.txt`
- `prompts/generated/v1.1.0/mcpserver/tools/prescribe_smart_description.txt`
- active copies under `prompts/mcpserver/tools/`

This requires:

- new contract sections for `mcp.prescribe_full` and `mcp.prescribe_smart`
- new templates `templates/mcp/prescribe_full.tmpl` and
  `templates/mcp/prescribe_smart.tmpl`
- prompt embed constants for both new files
- updated manifest generation and prompt verification

The skill prompt remains one document, but it should explain the two direct
tools as intentionally different workflows, not two shapes of one tool.

## Documentation Rollout

Public docs should explain three evidence modes:

- direct full: `prescribe_full`
- direct smart: `prescribe_smart`
- proxy mode

The docs must make the tradeoff explicit:

- use `prescribe_full` when you have artifact bytes and want detector coverage
- use `prescribe_smart` when you need a lightweight direct call
- use proxy mode when you want command interception instead of direct protocol
  calls

## Testing Strategy

Required coverage:

- execcontract schema/validation tests for both tool shapes
- prompt embed tests for both MCP descriptions and updated skill guidance
- MCP server tests that list the new tool set
- end-to-end tests proving:
  - `prescribe_full` preserves artifact-aware behavior
  - `prescribe_smart` produces a valid prescription and report lifecycle
  - `report` works identically for both
- prompt generation and manifest verification
- repo lint and targeted doc guard scripts after docs updates

## Migration Notes

This is an intentional MCP tool-surface change. Existing prompts and direct MCP
callers that use `prescribe` must migrate to either `prescribe_full` or
`prescribe_smart`.

That is acceptable because the split is the whole point of the redesign: one
tool per workflow, one prompt per workflow, one shared backend.
