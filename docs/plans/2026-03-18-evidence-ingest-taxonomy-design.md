# Evidence Ingest Taxonomy Cleanup Design

**Date:** 2026-03-18
**Status:** Approved for implementation
**Target:** `pkg/evidence`, `internal/lifecycle`, `internal/automationevent`, CLI/MCP adapter edges, and the normative protocol docs

## Goal

Clean up Evidra's evidence taxonomy before the AgentGateway and external-adapter
demo work lands.

The current model overloads `payload.flavor` and uses it as the only explicit
execution context field. That is enough for today's direct and Argo CD flows,
but it will get messy once Evidra starts ingesting external telemetry from
systems such as AgentGateway and later adapters such as Argo CD, CI systems, or
other controllers.

This change keeps one lifecycle (`prescribe` / `report`) but splits execution
shape from evidence acquisition shape.

## Problem

Today the protocol says:

- use `payload.flavor` to distinguish `imperative`, `reconcile`, and
  `pipeline_stage`
- keep one scoring lane across all of them

That part is still directionally right, but two issues are now visible:

1. `pipeline_stage` is poor product wording
2. `flavor` is being asked to answer two different questions:
   - what kind of execution was this?
   - how did Evidra learn about it?

That ambiguity is already visible in code:

- direct lifecycle writes do not carry explicit typed metadata beyond the raw
  payload
- `flavor` is injected into mapped webhook payloads as loose JSON instead of a
  typed field
- there is no first-class axis for `declared` versus `observed` versus
  `translated` evidence

If left as-is, AgentGateway and future adapters will either:

- pretend observed telemetry is the same thing as direct declaration, or
- overload `flavor` further and turn it into a junk drawer

## Decision

Keep the single `prescribe` / `report` lifecycle, but split the taxonomy into
three explicit payload dimensions:

- `flavor`
- `evidence.kind`
- `source.system`

### `flavor`

`flavor` remains the execution-shape dimension only:

- `imperative`
- `reconcile`
- `workflow`

`workflow` replaces `pipeline_stage`.

### `evidence.kind`

`evidence.kind` describes how Evidra obtained the lifecycle evidence:

- `declared`
- `observed`
- `translated`

Semantics:

- `declared`: the caller intentionally registered the lifecycle with Evidra
  through a native surface such as CLI or MCP
- `observed`: Evidra inferred or recorded the lifecycle from observed runtime
  behavior or telemetry rather than an explicit lifecycle declaration
- `translated`: another system's lifecycle or controller events were mapped into
  Evidra's `prescribe` / `report` model

### `source.system`

`source.system` identifies the adapter or upstream system that produced the
evidence:

- `cli`
- `mcp`
- `argocd`
- `agentgateway`
- future values as needed

The combination is the important part:

- MCP direct mode: `flavor=imperative`, `evidence.kind=declared`,
  `source.system=mcp`
- CLI direct mode: `flavor=imperative`, `evidence.kind=declared`,
  `source.system=cli`
- Argo CD mapped controller events: `flavor=reconcile`,
  `evidence.kind=translated`, `source.system=argocd`
- future AgentGateway bridge: `flavor=imperative`,
  `evidence.kind=observed`, `source.system=agentgateway`

## Non-Goals

- No new primary lifecycle entry types
- No change to score semantics or signal math
- No attempt to implement the AgentGateway bridge in this branch
- No change to the MCP prompt contract surface for this taxonomy cleanup
- No forced migration of historical evidence files

This change is a prerequisite cleanup for later ingest/demo work, not the bridge
itself.

## Why This Is Better

This split keeps the protocol agnostic and product-clean:

- `flavor` answers "what kind of execution is this?"
- `evidence.kind` answers "how did Evidra get this evidence?"
- `source.system` answers "which system produced it?"

Those are separate questions. Making them separate fields gives Evidra a stable
base for external adapters without making AgentGateway or Argo CD feel like
special cases.

It also matches the existing architecture direction: one lifecycle, one scoring
lane, but richer context for explanation and slicing.

## Data Model Changes

The current typed payload structs should grow first-class metadata instead of
relying on post-marshal JSON mutation.

Add typed metadata to `pkg/evidence/payloads.go`:

- `Flavor` enum or validated string constants
- `EvidenceMetadata{Kind}`
- `SourceMetadata{System}`

Then add these fields to both:

- `PrescriptionPayload`
- `ReportPayload`

Expected serialized shape:

```json
{
  "prescription_id": "01...",
  "canonical_action": {...},
  "effective_risk": "high",
  "flavor": "reconcile",
  "evidence": {
    "kind": "translated"
  },
  "source": {
    "system": "argocd"
  }
}
```

This keeps the new taxonomy inside the payload where `flavor` already lives,
without forcing an evidence-envelope redesign.

## Write-Path Rules

### Native lifecycle service

`internal/lifecycle` is shared by CLI and MCP, so it must not guess the
`source.system`. Adapter edges must provide metadata explicitly via
`PrescribeInput` and `ReportInput`.

Default expectations:

- CLI direct paths write:
  - `flavor=imperative`
  - `evidence.kind=declared`
  - `source.system=cli`
- MCP direct paths write:
  - `flavor=imperative`
  - `evidence.kind=declared`
  - `source.system=mcp`

### Mapped automation events

`internal/automationevent` should stop mutating raw JSON maps and instead build
typed payloads directly.

Current Argo CD mapped/controller flows should write:

- `flavor=reconcile`
- `evidence.kind=translated`
- `source.system=argocd`

### Proxy and future observed adapters

This branch does not need to retrofit proxy mode end to end, but the protocol
must make room for it. Future observed adapter paths should use:

- `evidence.kind=observed`

That is the key prerequisite for AgentGateway.

## Protocol And Documentation Rules

The normative docs should be updated to reflect the cleanup:

- replace `pipeline_stage` with `workflow`
- document the three metadata axes and their meanings
- update Argo CD docs to describe translated reconcile evidence
- update public architecture/docs wording so external adapters fit naturally

## Versioning

The MCP prompt contract remains on `v1.1.0` for this work. This taxonomy cleanup
does not require a new prompt surface, because the adapter metadata is written
by Evidra rather than chosen by the agent.

The evidence/spec version should be bumped because the normative evidence
payload vocabulary changes in a public-facing way.

## Testing Strategy

Write tests first for:

1. typed payload round-trip with the new metadata fields
2. lifecycle direct writes preserving CLI/MCP metadata
3. mapped Argo CD/webhook writes preserving translated reconcile metadata
4. removal of the loose `withPayloadFlavor` mutation helper
5. docs/examples updated to `workflow`

Repo verification should focus on targeted packages for this branch because the
current worktree has a stale `v1.0.1` promptfactory baseline issue that is
separate from this taxonomy cleanup. The active prompt contract for product
surfaces is already `v1.1.0`.

## Expected Outcome

After this change:

- Evidra has a cleaner public ingest taxonomy before external adapters land
- AgentGateway can later map into `observed` evidence cleanly
- Argo CD remains supported but is represented more honestly as translated
  reconcile evidence
- the product protocol becomes more agnostic and easier to extend to systems
  such as Argo CD and other control planes without redesigning the core model
