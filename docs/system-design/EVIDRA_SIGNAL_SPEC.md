# Evidra Signal Specification v1.0

## Status
Stable. All five signals defined in this document are v1.0 stable.
Breaking changes require a major version bump (v2.0).

---

## Purpose

This document is the formal specification for Evidra signals.
It defines the detection contract, metric contract, and stability
guarantees for each signal.

Implementations MUST follow this spec to produce comparable
results. A conforming implementation emits the same signals
given the same evidence chain, regardless of language or platform.

---

## Signal Model

Every signal follows the same structure:

```
Signal:
  name:       unique identifier (snake_case)
  version:    semver (major.minor)
  status:     experimental | stable | deprecated
  input:      what the detector reads
  algorithm:  how the detector decides
  output:     what the detector produces
  metric:     Prometheus metric name and labels
  weight:     default weight in reliability score
```

A signal detector is a pure function:

```
detect(entry, evidence_chain) → SignalEvent | nil
```

No side effects. No external I/O. No state beyond the evidence
chain. Deterministic: same input → same output.

---

## Signal Registry

| Name | Version | Status | Weight |
|------|---------|--------|--------|
| protocol_violation | 1.0 | stable | 0.35 |
| artifact_drift | 1.0 | stable | 0.30 |
| retry_loop | 1.0 | stable | 0.20 |
| blast_radius | 1.0 | stable | 0.10 |
| new_scope | 1.0 | stable | 0.05 |

---

## Signal 1: protocol_violation

### Identity
```
name:    protocol_violation
version: 1.0
status:  stable
```

### Detection Contract

**Input:** A prescription entry or report entry from the evidence
chain, plus the full chain for context.

**Algorithm:**

```
For each prescription in the scoring window:
  1. Find matching report (same prescription_id)
  2. If no matching report within TTL → FIRE (unreported_prescription)
  3. If matching report has different actor.id → FIRE (cross_actor_report)

For each report in the scoring window:
  1. Find matching prescription (same prescription_id)
  2. If no matching prescription → FIRE (unprescribed_action)
  3. If a previous report with same prescription_id exists → FIRE (duplicate_report)
```

**Parameters:**
- TTL: default 10 minutes, configurable per scorecard invocation
- TTL detection happens at scorecard computation time, not real-time

**Sub-signals:**

| Sub-signal | Trigger | Meaning |
|------------|---------|---------|
| unreported_prescription | prescription without report within TTL | Agent didn't report |
| unprescribed_action | report without matching prescription | Agent acted without prescribing |
| duplicate_report | second report for same prescription_id | Agent reported twice |
| cross_actor_report | report actor != prescription actor | Wrong agent reported |
| stalled_operation | unreported + no further agent activity | Agent is hung |
| crash_before_report | unreported + agent sent new prescribe | Agent crashed and restarted |

Sub-signals are informational breakdowns. All count as
protocol_violation in the score.

**Output:**
```go
type SignalEvent struct {
    Signal    string    // "protocol_violation"
    SubSignal string    // "unreported_prescription", "stalled_operation", etc.
    Timestamp time.Time
    EntryRef  string    // prescription_id or report entry_id that triggered
    Details   string    // human-readable description
}
```

### Metric Contract

```
evidra_signal_total{signal="protocol_violation", agent, tool, scope}
```

Counter. Incremented by 1 for each protocol violation detected.
Rate: `rate(evidra_signal_total{signal="protocol_violation"}[5m])`

### Score Contribution

```
violation_rate = protocol_violation_count / total_operations
penalty_contribution = 0.35 × violation_rate
```

total_operations = prescriptions + unprescribed reports (deduplicated).

---

## Signal 2: artifact_drift

### Identity
```
name:    artifact_drift
version: 1.0
status:  stable
```

### Detection Contract

**Input:** A report entry with its matching prescription.

**Algorithm:**

```
For each report with matching prescription:
  If prescription.artifact_digest != report.artifact_digest → FIRE
```

**Edge cases:**
- Report without matching prescription → protocol_violation, not drift
- Report with no artifact_digest → no drift check (field optional for
  tools that don't produce artifacts at report time)

**Output:**
```go
type SignalEvent struct {
    Signal    string    // "artifact_drift"
    Timestamp time.Time
    EntryRef  string    // report entry_id
    Details   string    // "prescribed sha256:abc..., reported sha256:def..."
}
```

### Metric Contract

```
evidra_signal_total{signal="artifact_drift", agent, tool, scope}
```

### Score Contribution

```
drift_rate = artifact_drift_count / total_reports
penalty_contribution = 0.30 × drift_rate
```

total_reports = reports with matching prescriptions (excludes
unprescribed reports — those are protocol_violation).

### Trust Model

Artifact drift measures protocol consistency, not ground truth.
Both digests are self-reported by the agent. An agent that lies
consistently (sends same digest both times but applies something
different) shows zero drift. Evidra detects inconsistency within
the protocol, not real-world compliance.

---

## Signal 3: retry_loop

### Identity
```
name:    retry_loop
version: 1.0
status:  stable
```

### Detection Contract

**Input:** A prescription entry, plus recent prescriptions from
the same actor.

**Algorithm:**

```
For each new prescription:
  Find all prescriptions from same actor within retry_window where:
    intent_digest matches AND
    resource_shape_hash matches AND
    previous operation was denied OR failed (exit_code != 0)
  
  If count >= retry_threshold → FIRE
```

**Parameters:**
- retry_window: default 30 minutes
- retry_threshold: default 3 (third identical attempt fires)

**Key distinction:**
- Same intent_digest + same shape_hash → retry (agent sending
  identical content after failure)
- Same intent_digest + different shape_hash → NOT retry (agent
  modified the artifact between attempts — this is fixing, not
  retrying)

**Output:**
```go
type SignalEvent struct {
    Signal    string    // "retry_loop"
    Timestamp time.Time
    EntryRef  string    // prescription_id that triggered
    Details   string    // "3rd identical attempt within 12min, all failed"
}
```

### Metric Contract

```
evidra_signal_total{signal="retry_loop", agent, tool, scope}
```

### Score Contribution

```
retry_rate = retry_loop_count / total_prescriptions
penalty_contribution = 0.20 × retry_rate
```

---

## Signal 4: blast_radius

### Identity
```
name:    blast_radius
version: 1.0
status:  stable
```

### Detection Contract

**Input:** A prescription with canonical_action.

**Algorithm:**

```
For each prescription:
  If canonical_action.operation_class == "destructive"
     AND canonical_action.resource_count > blast_threshold → FIRE
```

**Parameters:**
- blast_threshold: default 5 resources per destructive operation

**Scope:**
- Only fires on destructive operations (delete, destroy, uninstall)
- Mutating operations with high resource_count are not flagged
  (deploying 20 services is normal; deleting 20 is suspicious)

**Generic adapter limitation:** resource_count is always 1 for
generic adapter. Blast radius effectively disabled for unknown tools.
This is acceptable — the signal fires for K8s and Terraform where
resource counting is reliable.

**Pre-canonicalized limitation:** resource_count is caller-provided.
If the caller lies, blast_radius is inaccurate. Documented trade-off.

**Output:**
```go
type SignalEvent struct {
    Signal    string    // "blast_radius"
    Timestamp time.Time
    EntryRef  string    // prescription_id
    Details   string    // "destructive operation on 12 resources (threshold: 5)"
}
```

### Metric Contract

```
evidra_signal_total{signal="blast_radius", agent, tool, scope}
```

### Score Contribution

```
blast_rate = blast_radius_count / total_prescriptions
penalty_contribution = 0.10 × blast_rate
```

---

## Signal 5: new_scope

### Identity
```
name:    new_scope
version: 1.0
status:  stable
```

### Detection Contract

**Input:** A prescription with canonical_action, plus full evidence
chain history.

**Algorithm:**

```
scope_key = (actor.id, tool, operation_class, scope_class)

For each prescription:
  Search evidence chain for any prior prescription with same scope_key
  If no prior prescription → FIRE (first time this actor operates
  in this tool/operation_class/scope_class combination)
```

**Scope:** Fires once per unique scope_key. After the first
occurrence, subsequent operations with the same key do not fire.

**Example:**
```
claude-code does kubectl/mutating/staging → first time → FIRE
claude-code does kubectl/mutating/staging → second time → no fire
claude-code does kubectl/mutating/production → first time → FIRE
claude-code does terraform/destructive/production → first time → FIRE
```

**Output:**
```go
type SignalEvent struct {
    Signal    string    // "new_scope"
    Timestamp time.Time
    EntryRef  string    // prescription_id
    Details   string    // "first kubectl/mutating/production operation for claude-code"
}
```

### Metric Contract

```
evidra_signal_total{signal="new_scope", agent, tool, scope}
```

### Score Contribution

```
scope_rate = new_scope_count / total_prescriptions
penalty_contribution = 0.05 × scope_rate
```

new_scope has the lowest weight. It's informational — entering
a new scope is often legitimate (first deploy to production).

---

## Reliability Score Formula

```
score = 100 × (1 - penalty)

penalty = Σ(weight_i × rate_i) for all 5 signals

where:
  rate_i = signal_count_i / denominator_i
  
  denominators:
    protocol_violation:  total_operations
    artifact_drift:      total_reports (with matching prescriptions)
    retry_loop:          total_prescriptions
    blast_radius:        total_prescriptions
    new_scope:           total_prescriptions
```

Score range: 0–100. Clamped (never negative).

Minimum sample: 100 operations. Below that, score is not computed
("insufficient data").

Default weights:

```
protocol_violation:  0.35
artifact_drift:      0.30
retry_loop:          0.20
blast_radius:        0.10
new_scope:           0.05
```

Weights are configurable. Sum must equal 1.0.

---

## Stability Guarantees

### What is stable (v1.0)

- Signal names (the five names above)
- Detection algorithms (the logic described above)
- Metric names and label keys
- Score formula structure
- Default parameter values

### What can change without version bump

- Default weights (tuning based on real-world data)
- Adding new sub-signals to existing signals
- Adding new labels to metrics (must be low-cardinality)
- Adding new optional parameters with backward-compatible defaults

### What requires version bump (v2.0)

- Changing detection algorithm for any signal
- Removing a signal
- Changing a metric name
- Changing the score formula structure
- Changing a parameter's default value in a way that would change
  detection results for existing evidence chains

### Signal lifecycle

```
experimental → stable → deprecated → removed
```

- experimental: may change without notice. Not counted in score.
- stable: changes require version bump. Counted in score.
- deprecated: still emitted, replacement exists. Counted in score.
- removed: no longer emitted. Version bump required.

Transition requires at least one minor version with both old and
new signal emitted simultaneously (except experimental → stable).

---

## Conformance

An implementation is conforming if:

1. Given the same evidence chain, it produces the same signal
   events as the reference implementation.
2. It exports metrics with the specified names and labels.
3. It computes reliability score using the specified formula.

Conformance test suite: a set of evidence chain fixtures with
expected signal events and scores. Available in the reference
implementation repository under `tests/signal_conformance/`.
