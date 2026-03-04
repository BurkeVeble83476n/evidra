# Evidra — Core Data Model

## Status
**Normative.** This document defines the core data model. All
implementations MUST conform to these schemas.

## Purpose

Precise schema for the objects that appear in the architecture:

- CanonicalAction
- Prescription
- Report
- ValidatorFinding
- EvidenceEntry
- Signal
- Scorecard

The goal is to make the architecture deterministic, replayable,
and stable during the full codebase refactor.

---

## 1. CanonicalAction

CanonicalAction is the normalized representation of an
infrastructure action. Produced by canonicalization adapters
(server-side) or by self-aware tools (pre-canonicalized path).

| Field | Type | Description |
|-------|------|-------------|
| tool | string | Tool identifier (kubectl, terraform, helm, ...) |
| operation_class | string | mutate, destroy, read, plan |
| scope_class | string | production, staging, development, unknown |
| resource_identity | []ResourceID | Normalized resource identifiers |
| resource_count | integer | Number of resources affected |
| resource_shape_hash | string | SHA256 of normalized spec (for retry detection) |

### ResourceID

| Field | Type | Description |
|-------|------|-------------|
| api_version | string | K8s: e.g. "apps/v1" |
| kind | string | K8s: e.g. "Deployment" |
| namespace | string | K8s: e.g. "prod" |
| name | string | Resource name |
| type | string | Terraform: e.g. "aws_s3_bucket" |
| actions | string | Terraform: e.g. "create", "update" |

Fields are tool-specific. K8s uses api_version/kind/namespace/name.
Terraform uses type/name/actions.

### Digest Rules

```
intent_digest  = SHA256(canonical_json(canonical_action))
artifact_digest = SHA256(raw_artifact_bytes)
```

- `intent_digest` identifies behavioral identity.
- `artifact_digest` ensures artifact integrity.
- Same artifact_digest → same intent_digest. Not the reverse.
- They MUST NOT be treated as interchangeable.

---

## 2. Prescription

Prescription records intent before execution.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| prescription_id | ULID | MUST | Globally unique identifier |
| tenant_id | string | MUST (service mode) | Empty in local mode |
| trace_id | string | MUST | Automation task/session correlation key |
| actor | Actor | MUST | Who is performing the action |
| canonical_action | CanonicalAction | MUST | Normalized action (contains tool) |
| intent_digest | string | MUST | SHA256 of canonical JSON |
| artifact_digest | string | MUST | SHA256 of raw artifact bytes |
| risk_level | string | MUST | From risk matrix (low, medium, high, critical) |
| risk_tags | []string | MAY | From catastrophic risk detectors |
| risk_details | []string | MAY | Human-readable risk descriptions |
| ttl_ms | integer | MUST | Time-to-live in milliseconds (materialized, not inferred) |
| canon_source | string | MUST | "adapter" (Evidra parsed) or "external" (tool self-reported) |
| timestamp | datetime | MUST | RFC 3339, UTC |

### Actor

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | MUST | ai_agent, ci, human, unknown |
| id | string | MUST | Stable identifier |
| provenance | string | MUST | mcp, cli, api, oidc, git, manual |

---

## 3. Report

Report records the result after execution.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| report_id | ULID | MUST | Globally unique identifier |
| prescription_id | ULID | MUST | Links to the prescription |
| trace_id | string | MUST | Same trace_id as prescription |
| actor | Actor | MUST | Who executed |
| exit_code | integer | MUST | Tool exit code |
| artifact_digest | string | MUST | SHA256 of artifact at execution time (for drift detection) |
| verdict | string | MUST | success, failure, error |
| timestamp | datetime | MUST | RFC 3339, UTC |

### Matching Rules

1. prescription_id is globally unique (ULID).
2. First report wins. Second report for same prescription_id
   → `duplicate_report` protocol violation.
3. Report with unknown prescription_id → `unprescribed_action`
   protocol violation.
4. Cross-actor report (report.actor.id != prescription.actor.id)
   → `cross_actor_report` protocol violation.
5. Relationship is strictly 1:1. Batched apply (e.g. terraform
   apply with 10 resources) = one prescription with
   resource_count=10, one report.

### Findings Are NOT on Report

Validator findings are independent evidence entries (type=finding),
linked to operations by `artifact_digest`. This decouples scanner
timing from the prescribe/report lifecycle. See §4.

---

## 4. ValidatorFinding

Normalized external scanner output. Written as independent
evidence entries (type=finding), NOT embedded in Report.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| tool | string | MUST | Scanner name (checkov, trivy, tfsec, ...) |
| rule_id | string | MUST | Scanner rule identifier |
| severity | string | MUST | high, medium, low, info |
| resource | string | MUST | Affected resource identifier |
| message | string | MUST | Human-readable finding description |
| artifact_digest | string | MUST | Links finding to the operation's artifact |

Findings may arrive before, during, or after execution. The
linking key is `artifact_digest` — the same digest that appears
on the prescription and report for that operation.

---

## 5. EvidenceEntry

Append-only event log entry. Every JSONL line is one EvidenceEntry.
All entry types share the same envelope.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| entry_id | ULID | MUST | Globally unique, monotonically increasing per writer |
| previous_hash | string | MUST | Hash of previous entry (empty for first in segment) |
| hash | string | MUST | Hash of this entry (all fields except hash itself) |
| signature | string | MUST | Ed25519 signature of hash |
| type | string | MUST | Closed enum (see Entry Types) |
| tenant_id | string | MUST (service mode) | Empty in local mode |
| trace_id | string | MUST | Automation task/session correlation key |
| actor | Actor | MUST (prescribe, report) | MAY be empty on signal, receipt |
| timestamp | datetime | MUST | RFC 3339, UTC |
| intent_digest | string | conditional | Present on prescription entries |
| artifact_digest | string | conditional | Present on prescription, report, finding entries |
| payload | object | MUST | Type-specific content |
| spec_version | string | MUST | Signal spec version |
| canonical_version | string | MUST | Adapter canon version (e.g. "k8s/v1") |
| adapter_version | string | MUST | Evidra adapter version |
| scoring_version | string | MUST | Scoring model version (empty if unknown) |

### Entry Types

| Type | When written | Payload contains |
|------|-------------|-----------------|
| `prescribe` | prescribe() processes an artifact | Prescription fields |
| `report` | report() records execution outcome | Report fields |
| `finding` | Scanner output (independent, linked by artifact_digest) | ValidatorFinding fields |
| `signal` | Signal detector fires (at scorecard time) | signal_name, sub_signal, entry_refs, details |
| `receipt` | evidra-api acknowledges forwarded batch (v0.5.0+) | batch_id, entry_count, server_ts |
| `canonicalization_failure` | Adapter fails to parse artifact | error_code, error_message, adapter, raw_digest |

### Schema Rules

1. `type` is a closed enum. Adding a new type requires a spec
   version bump.
2. Timestamp is always UTC. Ordering is by entry position in
   chain, not by timestamp.
3. Entries are immutable. Corrections are new entries.
4. Hash chain creates append-only integrity. Insertion,
   reordering, or modification is detectable during verification.
5. Verification is possible offline.

---

## 6. Signal

Signals represent detected automation reliability behavior.
Signals are binary (detected or not) — they do not carry
per-signal confidence. Confidence is a scorecard-level property.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| signal_id | ULID | MUST | Globally unique |
| trace_id | string | MUST | Correlation key |
| name | string | MUST | Signal name (see Core Signals) |
| sub_signal | string | MAY | Sub-classification (e.g. stalled_operation, crash_before_report) |
| severity | string | MUST | Severity level |
| evidence_refs | []entry_id | MUST | Entry IDs that triggered the signal |
| details | object | MAY | Additional context |

### Core Signals

| Signal | What it detects |
|--------|----------------|
| protocol_violation | Missing report, missing prescribe, duplicate report, cross-actor report |
| artifact_drift | Agent changed artifact between prescribe and report |
| retry_loop | Same actor + same intent + same shape, repeated after failure within time window |
| blast_radius | Operation affects too many resources for its operation_class |
| new_scope | First operation in a (tool, operation_class, scope_class) tuple |

---

## 7. Scorecard

Scorecard summarizes reliability over a dataset.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| actor_id | string | MUST | Actor being scored |
| period | string | MUST | Time period (e.g. "30d") |
| total_operations | integer | MUST | Operation count in period |
| score | float | MUST | 0-100, computed from penalty |
| band | string | MUST | excellent, good, fair, poor, insufficient_data |
| confidence | float | MUST | Scorecard-level confidence (see Confidence Model) |
| signals | SignalRates | MUST | Per-signal rates |
| top_signals | []string | MUST | Top contributing signals to penalty |
| evidence_refs | []entry_id | MUST | Supporting evidence entries |
| scoring_version | string | MUST | Scoring model version |
| spec_version | string | MUST | Signal spec version |
| canon_version | string | MUST | Canonicalization version |
| evidra_version | string | MUST | Evidra binary version |
| generated_at | datetime | MUST | When scorecard was computed |

### SignalRates

| Field | Type | Description |
|-------|------|-------------|
| protocol_violation_rate | float | violations / total_ops |
| drift_rate | float | drifts / total_reports |
| retry_rate | float | retry_events / total_ops |
| blast_rate | float | blast_events / total_ops |
| scope_rate | float | scope_events / total_ops |

### Score Formula

```
score = 100 * (1 - penalty)

penalty = 0.35 * violation_rate
        + 0.30 * drift_rate
        + 0.20 * retry_rate
        + 0.10 * blast_rate
        + 0.05 * scope_rate
```

| Band | Score | Meaning |
|------|-------|---------|
| Excellent | 99-100 | Production-ready |
| Good | 95-99 | Minor issues |
| Fair | 90-95 | Needs attention |
| Poor | <90 | Unreliable |

Minimum sample: 100 operations. Below that: band = "insufficient_data".

### Confidence Model

Confidence is a scorecard-level property, not per-signal.

```
confidence = f(evidence_completeness, canon_trust, actor_trust)
```

| Confidence | Score ceiling | Condition |
|------------|--------------|-----------|
| High | 100 (no cap) | Full evidence, adapter-canonicalized, verified identity |
| Medium | 95 | >50% canon_source=external, or unverified actor with no tenant_id |
| Low | 85 | >10% protocol_violation_rate, or severe evidence gaps |

---

## 8. Core Data Flow

```
prescribe(raw_artifact)
    │
    ▼
Prescription ──────► EvidenceEntry (type=prescribe)
    │
    │  agent executes
    ▼
report(prescription_id, exit_code, artifact_digest)
    │
    ▼
Report ────────────► EvidenceEntry (type=report)

findings (async) ──► EvidenceEntry (type=finding)
                     linked by artifact_digest

signals (on demand) ► EvidenceEntry (type=signal)

scorecard (on demand)  computed from evidence entries
```

All signals and scores are derived strictly from the evidence log.

---

## 9. Frozen Enums

All enum values are closed sets. Adding a value requires a spec
version bump.

### operation_class

| Value | Meaning |
|-------|---------|
| `read` | Read-only operation (get, describe, list) |
| `mutate` | Create or update operation (apply, patch) |
| `destroy` | Delete operation (delete, destroy) |
| `plan` | Dry-run operation (plan, diff) |

### scope_class

| Value | Meaning | Resolution |
|-------|---------|------------|
| `production` | Production environment | Explicit `--env` flag, or namespace contains "prod" |
| `staging` | Staging environment | Namespace contains "stag" |
| `development` | Development environment | Namespace contains "dev" |
| `unknown` | Cannot determine | Default when no match |

### risk_level

| Value | Meaning |
|-------|---------|
| `low` | Routine operation |
| `medium` | Elevated risk, worth noting |
| `high` | Significant risk, agent should consider human approval |
| `critical` | Catastrophic risk pattern detected |

### entry_type

| Value | Written by |
|-------|-----------|
| `prescribe` | prescribe() call |
| `report` | report() call |
| `finding` | Scanner output (independent, linked by artifact_digest) |
| `signal` | Signal detector (at scorecard time) |
| `receipt` | evidra-api acknowledgment (v0.5.0+) |
| `canonicalization_failure` | Adapter parse failure |

### verdict (on report)

| Value | Meaning |
|-------|---------|
| `success` | exit_code == 0 |
| `failure` | exit_code != 0 |
| `error` | Execution could not complete |

### band (on scorecard)

| Value | Score range |
|-------|-----------|
| `excellent` | 99-100 |
| `good` | 95-99 |
| `fair` | 90-95 |
| `poor` | <90 |
| `insufficient_data` | <100 operations |

---

## 10. trace_id Generation Rules

| Context | trace_id lifecycle | Generation |
|---------|-------------------|------------|
| evidra-mcp | One MCP server process = one trace_id | ULID generated at server startup |
| evidra CLI (prescribe) | One command invocation = one trace_id | ULID generated per `evidra prescribe`, included in output |
| evidra CLI (report) | Same trace_id as corresponding prescribe | Passed via `--trace-id` flag (from prescribe output) |
| evidra-api | One API request session | ULID generated per request, or caller-provided |

Rules:
1. trace_id MUST be a ULID.
2. A single trace_id MAY span multiple prescribe/report pairs
   (e.g. multi-resource apply).
3. A trace_id MUST NOT span multiple actors.
4. A trace_id MUST NOT span multiple tenants.
5. If trace_id is not provided by the caller, Evidra generates one.

---

## 11. Invariants

1. Evidence is append-only.
2. Signals derive from evidence only.
3. Validators produce findings; Evidra produces signals.
4. Canonicalization defines intent identity.
5. Evidence replay MUST produce identical signals and scores.
6. Findings are independent entries, not embedded in reports.
7. Confidence is scorecard-level, not per-signal.
8. All digests use `sha256:` prefix format.
9. Enum values are closed sets (§9). New values require spec version bump.
10. intent_digest excludes resource_shape_hash (hashes identity fields only).
