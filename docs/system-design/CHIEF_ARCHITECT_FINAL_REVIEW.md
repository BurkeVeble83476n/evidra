# Chief Architect Final Review: Evidra Benchmark v0.3.0

**Date:** 2026-03-04
**Status:** BINDING ARCHITECTURAL DIRECTIVE
**Inputs:** Chief Architect Review V1, Chief Architect Review V2,
verified codebase audit, system design corpus (6 active docs)

---

## 1. Executive Verdict

Both colleague reviews are correct in their core findings. The
system design is coherent and differentiated. The codebase is
**structurally reusable but semantically misaligned** — it carries
the OPA/enforcer-era data model while the architecture has moved
to the inspector/benchmark model.

This is not a "mostly done, needs polish" situation. The
misalignment is in the **data path** — evidence schema, digest
computation, signal parameters, and scorecard pipeline. These
are the load-bearing walls. Everything else (adapters, risk
detectors, MCP server shell, CLI structure) is sound and reusable.

**Risk assessment:** If we ship v0.3.0 without fixing the data
path, we create a compatibility debt that will cost 10x to fix
after users have evidence files in the old format. The evidence
schema is the one thing we cannot change after release.

---

## 2. Consolidated Findings

I verified every finding from both reviews against the codebase.
Below is the authoritative list, deduplicated and prioritized.

### CRITICAL (blocks v0.3.0 release)

| ID | Finding | V1 | V2 | Verified |
|----|---------|----|----|----------|
| C1 | **Two EvidenceRecord structs, both wrong.** `pkg/evidence/types.go:30` has PolicyDecision, PolicyRef, BundleRevision. `internal/evidence/types.go:33` has a different struct with Decision, SigningPayload. Neither matches `EVIDRA_CORE_DATA_MODEL.md`. | GAP in §2.3 | Finding #1 | Yes — two structs, zero alignment with design |
| C2 | **Scorecard is a stub.** `cmd/evidra/main.go:68` passes `nil` to signal detectors, 0 for total ops. No evidence chain reader. | GAP-02 | Finding #2 | Yes — `signal.AllSignals(nil)`, `score.Compute(results, 0)` |
| C3 | **ScopeClass is topology, not environment.** `internal/canon/types.go:143` returns single/namespace/cluster by counting namespaces. Design requires production/staging/development/unknown. | GAP-01 | Finding #3 | Yes — `ScopeClass()` counts namespaces |
| C4 | **intent_digest includes resource_shape_hash.** `internal/canon/k8s.go:91` marshals full CanonicalAction (including ResourceShapeHash) into digest. Contract §Digest Rules explicitly excludes shape_hash from intent. | — | Finding #3 | Yes — `json.Marshal(action)` includes all fields |
| C5 | **Signal parameters diverge from spec.** Retry: 10min window (spec: 30min), no prior-failure requirement, no actor scoping. Blast: mutating threshold 50 (spec: destructive-only, threshold 5). New scope: key is (tool, opClass), spec requires (actor, tool, opClass, scopeClass). Stalled/crash classification inverted. | — | Finding #5 | Yes — all five detectors have parameter mismatches |
| C6 | **Report has no actor/trace_id in MCP input.** `pkg/mcpserver/server.go:57` ReportInput has only PrescriptionID, ExitCode, ArtifactDigest. No actor, no trace_id. Cross-actor violation detection is impossible. | — | Finding #4 | Yes — ReportInput missing actor and trace_id |
| C7 | **No ttl_ms materialization.** Prescribe output and evidence entry have no TTL field. TTL detection at scorecard time requires the TTL value to be stored. | — | Finding #4 | Yes — no ttl_ms anywhere in prescribe path |

### HIGH (should fix for v0.3.0)

| ID | Finding | Verified |
|----|---------|----------|
| H1 | **Digest format inconsistency.** Canon digests are raw hex (`internal/canon/generic.go:51`). Evidence hashes use `sha256:` prefix (`internal/evidence/builder.go:67`). Design requires `sha256:` prefix everywhere. |
| H2 | **Risk matrix uses topology scope.** `internal/risk/matrix.go:5` uses single/namespace/cluster. Must use production/staging/development/unknown to match ScopeClass fix. |
| H3 | **Forwarding not wired.** CLI/MCP expose `--forward-url` / `EVIDRA_API_URL` but `pkg/mcpserver/server.go` never consumes it. Dead config. |
| H4 | **Dead legacy code.** `internal/evidence/*` (builder, signer, types) and `pkg/invocation/` are largely unreferenced by the runtime pipeline. Increases ambiguity. |
| H5 | **Evidence store tests missing.** `evidra-mcp` has robust chain integrity/segmentation/concurrency tests. This repo has zero tests for `pkg/evidence/`. |
| H6 | **Terminology drift.** Some code uses `mutate/destroy`, contract uses `mutating/destructive`. Must freeze one canonical enum set. |

### MEDIUM (v0.3.0 or early v0.3.x)

| ID | Finding | Verified |
|----|---------|----------|
| M1 | **No confidence model in scoring.** `internal/score/score.go` computes only weighted penalty. No confidence dimension, no score ceilings. |
| M2 | **No canonicalization_failure entry type.** Parse errors return MCP error but don't write evidence. |
| M3 | **No finding entry type.** No SARIF parser, no `--scanner-report` flag. |
| M4 | **No canon_source field.** Pre-canonicalized path not marked in evidence. |
| M5 | **No version fields in evidence entries.** spec_version, canon_version, adapter_version, scoring_version not written. |

---

## 3. Where V1 and V2 Agree (and I Concur)

Both reviews agree on:

1. **Design is correct.** The inspector model, five signals,
   append-only evidence, canonicalization contract — all sound.
2. **Evidence schema is the #1 priority.** Fix this first because
   everything downstream depends on it.
3. **OPA remnants must be purged.** PolicyDecision, PolicyRef,
   BundleRevision, validate — all must go.
4. **Scorecard must be real.** A stub scorecard is worse than no
   scorecard — it creates false confidence.
5. **The adapters are reusable.** K8s and Terraform adapter logic
   is good. Only scope resolution and digest computation need fixing.

I concur with all five points.

## 4. Where V1 and V2 Disagree (My Resolution)

### Digest format: sha256: prefix or raw hex?

V1 doesn't address this. V2 identifies the inconsistency.

**Decision: `sha256:` prefix everywhere.** This is what the design
docs specify. Raw hex is ambiguous (could be SHA-1 in the future).
Prefixed format is self-describing and matches OCI/Docker conventions.
One function, one format, used by all digest producers.

### Protocol rename timing

V1 says rename `validate` → `prescribe` immediately. V2 treats it
as part of the broader MCP alignment.

**Decision: Rename in the evidence schema PR (PR-1).** The MCP tool
name change and the evidence schema change are the same PR because
both affect the wire format. Doing them separately creates a
half-migrated state.

### Forwarding wiring

V1 doesn't mention it. V2 flags it as dead config.

**Decision: Remove the config flags for v0.3.0.** Forward-url and
API-url are v0.5.0 features. Dead config that suggests working
features is worse than no config. Remove flags, remove env var
handling. Re-add when evidra-api exists.

### Confidence model timing

V1 puts it in Phase 3 (P2). V2 puts it in Phase P1.

**Decision: P1, but after scorecard works.** Confidence without a
working scorecard is academic. Wire the scorecard first (C2), then
add confidence (M1). Both land in v0.3.0.

---

## 5. My Additional Findings

Beyond what both reviews covered:

### A1. CanonicalAction.tool duplication risk

The `CanonicalAction` struct has a `Tool` field. The evidence entry
envelope also needs to know the tool for filtering. Current code
puts tool in both places. **Decision:** tool lives in
`canonical_action.tool` only. Evidence entry payload contains the
full canonical_action. No top-level duplication.

### A2. trace_id generation is undefined

Design says trace_id is mandatory. No code generates it. The MCP
server receives prescribe/report calls but has no session concept.

**Decision:** For MCP, trace_id = one MCP session (process lifetime
of evidra-mcp). Generate a ULID at server startup, use for all
entries until process exits. For CLI, trace_id = one command
invocation. Generate a ULID per `evidra prescribe` call, pass it
to the corresponding `evidra report` via the prescription output.

### A3. Operation class enum must be frozen

Code uses `read/mutate/destroy/plan`. Contract uses
`read-only/mutating/destructive`. Signal spec uses yet another
variant in places.

**Decision:** Freeze the enum as: `read`, `mutate`, `destroy`,
`plan`. These are the values in the code, they're terse, and
they're already in the golden corpus digests. Changing them now
would break all golden files. Update the contract doc to match.

### A4. Generic adapter is a stub

V1 mentions it. The generic adapter exists but does minimal work.
For pre-canonicalized input, this is acceptable — the caller
provides the canonical_action. But for truly generic input (opaque
YAML/JSON), the adapter should at minimum compute artifact_digest
and resource_shape_hash.

**Decision:** Generic adapter stays minimal for v0.3.0. Its job is
pre-canonicalized pass-through. Document this explicitly.

---

## 6. Implementation Plan

### PR-1: Evidence Schema Migration (P0, ~3 days)

**Goal:** Single EvidenceEntry struct matching EVIDRA_CORE_DATA_MODEL.md.

Changes:
- Delete `internal/evidence/types.go` (legacy Decision/DecisionRecord)
- Rewrite `pkg/evidence/types.go` → single `EvidenceEntry` envelope
  with typed payloads per CORE_DATA_MODEL §5
- All fields: entry_id, previous_hash, hash, signature, type,
  tenant_id, trace_id, actor, timestamp, intent_digest,
  artifact_digest, payload, spec_version, canon_version,
  adapter_version, scoring_version
- Entry types: prescribe, report, canonicalization_failure,
  finding, signal, receipt
- Digest format: `sha256:` prefix everywhere
- MCP tool rename: validate → prescribe
- Remove ForwardURL config (dead code)
- Port evidence store tests from evidra-mcp
- Delete dead code: `pkg/invocation/`, unused `internal/evidence/`
  builder/signer

**Verification:** Golden corpus still passes (digests unchanged
at adapter level). New evidence entries written in correct format.

### PR-2: Canonicalization Contract Sync (P0, ~2 days)

**Goal:** Adapter output matches contract exactly.

Changes:
- `ScopeClass()`: environment-based (production/staging/development/
  unknown) via namespace substring matching + explicit `--env` override
- `intent_digest`: exclude resource_shape_hash from hash input.
  Marshal only identity fields (tool, operation_class, scope_class,
  resource_identity, resource_count)
- Risk matrix: update to use environment-based scope values
- Operation class enum: freeze as read/mutate/destroy/plan,
  update contract doc to match
- Digest format: all canon digests use `sha256:` prefix

**Verification:** Golden corpus updated (EVIDRA_UPDATE_GOLDEN=1).
Review all digest changes. Bump canon version if identity changes.

### PR-3: Signal Detector Alignment (P0, ~2 days)

**Goal:** All five detectors match EVIDRA_SIGNAL_SPEC.md exactly.

| Detector | Current | Target |
|----------|---------|--------|
| retry_loop | 10min window, no failure req, no actor scope | 30min window, requires prior failure, scoped by actor |
| blast_radius | mutating=50, destructive=10 | destructive-only, threshold=5 |
| new_scope | key=(tool, opClass) | key=(actor, tool, opClass, scopeClass) |
| protocol_violation | stalled/crash classification | verify alignment with spec §Signal 1 table |
| artifact_drift | — | verify — likely correct |

Changes:
- Update constants and grouping logic in all five detectors
- Add actor field to ReportInput in MCP server
- Add trace_id generation (ULID at server/command startup)
- Add ttl_ms to prescription output and evidence entry

**Verification:** Update signal unit tests. Run against evidence
fixtures. Verify scorecard output changes are expected.

### PR-4: Real Scorecard Pipeline (P0, ~2 days)

**Goal:** `evidra scorecard` reads evidence, computes signals, outputs score.

Changes:
- Evidence chain reader: scan JSONL, deserialize EvidenceEntry,
  map to signal.Entry
- Wire `cmd/evidra/main.go` scorecard command to reader
- Wire `cmd/evidra/main.go` compare command
- Add `evidra explain` command (top signals + entry refs)
- Add version metadata to scorecard output (spec_version,
  canon_version, evidra_version, generated_at)

**Verification:** Create test evidence JSONL with known signals.
Assert scorecard output matches expected score and band.

### PR-5: Confidence Model + Score Ceilings (P1, ~1 day)

**Goal:** Scorecard includes confidence based on evidence quality.

Changes:
- Compute confidence from: evidence completeness (unreported %),
  canon_source distribution (adapter vs external %), actor trust
- Apply score ceilings: High=100, Medium=95, Low=85
- Add safety floors: protocol_violation_rate > 10% → score ≤ 90,
  artifact_drift_rate > 5% → score ≤ 85
- Output confidence alongside score and band

### PR-6: Entry Types + SARIF (P1, ~3 days)

**Goal:** canonicalization_failure, finding entry types. SARIF parser.

Changes:
- Write evidence entry on parse failure (type=canonicalization_failure)
- Add `--scanner-report` flag accepting SARIF
- SARIF parser: extract findings, severity, resource
- Findings → independent evidence entries (type=finding), linked
  by artifact_digest
- Scanner findings elevate risk_level on prescription
- Add canon_source field (adapter/external) to prescription payload

### PR-7: Test Porting (P1, ~2 days)

**Goal:** Evidence store and MCP integration tests.

Changes:
- Port chain integrity tests from evidra-mcp
- Port segmentation and concurrency tests
- Port MCP stdio integration tests, rewrite for prescribe/report
- Add signal detector tests with evidence fixtures

---

## 7. Dependency Graph

```
PR-1 (evidence schema)
  │
  ├──► PR-2 (canon sync)
  │       │
  │       └──► PR-3 (signal alignment)
  │               │
  │               └──► PR-4 (real scorecard)
  │                       │
  │                       ├──► PR-5 (confidence)
  │                       └──► PR-6 (entry types + SARIF)
  │
  └──► PR-7 (test porting) — can start after PR-1
```

PR-1 is the foundation. Everything depends on it. PR-7 can run
in parallel with PR-2/3/4 after PR-1 lands.

**Critical path:** PR-1 → PR-2 → PR-3 → PR-4
**Estimated calendar time:** ~9 days sequential, ~7 days with
PR-7 parallelized.

---

## 8. What We Do NOT Do

Reaffirming boundaries from both reviews plus my own:

1. **No deny logic.** prescribe() returns risk_level, never blocks.
2. **No auto-remediation.** Evidra has zero write access to infra.
3. **No OPA, no Rego.** Delete all policy engine references.
4. **No real-time TTL.** Detection at scorecard time only (v0.3.0).
5. **No forwarding.** Remove dead config. Re-add for v0.5.0.
6. **No web dashboard.** CLI scorecard is sufficient for v0.3.0.
7. **No per-scanner code.** SARIF parser covers all scanners.
8. **No new signals.** Five signals, fixed. risk_ignorance is v0.4.0.
9. **No ML, no baselines, no adaptive thresholds.**
10. **No backwards compatibility with v0.2.0 evidence format.**
    Clean break. v0.3.0 is a new evidence format.

---

## 9. Document Artifacts Required

The design docs are consolidated (6 active files). But two gaps
remain before code hardening:

| Artifact | Status | Action |
|----------|--------|--------|
| Frozen enum table (operation_class, scope_class, risk_level, entry_type, verdict) | **Done** | Added to CORE_DATA_MODEL.md §9. |
| Required fields table for prescribe/report MCP input | **Done** | Added to CORE_DATA_MODEL.md §3 (MCP Input Contract). |
| trace_id generation rules | **Done** | Added to CORE_DATA_MODEL.md §10. |
| Evidence format migration note | **Done** | Added to ARCHITECTURE_OVERVIEW.md §Known Gaps. |
| CANONICALIZATION_CONTRACT_V1.md §Digest Rules | **Done** | intent_digest exclusion clarified, operation_class frozen as read/mutate/destroy/plan throughout. |

---

## 10. Disposition of Colleague Reviews

| Review | Verdict | Notes |
|--------|---------|-------|
| V1 | **Accepted with amendments.** Core findings correct. Phase ordering adjusted (evidence schema first, not protocol rename first — they're the same PR). Safety floors moved to P1 (needs working scorecard first). | Archive to `done/` after this review is approved. |
| V2 | **Accepted as authoritative diagnostic.** Most thorough review. All 12 findings verified. Reuse analysis is accurate and actionable. Only amendment: forwarding should be removed, not left as dead code. | Archive to `done/` after this review is approved. |

---

## 11. Success Criteria for v0.3.0

v0.3.0 is ready when:

1. `EvidenceEntry` matches CORE_DATA_MODEL.md exactly.
2. Zero OPA-era fields in any runtime code path.
3. Golden corpus passes with correct intent_digest (shape_hash excluded).
4. All five signal detectors match SIGNAL_SPEC.md parameters.
5. `evidra scorecard` produces real scores from real evidence.
6. `evidra explain` shows top signals with entry references.
7. Confidence model caps scores for untrusted evidence.
8. SARIF parser accepts Checkov/Trivy output.
9. Evidence entries include all version fields.
10. At least 65 tests pass (golden + noise + shape + signal + evidence).

**Approved by:**
*Chief Architect, Evidra Benchmark Project*
