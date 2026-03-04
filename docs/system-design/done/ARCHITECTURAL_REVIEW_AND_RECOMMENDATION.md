# Chief Architect Review: System Design Alignment and Reuse Map

Date: 2026-03-04  
Scope: `docs/system-design/` (source of truth) + current repo code + `../evidra-mcp` code lineage  
Objective: identify what is reusable from `evidra-mcp`, what is already reused, and what is not synchronized with the new system design.

## Executive Verdict

The architecture documents define a strong inspector-model product, but the implementation is only partially synchronized. The project contains meaningful reused code from `evidra-mcp`, yet the semantic contract in `docs/system-design/` is not fully implemented end-to-end.

Current status is best described as: **foundation in place, contract conformance not complete**.

## Findings (ordered by severity)

### Critical findings

1. Evidence model mismatch with system design contract.
- Design requires `EvidenceEntry` envelope with `entry_id`, `type`, `trace_id`, version fields, and typed payloads (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:562`, `docs/system-design/EVIDRA_CORE_DATA_MODEL.md:160`).
- Current storage model still uses legacy record shape with `PolicyDecision`, `policy_ref`, `bundle_revision` (`pkg/evidence/types.go:11`, `pkg/evidence/types.go:33`, `pkg/evidence/types.go:43`).
- Impact: signal and score reproducibility cannot satisfy the stated invariants.

2. Scorecard path is not wired to real evidence.
- `evidra scorecard` uses `signal.AllSignals(nil)` and does not read evidence (`cmd/evidra/main.go:67`).
- Impact: benchmark output is placeholder, not actual reliability computation.

3. Canonicalization contract divergence in identity semantics.
- Scope class in code is topology (`single`, `namespace`, `cluster`) (`internal/canon/types.go:143`), while design expects environment scope (`production`, `staging`, `development`, `unknown`) (`docs/system-design/CANONICALIZATION_CONTRACT_V1.md:27`, `docs/system-design/CANONICALIZATION_CONTRACT_V1.md:944`).
- `intent_digest` is derived from full `CanonicalAction` including `resource_shape_hash` (`internal/canon/k8s.go:92`, `internal/canon/terraform.go:96`, `internal/canon/generic.go:37`), but contract excludes `resource_shape_hash` from intent digest (`docs/system-design/CANONICALIZATION_CONTRACT_V1.md:40`, `docs/system-design/CANONICALIZATION_CONTRACT_V1.md:285`).
- Impact: retry behavior and identity guarantees drift from the published contract.

4. Prescribe/report lifecycle not fully represented in persisted entries.
- Report input/output and persisted report do not carry actor/trace correlation as required by design (`pkg/mcpserver/server.go:304`, `pkg/mcpserver/server.go:337`; compare with `docs/system-design/EVIDRA_CORE_DATA_MODEL.md:105`).
- No `ttl_ms`, `canonicalization_failure`, `finding` entry paths (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:361`, `docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:601`, `docs/system-design/EVIDRA_CORE_DATA_MODEL.md:186`).
- Impact: protocol_violation sub-signals and confidence scoring are under-specified at runtime.

5. Signal detector defaults and logic diverge from signal spec.
- Retry window is 10m in code (`internal/signal/retry_loop.go:14`) vs 30m in spec (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:419`).
- Blast radius thresholds in code are `destroy>10`, `mutate>50` (`internal/signal/blast_radius.go:5`) vs spec destructive-only threshold 5 (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:471`, `docs/system-design/EVIDRA_SIGNAL_SPEC.md:476`).
- New scope key in code is `(tool, opClass)` (`internal/signal/new_scope.go:6`) vs spec `(actor.id, tool, operation_class, scope_class)` (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:533`).
- `stalled_operation` and `crash_before_report` classification is reversed relative to spec meaning (`internal/signal/protocol_violation.go:141`; `docs/system-design/EVIDRA_SIGNAL_SPEC.md:275`).

### High findings

6. Risk matrix taxonomy does not match environment-based model.
- Code matrix keyed by `single|namespace|cluster` scope (`internal/risk/matrix.go:6`).
- Design matrix keyed by `development|staging|production|unknown` (`docs/system-design/EVIDRA_AGENT_RELIABILITY_BENCHMARK.md:515`).

7. Digest formatting inconsistency.
- Design examples show `sha256:`-prefixed digests (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:674`).
- Code emits raw hex digest strings (`internal/canon/generic.go:51`, `internal/canon/k8s.go:165`).

8. Forwarding contract is exposed but not functionally wired.
- CLI exposes `--forward-url` and env `EVIDRA_API_URL` (`cmd/evidra-mcp/main.go:29`, `cmd/evidra-mcp/main.go:97`).
- `ForwardURL` option exists but is not consumed in runtime logic (`pkg/mcpserver/server.go:24`).

### Medium findings

9. Confidence model is documented but not implemented.
- Design requires confidence and score ceilings (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:497`, `docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:508`).
- Current `internal/score/score.go` returns weighted score only.

10. Evidence subsystem test depth from `evidra-mcp` was not carried over.
- `evidra-mcp` has robust evidence tests (`../evidra-mcp/pkg/evidence/evidence_test.go`, `../evidra-mcp/pkg/evidence/segmented_test.go`).
- Current repo has no tests in `pkg/evidence`.

11. Design terminology drift exists across docs and must be frozen.
- Some docs use `mutate/destroy/read/plan`; others use `mutating/destructive/read-only`.
- This ambiguity will keep generating code drift unless resolved in one normative enum set.

## Reuse State from evidra-mcp

### File-level lineage snapshot

- Current repo (`cmd/internal/pkg`) file count: 50  
- `evidra-mcp` (`cmd/internal/pkg`) file count: 111  
- Direct overlap: 27 files

### Reused and worth keeping

1. `pkg/evidence/*` storage mechanics (segmentation, locking, chain validation)
- Keep implementation patterns.
- Refactor persisted schema to contract-compliant `EvidenceEntry`.

2. `pkg/evlock/*`
- Keep as-is.

3. MCP server scaffold (`pkg/mcpserver/*`)
- Keep tool wiring/resource patterns.
- Refactor request/response/persistence models to contract.

4. Detector implementation style (`internal/risk/*`)
- Keep pure-Go detector approach.
- Align thresholds, scope taxonomy, and detector catalog to docs.

### Reused but not synchronized

1. `pkg/evidence/types.go` and record payload.
2. `pkg/mcpserver/server.go` persist/report semantics.
3. `internal/canon/*` scope and digest semantics.
4. `internal/signal/*` defaults and keying semantics.
5. `cmd/evidra/main.go` scorecard/compare runtime behavior.

### Not yet reused but recommended to reuse (with adaptation)

1. API platform primitives from `evidra-mcp`:
- `internal/api/middleware.go`, `internal/api/response.go`, `internal/api/health_handler.go`
- `internal/auth/*`
- `internal/store/keys.go`
- `internal/db/*`
- `cmd/evidra-api/main.go` bootstrap pattern

Use these as infrastructure scaffolding for v0.5+, but remove old `/v1/validate` policy-engine semantics and replace with benchmark protocol endpoints.

2. Evidence and MCP integration tests:
- Port and adapt:
  - `../evidra-mcp/pkg/evidence/evidence_test.go`
  - `../evidra-mcp/pkg/evidence/segmented_test.go`
  - `../evidra-mcp/cmd/evidra-mcp/test/*`

### Explicitly do not reuse

- `pkg/policy/*`
- `pkg/runtime/*`
- `pkg/validate/*`
- `pkg/bundlesource/*`
- `pkg/policysource/*`
- `internal/engine/*`
- any `validate`-first API workflow

These conflict with the inspector model and no-OPA direction.

## Recommended Plan

### P0: Contract conformance first

1. Freeze one canonical enum taxonomy in docs, then enforce in code/schemas.
2. Implement unified `EvidenceEntry` envelope and migrate writers/readers.
3. Wire scorecard to real evidence scan and signal conversion.
4. Fix canonicalization scope and digest rules to contract.
5. Align signal detectors to signal spec defaults and semantics.

### P1: Reliability and trust hardening

1. Add `trace_id`, `ttl_ms`, `canon_source`, and version fields to all entries.
2. Implement `canonicalization_failure` and `finding` entry types.
3. Implement confidence model and score ceilings.
4. Port/adapt evidence and MCP end-to-end tests from `evidra-mcp`.

### P2: Platform reuse (v0.5+)

1. Reuse API/auth/db/store scaffolding from `evidra-mcp`.
2. Replace legacy validate endpoints with prescribe/report/scorecard APIs.
3. Complete forward path and add metrics export per `EVIDRA_SIGNAL_SPEC.md`.

## Immediate Actionable PR Sequence

1. PR-1: EvidenceEntry schema migration and compatibility reader.
2. PR-2: Canonicalization + risk taxonomy synchronization.
3. PR-3: Real scorecard computation + confidence output.
4. PR-4: Signal detector spec conformance updates.
5. PR-5: Port/adapt evidence and MCP integration tests from `evidra-mcp`.

## Final Position

The strategic direction is strong.  
The primary risk is semantic drift between architecture docs and reused code.  
Use `evidra-mcp` as a source of reusable infrastructure primitives, not as the semantic contract.  
Treat `docs/system-design/` as normative and refactor all reused modules to match it.
