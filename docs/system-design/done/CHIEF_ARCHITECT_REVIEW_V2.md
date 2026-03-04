# Chief Architect Review V2: System Design vs Implementation vs evidra-mcp Reuse

Date: 2026-03-04  
Author: Chief Architect Review  
Scope baseline: `docs/system-design/` (active docs) + code in this repo + code in `../evidra-mcp`

## Executive Verdict

The system design in `docs/system-design/` is coherent at the model level (inspector model, five signals, append-only evidence), but the current implementation is only partially aligned. The codebase still carries substantial legacy structure reused from `evidra-mcp` that is not synchronized with the new contract.

Current state should be treated as **architecture-in-progress**, not contract-complete `v0.3.0`.

## Findings (ordered by severity)

### Critical

1. Evidence schema is not aligned with the system design contract.
- Design expects envelope-style `EvidenceEntry` with `entry_id`, `type`, `trace_id`, version fields, and typed payloads (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:562`, `docs/system-design/EVIDRA_CORE_DATA_MODEL.md:160`).
- Implementation persists legacy record shape with `PolicyDecision`, `policy_ref`, `bundle_revision`, `profile_name` (`pkg/evidence/types.go:11`, `pkg/evidence/types.go:33`, `pkg/evidence/types.go:43`).
- Impact: core invariants are broken (replay portability, lifecycle semantics, version visibility).

2. Scorecard and evidence-to-signal pipeline are not actually wired.
- `cmd/evidra scorecard` runs signals on `nil` entries (`cmd/evidra/main.go:67`).
- No conversion bridge from persisted evidence records to `internal/signal.Entry`.
- Impact: reliability outputs are placeholders; design promises are not executable.

3. Canonicalization contract drift: scope model and digest composition.
- Scope in code is topology (`single|namespace|cluster`) (`internal/canon/types.go:143`) while design requires environment scope (`production|staging|development|unknown`) (`docs/system-design/CANONICALIZATION_CONTRACT_V1.md:27`, `docs/system-design/CANONICALIZATION_CONTRACT_V1.md:944`).
- `intent_digest` currently hashes full `CanonicalAction` including `resource_shape_hash` (`internal/canon/k8s.go:92`, `internal/canon/terraform.go:96`, `internal/canon/generic.go:37`), but contract explicitly excludes `resource_shape_hash` (`docs/system-design/CANONICALIZATION_CONTRACT_V1.md:40`, `docs/system-design/CANONICALIZATION_CONTRACT_V1.md:285`).
- Impact: identity semantics and retry semantics are unstable against the published contract.

4. MCP prescribe/report protocol is incomplete vs design lifecycle.
- Report input has no actor/trace; persisted report lacks actor identity, making cross-actor validation non-deterministic (`pkg/mcpserver/server.go:304`, `pkg/mcpserver/server.go:337`).
- No `ttl_ms` materialization, no canonicalization_failure entry write path, no finding entry type path (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:361`, `docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:601`, `docs/system-design/EVIDRA_CORE_DATA_MODEL.md:186`).
- Impact: protocol violation semantics in docs cannot be reliably implemented from stored evidence.

5. Signal implementation diverges from formal signal spec defaults and logic.
- Retry window uses 10m (`internal/signal/retry_loop.go:14`) while spec states 30m (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:419`).
- Retry detector does not require prior failed execution and does not scope by actor (`internal/signal/retry_loop.go:24`), unlike spec (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:410`).
- Blast radius includes mutating threshold and uses 10/50 (`internal/signal/blast_radius.go:5`) while spec says destructive-only threshold 5 (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:471`, `docs/system-design/EVIDRA_SIGNAL_SPEC.md:476`).
- New scope key is `(tool, opClass)` (`internal/signal/new_scope.go:6`) but spec key is `(actor.id, tool, operation_class, scope_class)` (`docs/system-design/EVIDRA_SIGNAL_SPEC.md:533`).
- `stalled_operation` vs `crash_before_report` classification is reversed relative to spec table (`internal/signal/protocol_violation.go:141`, `docs/system-design/EVIDRA_SIGNAL_SPEC.md:275`).

### High

6. Risk matrix taxonomy mismatch.
- Code uses `read|mutate|destroy|plan` over topology-like scopes (`internal/risk/matrix.go:6`).
- Design matrix uses environment scope and operation-class taxonomy from contract docs (`docs/system-design/EVIDRA_AGENT_RELIABILITY_BENCHMARK.md:515`, `docs/system-design/CANONICALIZATION_CONTRACT_V1.md:25`).

7. Digest format mismatch.
- Design examples require `sha256:` prefixed digests (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:674`).
- Code emits raw hex without prefix (`internal/canon/generic.go:51`, `internal/canon/k8s.go:165`).

8. Forwarding/API wiring signals unfinished integration.
- CLI/server expose `--forward-url` and `EVIDRA_API_URL` (`cmd/evidra-mcp/main.go:29`, `cmd/evidra-mcp/main.go:97`), but `Options.ForwardURL` is not consumed in server logic (`pkg/mcpserver/server.go:24`).
- Impact: documented deployment shape (local -> forwarded -> API) is not operational in this repo.

9. Large reused legacy code is currently dead and increases ambiguity.
- `internal/evidence/*` and `pkg/invocation.ToolInvocation` are largely unreferenced by runtime pipeline.
- Impact: raises maintenance cost and obscures canonical data flow.

### Medium

10. Missing implementation of confidence and score ceilings.
- Design defines confidence model and score caps (`docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:497`, `docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md:508`).
- `internal/score/score.go` computes only weighted penalty; no confidence dimension.

11. Evidence store test coverage dropped during migration.
- `evidra-mcp` contains robust tests for chain integrity, segmentation, concurrency (`../evidra-mcp/pkg/evidence/evidence_test.go`, `../evidra-mcp/pkg/evidence/segmented_test.go`).
- Benchmark repo has no tests in `pkg/evidence`.

12. Design docs themselves contain terminology drift that should be frozen before code hardening.
- Some docs use `mutate/destroy/read/plan` while canonicalization contract uses `mutating/destructive/read-only`.
- This must be resolved once, then enforced in code and schemas.

## Reuse Analysis: evidra-mcp -> evidra-benchmark

### Inventory snapshot

- Bench files in `cmd/internal/pkg`: 50  
- evidra-mcp files in `cmd/internal/pkg`: 111  
- Overlap: 27 files (direct reuse lineage is real and significant)

### Reused and still valuable (keep, but align contract)

1. `pkg/evidence/*` storage mechanics (locking, segmentation, manifest, chain validation)
- Reuse value: high
- Required sync work: replace legacy record schema and hash payload fields with contract `EvidenceEntry`.

2. `pkg/evlock/*`
- Reuse value: high
- Sync work: none significant.

3. `pkg/mcpserver` server shell + MCP tool/resource registration pattern
- Reuse value: high
- Sync work: complete protocol/model alignment (inputs, outputs, persisted entry model, lifecycle checks).

4. `internal/risk` detector approach (simple Go detectors on raw artifacts)
- Reuse value: high
- Sync work: thresholds, taxonomy, and detector catalog alignment with design.

### Reused but currently not synced (major refactor required)

1. `pkg/evidence/types.go` and persisted record shape
- Legacy fields and structure do not match design model.

2. `pkg/mcpserver/server.go`
- Still writes legacy policy decision wrappers and report shape not aligned to contract lifecycle.

3. `internal/canon/*`
- Core adapter logic is good, but scope and digest rules are contract-divergent.

4. `internal/signal/*`
- Has all five detectors, but several algorithms/parameters differ from formal spec.

5. `cmd/evidra/main.go`
- Command surface exists, but scorecard/compare are stubs and do not operate on evidence.

### Not yet reused from evidra-mcp, but should be selectively reused

1. API platform skeleton for v0.5+:
- `internal/api/middleware.go`, `internal/api/response.go`, `internal/api/health_handler.go` are reusable with low risk.
- `internal/auth/*`, `internal/store/keys.go`, `internal/db/*` are reusable for tenant/auth/key model.
- `cmd/evidra-api/main.go` bootstrap pattern is reusable after removing OPA/validate engine dependencies.

2. Evidence-store tests:
- Port and adapt `pkg/evidence/evidence_test.go`, `pkg/evidence/segmented_test.go`, `pkg/evidence/lock_test.go`.

3. MCP integration tests:
- Port stdio integration tests from `cmd/evidra-mcp/test/*` and rewrite around `prescribe/report`.

### Do not reuse (or keep isolated only for historical context)

- `pkg/policy/*`
- `pkg/runtime/*`
- `pkg/validate/*`
- `pkg/bundlesource/*`
- `pkg/policysource/*`
- `internal/engine/*`
- any `validate`-first API flow

These are enforcement-era artifacts and conflict directly with inspector-model invariants.

## Recommendation Plan

### Phase P0 (must complete before claiming architecture alignment)

1. Freeze contract terms in docs:
- Resolve taxonomy conflict (`mutate` vs `mutating`, etc.).
- Publish one canonical enum set used by schema, canonicalization, risk, signals, score.

2. Replace persisted evidence model:
- Introduce single `EvidenceEntry` envelope exactly per design.
- Remove legacy `PolicyDecision` and policy-era fields from runtime path.

3. Wire real scorecard pipeline:
- Read evidence -> map to `signal.Entry` -> run detectors -> compute score.
- Remove placeholder behavior in CLI scorecard/compare.

4. Fix canonicalization contract mismatches:
- scope_class resolution by environment/namespace mapping.
- `intent_digest` excludes `resource_shape_hash`.
- digest formatting policy finalized (`sha256:` or raw hex) and applied consistently.

5. Align signal implementations with spec:
- Retry window/logic, blast radius threshold scope, new_scope key dimensions, stalled/crash semantics.

### Phase P1 (stabilization and trust)

1. Implement `trace_id`, `ttl_ms`, `canon_source`, and version fields in all entries.
2. Add `canonicalization_failure` and `finding` entry types.
3. Implement confidence model and score ceilings.
4. Port evidence store tests and MCP e2e tests from evidra-mcp.

### Phase P2 (v0.5 platform reuse)

1. Reintroduce API/server path by reusing evidra-mcp HTTP skeleton:
- keep middleware/auth/store/db scaffolding
- replace `/v1/validate` with `/v1/prescribe`, `/v1/report`, `/v1/scorecard`

2. Implement forward path from local writers to API.
3. Add Prometheus metrics per signal spec registry.

## Suggested Immediate Work Items (next PRs)

1. PR-1: EvidenceEntry schema + writer migration (no score changes).
2. PR-2: Canonicalization/risk taxonomy sync + signal detector parameter fixes.
3. PR-3: Real `evidra scorecard` over evidence chain + confidence output.
4. PR-4: Port/adapt evidence and MCP integration tests from evidra-mcp.

## Final Architectural Position

The design direction is correct and differentiated.  
The main risk is not missing features; it is semantic drift between contract docs and reused code.  
Use evidra-mcp as a **source of infrastructure primitives**, not as a semantic source of truth.  
Treat `docs/system-design/` as the contract authority, then refactor reused modules to conform.
