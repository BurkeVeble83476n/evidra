# Chief Architect Review: Evidra Benchmark v0.3.0

**Date:** March 4, 2026  
**Status:** MANDATORY ARCHITECTURAL GUIDANCE  
**Subject:** Alignment of Implementation with System Design Contract v1

---

## 1. Executive Summary

The project has undergone a fundamental pivot from a **Policy Enforcement Engine** (OPA-based) to a **Reliability Benchmark & Behavioral Telemetry Layer**. While the System Design (under `@docs/system-design/**`) is high-quality and frozen, the implementation (largely ported from the legacy `evidra-mcp` repo) is in a "corrupted" state containing OPA remnants that violate the new Inspector Model invariants.

We must purge the "Enforcer" mindset and fully commit to the "Inspector" model to achieve v0.3.0 stability.

---

## 2. Architectural Alignment Review

### 2.1 The Protocol: Prescribe/Report vs. Validate
*   **System Design Requirement:** A 1:1 1-way protocol: `prescribe` (intent) and `report` (outcome).
*   **Current State:** The MCP server still exposes a `validate` tool. This is a legacy OPA concept.
*   **Recommendation:** Rename MCP tools immediately. `validate` must be aliased to `prescribe`. The `report` tool must be implemented as the primary signal source for `artifact_drift` and `protocol_violation`.

### 2.2 Canonicalization & The "Moat"
*   **System Design Requirement:** Two independent digests: `artifact_digest` (raw integrity) and `intent_digest` (semantic identity).
*   **Current State:** The K8s and Terraform adapters are well-implemented, but the "Generic" adapter is a stub.
*   **Recommendation:** The **Golden Corpus** is our strategic moat. We must ensure that the `resource_shape_hash` is sensitive enough to catch image tag changes (for `retry_loop` detection) but ignores metadata noise (to prevent false drift).

### 2.3 Evidence Integrity
*   **System Design Requirement:** Append-only, hash-linked, Ed25519 signed JSONL.
*   **Current State:** The code contains two `EvidenceRecord` structs. One in `internal/evidence` and one in `pkg/evidence`. Both contain OPA fields like `PolicyRef` and `BundleRevision`.
*   **MANDATE:** Unify into a single `EvidenceEntry` struct defined in `EVIDRA_CORE_DATA_MODEL.md`. **Remove all OPA fields.** Any field not required for signal computation or identity is "contextual noise" and should be moved to an optional `metadata` map.

---

## 3. Critical Gaps (v0.3.0 Blockers)

| Gap ID | Issue | Architect's Direction |
|:---|:---|:---|
| **GAP-01** | **ScopeClass Logic** | The code currently identifies scope by resource count ("single" vs "cluster"). **REJECTED.** Scope MUST be environment-based (prod/staging/dev) derived from the `environment` field or namespace substrings per `CANONICALIZATION_CONTRACT_V1.md §10`. |
| **GAP-02** | **CLI Scorecard** | The `evidra scorecard` command is a stub. It MUST be wired to the `pkg/evidence` reader to perform a single-pass scan of local JSONL and compute the 5 signals. |
| **GAP-03** | **Scanner Integration** | We are missing the SARIF parser. We should not build custom detectors for everything. We must consume SARIF from Checkov/Trivy and map them to `risk_tags` in the Prescription. |
| **GAP-04** | **TTL Detection** | Real-time TTL is a non-goal for v0.3.0. Detection MUST happen at scorecard-time by identifying prescriptions without matching reports. |

---

## 4. Strategic Positioning: The "Inspector" Moat

We are not competing with **Gatekeeper** or **Kyverno**. They are "Judges." We are the "Flight Recorder."
*   **DO NOT** add "Deny" logic.
*   **DO NOT** add "Auto-remediation."
*   **DO** focus on **Attribution**. The `actor.id` and `trace_id` are our most important metadata.

---

## 5. Implementation Roadmap (Phased)

### Phase 1: Pure Foundation (P0)
1.  **Unify Evidence Types:** Remove OPA remnants (`PolicyRef`, etc.).
2.  **Protocol Rename:** MCP tools become `prescribe` and `report`.
3.  **CLI Wiring:** Ensure `evidra prescribe` actually writes to the same JSONL as the MCP server.

### Phase 2: Reach & Distribution (P1)
1.  **SARIF Parser:** Enable `--scanner-report` to ingest Checkov/Trivy results.
2.  **Scorecard "Explain":** Add a command that doesn't just show a score, but lists the specific `entry_id`s that caused penalties.

### Phase 3: Polish (P2)
1.  **Safety Floors:** Implement the logic where a high `protocol_violation_rate` caps the score at 85 regardless of other metrics.

---

## 6. Conclusion

The System Design is solid. The codebase is "mostly there" but carries the weight of a previous product iteration. As Chief Architect, I authorize a **Surgical Refactor** to align the Go types with the `EVIDRA_CORE_DATA_MODEL.md`. 

**Finality is only achieved through validation.** Every change must be verified against the Golden Corpus.

**Approved by:**
*Chief Architect, Evidra Benchmark Project*
