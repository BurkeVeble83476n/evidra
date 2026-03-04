# v0.5.0 Implementation Backlog

**Source:** CHIEF_ARCHITECT_POST_IMPLEMENTATION_REVIEW.md (v0.5.0 tier)

## Items

### 1. Wire Ed25519 Signing into Evidence Pipeline

**Current state:** `internal/evidence/signer.go` has a complete Ed25519 signing
module (149 LOC, 14 tests). `EvidenceEntry.Signature` field exists but is always
empty string. No integration point in `BuildEntry()` or MCP server.

**Work required:**
- Add `Signer` parameter to `BuildEntry()` or wrap it
- Consume `EVIDRA_SIGNING_KEY` / `EVIDRA_SIGNING_KEY_PATH` env vars in MCP server
- Populate `Signature` field on every evidence entry
- Add verification in `ValidateChainAtPath`
- Update CLI scorecard to verify signatures if present

**Effort:** ~2 days

### 2. Forward Integrity + Server Receipts

**Current state:** `EntryTypeReceipt` exists as an enum value but no code
path creates receipt entries. `--forward-url` was removed in v0.3.0.

**Work required:**
- Add `EVIDRA_API_URL` config back to MCP server
- Implement HTTP forwarder (POST evidence entries to remote API)
- Remote API returns signed receipt -> write as `receipt` entry
- Receipt entry links back to forwarded entry by entry_id

**Effort:** ~3 days

### 3. Actor auth_context / OIDC

**Current state:** `Actor` struct has `Type`, `ID`, `Provenance`. No
authentication or identity verification.

**Work required:**
- Add `AuthContext` field to Actor (optional JWT/OIDC token reference)
- MCP server validates token if present
- Evidence entries carry verified actor identity
- Confidence model considers actor verification level

**Effort:** ~3 days

### 4. Multi-Tenancy Enforcement

**Current state:** `EvidenceEntry.TenantID` field exists (omitempty).
No enforcement or isolation logic.

**Work required:**
- MCP server requires tenant_id in service mode
- Evidence store partitions by tenant_id
- Scorecard filters by tenant_id
- Cross-tenant access prevention

**Effort:** ~3 days

### 5. Compare --versions (Agent Version Comparison)

**Current state:** `evidra compare` compares different actors by workload
profile (Jaccard similarity). No version-based comparison exists.
The doc describes `evidra compare --actor claude-code --versions v1.2,v1.3`.

**Work required:**
- Add `agent_version` and `model_id` to Actor or actor_meta in prescribe input
- Store in evidence entries
- Add `--versions` flag to `cmdCompare`
- Filter entries by actor+version, compute per-version scorecards
- Always valid comparison (same agent, same workload)

**Effort:** ~2 days

### 6. Scorecard BY TOOL / BY SCOPE Breakdowns

**Current state:** Scorecard outputs aggregate score only. Tool/scope
filtering exists via `--tool`/`--scope` flags, but no grouped breakdown.
The doc shows per-tool and per-scope score tables.

**Work required:**
- Group signal entries by tool and scope_class
- Compute per-group scores (score, ops, drift rate, retry rate)
- Add `by_tool` and `by_scope` sections to scorecard JSON output
- Consider text table output mode for human readability

**Effort:** ~2 days

### 7. Compare --force and Overlap Warning

**Current state:** `cmdCompare` computes `workload_overlap` silently
in JSON output. No warning to stderr when overlap is low.

**Work required:**
- Add `--force` flag to suppress warning
- Print warning to stderr when overlap < 0.3 and --force not set
- Include overlap percentage and per-actor tool/scope lists in warning

**Effort:** ~0.5 days
