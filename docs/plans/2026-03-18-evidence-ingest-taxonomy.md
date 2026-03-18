# Evidence Ingest Taxonomy Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split execution flavor from evidence acquisition metadata so Evidra can support clean external adapters such as AgentGateway without overloading `payload.flavor`.

**Architecture:** Add typed payload metadata for `flavor`, `evidence.kind`, and `source.system`; wire direct CLI/MCP and mapped Argo CD paths to populate it explicitly; update the public protocol/docs to replace `pipeline_stage` with `workflow` and describe the new taxonomy.

**Tech Stack:** Go, Go tests, Markdown system-design docs, existing CLI/MCP/API code paths

---

### Task 1: Add typed payload metadata primitives

**Files:**
- Modify: `pkg/evidence/payloads.go`
- Test: `pkg/evidence/payloads_test.go`

**Step 1: Write the failing tests**

Add tests that assert:

- `PrescriptionPayload` round-trips `flavor`, `evidence.kind`, and `source.system`
- `ReportPayload` round-trips the same fields
- zero-value metadata remains omitted when empty

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/evidence -run 'TestPrescriptionPayload|TestReportPayload' -v`

Expected: FAIL because the typed payloads do not yet include the new metadata fields.

**Step 3: Write minimal implementation**

Add:

- typed constants for `imperative`, `reconcile`, `workflow`
- typed constants for `declared`, `observed`, `translated`
- small metadata structs for `evidence.kind` and `source.system`
- the new fields on `PrescriptionPayload` and `ReportPayload`

Keep the serialized field names:

- `flavor`
- `evidence`
- `source`

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/evidence -run 'TestPrescriptionPayload|TestReportPayload' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/evidence/payloads.go pkg/evidence/payloads_test.go
git commit -m "feat: add typed evidence payload taxonomy"
```

### Task 2: Thread metadata through the native lifecycle path

**Files:**
- Modify: `internal/lifecycle/types.go`
- Modify: `internal/lifecycle/service.go`
- Modify: `cmd/evidra/prescribe.go`
- Modify: `cmd/evidra/report.go`
- Modify: `cmd/evidra/record.go`
- Modify: `cmd/evidra/import.go`
- Modify: `pkg/mcpserver/correlation.go`
- Test: `internal/lifecycle/service_test.go`
- Test: `internal/lifecycle/report_declined_test.go`
- Test: `pkg/mcpserver/correlation_test.go`

**Step 1: Write the failing tests**

Add tests that assert:

- direct lifecycle prescribe/report entries include `flavor=imperative`
- CLI-prepared inputs produce `evidence.kind=declared` and `source.system=cli`
- MCP correlation helpers produce `evidence.kind=declared` and `source.system=mcp`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/lifecycle ./pkg/mcpserver -run 'Test.*(Flavor|Evidence|Source)' -v`

Expected: FAIL because lifecycle inputs and payload writes do not yet carry the metadata.

**Step 3: Write minimal implementation**

Add metadata fields to:

- `lifecycle.PrescribeInput`
- `lifecycle.ReportInput`

Populate them at adapter edges:

- CLI commands -> `imperative`, `declared`, `cli`
- MCP direct adapters -> `imperative`, `declared`, `mcp`

Update `internal/lifecycle/service.go` to write the typed payload metadata.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/lifecycle ./pkg/mcpserver -run 'Test.*(Flavor|Evidence|Source)' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/lifecycle/types.go internal/lifecycle/service.go cmd/evidra/prescribe.go cmd/evidra/report.go cmd/evidra/record.go cmd/evidra/import.go pkg/mcpserver/correlation.go internal/lifecycle/service_test.go internal/lifecycle/report_declined_test.go pkg/mcpserver/correlation_test.go
git commit -m "feat: tag direct evidence with source taxonomy"
```

### Task 3: Convert mapped automation events to typed metadata

**Files:**
- Modify: `internal/automationevent/emitter.go`
- Modify: `internal/api/webhooks.go`
- Modify: `internal/gitops/argocd/controller.go`
- Test: `internal/automationevent/emitter_test.go`
- Test: `internal/api/webhooks_test.go`
- Test: `internal/gitops/argocd/controller_test.go`

**Step 1: Write the failing tests**

Add tests that assert mapped Argo CD and webhook entries serialize:

- `flavor=reconcile`
- `evidence.kind=translated`
- `source.system=argocd`

and that the old loose JSON mutation helper is no longer needed.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/automationevent ./internal/api ./internal/gitops/argocd -run 'Test.*(Flavor|Evidence|Source|Mapped)' -v`

Expected: FAIL because mapped payload builders only inject `flavor`.

**Step 3: Write minimal implementation**

Build typed `PrescriptionPayload` / `ReportPayload` directly in the mapped paths.

Remove or replace `withPayloadFlavor` with typed helpers.

Set mapped Argo CD writes to:

- `reconcile`
- `translated`
- `argocd`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/automationevent ./internal/api ./internal/gitops/argocd -run 'Test.*(Flavor|Evidence|Source|Mapped)' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/automationevent/emitter.go internal/api/webhooks.go internal/gitops/argocd/controller.go internal/automationevent/emitter_test.go internal/api/webhooks_test.go internal/gitops/argocd/controller_test.go
git commit -m "feat: tag mapped evidence with translated taxonomy"
```

### Task 4: Rename `pipeline_stage` to `workflow` in protocol/docs

**Files:**
- Modify: `docs/system-design/EVIDRA_PROTOCOL_V1.md`
- Modify: `docs/system-design/EVIDRA_ARCHITECTURE_V1.md`
- Modify: `docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md`
- Modify: `docs/guides/argocd-gitops-integration.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `CHANGELOG.md`

**Step 1: Write the failing test**

Add or update a focused doc/grep-style test if one exists; otherwise use a diff check plus targeted grep verification in the implementation step.

**Step 2: Run verification to confirm old wording exists**

Run: `rg -n 'pipeline_stage' docs internal`

Expected: matches in normative docs and constants.

**Step 3: Write minimal implementation**

Update the docs to:

- replace `pipeline_stage` with `workflow`
- explain `flavor`, `evidence.kind`, and `source.system`
- describe Argo CD as translated reconcile evidence

**Step 4: Run verification to confirm new wording**

Run: `rg -n 'pipeline_stage' docs internal`

Expected: no remaining product/protocol matches for the retired term.

**Step 5: Commit**

```bash
git add docs/system-design/EVIDRA_PROTOCOL_V1.md docs/system-design/EVIDRA_ARCHITECTURE_V1.md docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md docs/guides/argocd-gitops-integration.md docs/ARCHITECTURE.md CHANGELOG.md
git commit -m "docs: clean up evidence ingest taxonomy"
```

### Task 5: Bump spec metadata and verify the branch

**Files:**
- Modify: `pkg/version/version.go`
- Modify: any tests asserting spec version where needed
- Verify: targeted packages above
- Verify: repo lint baseline

**Step 1: Write the failing test**

Add or update a targeted assertion that written entries expose the intended spec version after the taxonomy change.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/evidra ./internal/lifecycle ./internal/automationevent ./pkg/evidence -run 'Test.*SpecVersion' -v`

Expected: FAIL if the version constant or expectations are stale.

**Step 3: Write minimal implementation**

Bump `pkg/version.SpecVersion` and adjust any targeted tests that intentionally
assert the new version.

Keep the MCP prompt contract on `v1.1.0`; do not invent a new prompt contract
for this branch.

**Step 4: Run verification**

Run:

```bash
go test ./pkg/evidence ./internal/lifecycle ./internal/automationevent ./internal/api ./internal/gitops/argocd ./pkg/mcpserver ./cmd/evidra -v
make lint
```

Expected:

- targeted Go tests PASS
- `make lint` either passes or fails only on known repo baseline issues, which must be reported explicitly

**Step 5: Commit**

```bash
git add pkg/version/version.go cmd/evidra internal/lifecycle internal/automationevent internal/api internal/gitops/argocd pkg/evidence pkg/mcpserver CHANGELOG.md
git commit -m "feat: split evidence flavor from ingest taxonomy"
```
