# Evidra — Implementation Prompt

**Give this file as context to Claude Code or any coding agent.**
**It describes exactly what code changes to make, in what order.**

---

## Context

You are working on the Evidra project — an infrastructure automation reliability benchmark.
The codebase is in Go. The repo is at `~/evidra-benchmark/` (or wherever it's cloned).

Read these files first to understand current architecture:
- `internal/canon/types.go` — Adapter interface, CanonicalAction, CanonResult
- `internal/risk/detectors.go` — Current detectors (7 detectors, all in one file)
- `internal/risk/matrix.go` — Risk matrix (RiskLevel, ElevateRiskLevel)
- `internal/signal/` — Signal detectors (retry_loop, protocol_violation, etc.)
- `internal/lifecycle/service.go` — Where prescribe calls detectors (line ~58: `risk.RunAll`)
- `pkg/mcpserver/server.go` — MCP server (PrescribeOutput, ReportOutput)

---

## Architecture Notes (verified from code)

**Signals are already clean.** Verified: none of the 5 signal detectors read `risk_level` or `risk_tags`. They depend only on operational data: intent_digest, exit_code, artifact_digest, resource_count, operation_class, scope_class. Do not add risk_level dependencies to signals.

**Confidence already exists.** `Scorecard.Confidence` has Level (low/medium/high) and ScoreCeiling. `MinOperations=100` gates scoring. No changes needed.

**Detectors must not parse deeply.** Detection logic should use adapter helpers (ParseK8sYAML, ParsePlan), not re-parse raw bytes. If a detector needs a new parsing capability, add it to helpers first.

---

## Task 1: Detector Architecture Refactor

### Goal

Split `internal/risk/detectors.go` (one monolithic file with 7 detectors) into self-registering individual files under `internal/detectors/`. Zero behavioral change — same tags, same severity, same output.

### Step 1.1: Create directory structure

```bash
mkdir -p internal/detectors/{k8s,terraform/{aws,gcp,azure},docker,ops,all}
```

### Step 1.2: Create registry (`internal/detectors/registry.go`)

New package `detectors` with:

```go
package detectors

type Stability string
const (
    Stable       Stability = "stable"
    Experimental Stability = "experimental"
    Deprecated   Stability = "deprecated"
)

type VocabularyLevel string
const (
    ResourceRisk  VocabularyLevel = "resource"
    OperationRisk VocabularyLevel = "operation"
)

type Detector interface {
    Tag() string
    BaseSeverity() string
    Detect(action canon.CanonicalAction, raw []byte) bool
    Metadata() TagMetadata
}

type TagMetadata struct {
    Tag          string          `json:"tag" yaml:"tag"`
    BaseSeverity string          `json:"base_severity" yaml:"base_severity"`
    Stability    Stability       `json:"stability" yaml:"stability"`
    Level        VocabularyLevel `json:"level" yaml:"level"`
    Domain       string          `json:"domain" yaml:"domain"`
    SourceKind   string          `json:"source_kind" yaml:"source_kind"`
    Summary      string          `json:"summary" yaml:"summary"`
}

// Global registry with Register(), All(), RunAll(), AllMetadata(), StableOnly()
// Use sync.RWMutex for thread safety
// Register() called from init() in each detector file
```

`RunAll(action, raw)` returns `[]string` (list of tags that fired). Same signature as current `risk.RunAll()`.

### Step 1.3: Move K8s helpers to `internal/detectors/k8s/helpers.go`

Move from `internal/risk/detectors.go`:
- `parseK8sYAML(raw []byte) []map[string]interface{}`
- `getPodSpec(obj map[string]interface{}) map[string]interface{}`
- `getAllContainers(obj map[string]interface{}) []map[string]interface{}`
- `getBool(m map[string]interface{}, key string) bool`

These become exported functions in the `k8s` package: `ParseK8sYAML`, `GetPodSpec`, `GetAllContainers`, `GetBool`.

### Step 1.4: Move TF helpers to `internal/detectors/terraform/helpers.go`

Move from `internal/risk/detectors.go`:
- `parsePlan` → `ParsePlan`
- `extractIAMStatements` → stays in `terraform/aws/iam_helpers.go` (AWS-specific)
- `isCompletePublicAccessBlock` → stays in `terraform/aws/s3_helpers.go` (AWS-specific)

Add new helpers:
- `ResourcesByType(plan, resourceType) []*ResourceChange`
- `HasResource(plan, resourceType) bool`
- `AfterValue(rc, key) (interface{}, bool)`
- `AfterBool(rc, key) bool`
- `AfterString(rc, key) string`

### Step 1.5: Split 7 existing detectors into individual files

Each detector becomes one file with `init()` self-registration:

| Current function | New file | Tag |
|-----------------|----------|-----|
| `DetectPrivileged` | `internal/detectors/k8s/privileged.go` | `k8s.privileged_container` |
| `DetectHostNamespace` | `internal/detectors/k8s/host_namespace.go` | `k8s.host_namespace_escape` |
| `DetectHostPath` | `internal/detectors/k8s/hostpath.go` | `k8s.hostpath_mount` |
| `DetectMassDestroy` | `internal/detectors/ops/mass_delete.go` | `ops.mass_delete` |
| `DetectWildcardIAM` | `internal/detectors/terraform/aws/iam_wildcard.go` | `aws.iam_wildcard_policy` |
| `DetectTerraformIAMWildcard` | `internal/detectors/terraform/aws/iam_wildcard_any.go` | `tf.iam_wildcard_policy` |
| `DetectS3PublicAccess` | `internal/detectors/terraform/aws/s3_public.go` | `aws.s3_public_access` |

**Pattern for each file:**

```go
package k8s  // or aws, ops, etc.

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "samebits.com/evidra-benchmark/internal/detectors"
)

func init() { detectors.Register(&Privileged{}) }

type Privileged struct{}

func (d *Privileged) Tag() string          { return "k8s.privileged_container" }
func (d *Privileged) BaseSeverity() string { return "critical" }
func (d *Privileged) Metadata() detectors.TagMetadata {
    return detectors.TagMetadata{
        Tag: "k8s.privileged_container", BaseSeverity: "critical",
        Stability: detectors.Stable, Level: detectors.ResourceRisk,
        Domain: "k8s", SourceKind: "k8s_yaml",
        Summary: "Container with securityContext.privileged=true",
    }
}
func (d *Privileged) Detect(_ canon.CanonicalAction, raw []byte) bool {
    // Move logic from DetectPrivileged, using k8s helpers
    for _, obj := range ParseK8sYAML(raw) {
        for _, c := range GetAllContainers(obj) {
            sc, ok := c["securityContext"].(map[string]interface{})
            if !ok { continue }
            if priv, ok := sc["privileged"].(bool); ok && priv {
                return true
            }
        }
    }
    return false
}
```

### Step 1.6: Create `internal/detectors/all/all.go`

```go
package all

import (
    _ "samebits.com/evidra-benchmark/internal/detectors/docker"
    _ "samebits.com/evidra-benchmark/internal/detectors/k8s"
    _ "samebits.com/evidra-benchmark/internal/detectors/ops"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/aws"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/azure"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/gcp"
)
```

### Step 1.7: Update callers

In `internal/lifecycle/service.go`, change:

```go
// OLD
riskTags := risk.RunAll(cr.CanonicalAction, input.RawArtifact)

// NEW
riskTags := detectors.RunAll(cr.CanonicalAction, input.RawArtifact)
```

Add import:
```go
import (
    "samebits.com/evidra-benchmark/internal/detectors"
    _ "samebits.com/evidra-benchmark/internal/detectors/all"
)
```

In `internal/risk/matrix.go`, the `ElevateRiskLevel` function stays — it uses tags but doesn't produce them. No change needed.

### Step 1.8: Create detector test contract (`internal/detectors/contract_test.go`)

Test that runs over ALL registered detectors and validates:
- Tag format: `^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`
- BaseSeverity: one of low/medium/high/critical
- Stability: one of stable/experimental/deprecated
- Level: one of resource/operation
- Summary: non-empty
- Tag() == Metadata().Tag
- BaseSeverity() == Metadata().BaseSeverity
- Idempotent: Detect(empty, empty) called twice gives same result

### Step 1.9: Delete old file

Delete `internal/risk/detectors.go` and `internal/risk/detectors_test.go`.
Move tests to per-detector test files.

### Step 1.10: Verify

```bash
make test
```

**All existing tests must pass.** The refactor changes internal organization only — external behavior is identical. Same tags, same severity, same evidence chain.

---

## Task 2: Add 13 New Detectors

After refactor, add new detectors. Each is ONE file + ONE test file.

### K8s detectors (`internal/detectors/k8s/`)

| File | Tag | BaseSeverity | Detection Logic |
|------|-----|-------------|----------------|
| `docker_socket.go` | `k8s.docker_socket` | critical | hostPath.path contains "docker.sock" |
| `run_as_root.go` | `k8s.run_as_root` | medium | runAsUser=0 OR runAsNonRoot absent/false in container securityContext |
| `capabilities.go` | `k8s.dangerous_capabilities` | high | capabilities.add contains SYS_ADMIN, NET_ADMIN, NET_RAW, or ALL |
| `cluster_admin.go` | `k8s.cluster_admin_binding` | critical | Kind=ClusterRoleBinding, roleRef.name=cluster-admin |
| `writable_rootfs.go` | `k8s.writable_rootfs` | low | readOnlyRootFilesystem absent or false |

### Ops detectors (`internal/detectors/ops/`)

| File | Tag | BaseSeverity | Detection Logic |
|------|-----|-------------|----------------|
| `kube_system.go` | `ops.kube_system` | high | Any K8s resource in namespace kube-system with op_class=mutate or destroy |
| `namespace_delete.go` | `ops.namespace_delete` | high | Kind=Namespace with operation=delete |

### Terraform/AWS detectors (`internal/detectors/terraform/aws/`)

| File | Tag | BaseSeverity | Detection Logic |
|------|-----|-------------|----------------|
| `security_group.go` | `aws.security_group_open` | high | aws_security_group ingress with cidr 0.0.0.0/0 on ports 22,3389,3306,5432 |
| `rds_public.go` | `aws.rds_public` | high | aws_db_instance with publicly_accessible=true |
| `ebs_unencrypted.go` | `aws.ebs_unencrypted` | medium | aws_ebs_volume without encrypted=true |

### Docker detectors (`internal/detectors/docker/`)

Need Docker compose parsing. Create `internal/detectors/docker/helpers.go` with `ParseCompose()`.

| File | Tag | BaseSeverity | Detection Logic |
|------|-----|-------------|----------------|
| `privileged.go` | `docker.privileged` | critical | service has privileged: true |
| `socket_mount.go` | `docker.socket_mount` | critical | volumes contains docker.sock |
| `host_network.go` | `docker.host_network` | high | network_mode: host |

For each detector, also create a test file with at least:
- One positive fixture (fires)
- One negative fixture (does not fire)

### Docker Adapter

Also need `internal/canon/docker.go`:

```go
type DockerComposeAdapter struct{}

func (a *DockerComposeAdapter) Name() string          { return "docker/v1" }
func (a *DockerComposeAdapter) CanHandle(tool string) bool {
    return tool == "docker" || tool == "docker-compose" || tool == "compose"
}
func (a *DockerComposeAdapter) Canonicalize(tool, operation, env string, raw []byte) (CanonResult, error) {
    // Parse docker-compose YAML
    // Extract services as resources
    // Map operation to operation_class
}
```

Add to `DefaultAdapters()` in `internal/canon/types.go`, before GenericAdapter.

---

## Task 3: Signal Validation

After detectors work, validate the signals engine.

Copy `tests/signal-validation/helpers.sh` and `tests/signal-validation/validate-signals-engine.sh` from the docs output.

Run:
```bash
export PATH="$PWD/bin:$PATH"
bash tests/signal-validation/validate-signals-engine.sh
```

Expected output: 5 sequences with different scores. If all sequences score the same, investigate signal detection thresholds.

---

## Task 4: Update MCP Prompts

Replace files in `prompts/mcpserver/` with the content from `docs/system-design/MCP_CONTRACT_PROMPTS.md`:

- `prompts/mcpserver/initialize/instructions.txt` — 12 lines, protocol overview
- `prompts/mcpserver/tools/prescribe_description.txt` — 200 words, WHEN TO CALL list
- `prompts/mcpserver/tools/report_description.txt` — 150 words, 6 explicit rules
- `prompts/mcpserver/resources/content/agent_contract_v1.md` — 350 words, full reference

---

## Order of Operations

```
1. Task 1 (refactor) — half day
   └── make test passes with zero behavioral change

2. Task 2 (new detectors) — 1-2 days
   └── make test passes with 20 detectors registered

3. Task 3 (signal validation) — 30 minutes
   └── 5 sequences produce meaningful score distribution
   └── THIS IS THE PRODUCT GATE

4. Task 4 (MCP prompts) — 15 minutes
   └── prompts replaced, make test passes

5. Task 5 (enhancements) — 1 hour
   └── signal profiles in scorecard, session docs

6. Task 6 (TagProducer) — half day
   └── interface + NativeProducer wrapper + SARIF scaffold
   └── ProduceAll() replaces RunAll() in lifecycle — zero behavioral change
   └── Trivy mapping YAML shipped but optional
```

Task 1 is the refactor gate. Do not start Task 2 until all existing tests pass.
Task 3 is the product gate. If scores don't differentiate, fix signals before anything else.
Task 6 is additive — doesn't change behavior, just enables future scanner integration.

---

## Success Criteria

1. `make test` passes (all existing tests)
2. `evidra detectors list` shows 20 registered detectors with metadata
3. `validate-signals-engine.sh` produces:
   - Sequence A (clean): score > 90
   - Sequence B (retry): score 50-70
   - Sequence C (protocol): score 40-65
   - Sequence D (blast): score 60-80
   - Sequence E (scope): score 80-95
4. Score distribution is meaningful — no two sequences have the same score

If criterion 3 fails: adjust signal weights in `internal/score/` before adding more detectors.
If criterion 4 fails: signals engine has a bug — investigate before proceeding.

---

## Task 5: Small Enhancements (after core tasks pass)

### 5.1: Signal Profile in Scorecard

Add `SignalProfile` to scorecard output — human-readable severity per signal:

```go
// In internal/score/score.go

type SignalProfile struct {
    Level string `json:"level"` // "none", "low", "medium", "high"
}

// Add to Scorecard struct:
SignalProfiles map[string]SignalProfile `json:"signal_profiles"`

// Compute profile from rates:
func signalProfileLevel(rate float64) string {
    switch {
    case rate == 0:
        return "none"
    case rate < 0.02:
        return "low"
    case rate < 0.10:
        return "medium"
    default:
        return "high"
    }
}
```

### 5.2: Add Two New Behavioral Signals

The data model already stores intent_digest + artifact_digest + exit_code per entry. Two new signals extract value from combinations of these fields that existing signals don't cover:

**repair_loop** — agent changed strategy and succeeded. This is GOOD behavior (unlike retry_loop which is bad). Detects: same intent_digest, different artifact_digest, eventual exit_code=0.

```go
// internal/signal/repair_loop.go

func DetectRepairLoop(entries []Entry) SignalResult {
    type key struct{ actor, intent string }

    // Group prescriptions by (actor, intent_digest)
    groups := make(map[key][]Entry)
    reportExit := make(map[string]*int)
    reportDigest := make(map[string]string)

    for _, e := range entries {
        if e.IsReport && e.PrescriptionID != "" {
            reportExit[e.PrescriptionID] = e.ExitCode
            reportDigest[e.PrescriptionID] = e.ArtifactDigest
        }
        if e.IsPrescription && e.IntentDigest != "" {
            k := key{e.ActorID, e.IntentDigest}
            groups[k] = append(groups[k], e)
        }
    }

    var eventIDs []string
    for _, group := range groups {
        if len(group) < 2 {
            continue
        }
        sort.Slice(group, func(i, j int) bool {
            return group[i].Timestamp.Before(group[j].Timestamp)
        })

        // Look for: fail with artifact A, then success with artifact B
        sawFailure := false
        failDigest := ""
        for _, p := range group {
            ec := reportExit[p.EventID]
            if ec != nil && *ec != 0 {
                sawFailure = true
                failDigest = p.ArtifactDigest
            }
            if sawFailure && ec != nil && *ec == 0 && p.ArtifactDigest != failDigest {
                eventIDs = append(eventIDs, p.EventID)
                sawFailure = false // reset for next potential chain
            }
        }
    }

    return SignalResult{
        Name:     "repair_loop",
        Count:    len(eventIDs),
        EventIDs: eventIDs,
    }
}
```

One sentence: **"Agent failed, changed the artifact, and succeeded."**

**thrashing** — agent trying many different strategies, none working. Worse than retry (at least retry is stable). Detects: N distinct intent_digests within a window, all failed.

```go
// internal/signal/thrashing.go

const ThrashingThreshold = 3 // minimum distinct intents, all failed

func DetectThrashing(entries []Entry) SignalResult {
    reportExit := make(map[string]*int)
    for _, e := range entries {
        if e.IsReport && e.PrescriptionID != "" {
            reportExit[e.PrescriptionID] = e.ExitCode
        }
    }

    // Collect sequential prescriptions
    var prescriptions []Entry
    for _, e := range entries {
        if e.IsPrescription {
            prescriptions = append(prescriptions, e)
        }
    }

    sort.Slice(prescriptions, func(i, j int) bool {
        return prescriptions[i].Timestamp.Before(prescriptions[j].Timestamp)
    })

    // Sliding window: track distinct failed intents with no success between them
    var eventIDs []string
    distinctIntents := make(map[string]bool)
    var windowEntries []Entry

    for _, p := range prescriptions {
        ec := reportExit[p.EventID]

        if ec != nil && *ec == 0 {
            // Success resets the window
            distinctIntents = make(map[string]bool)
            windowEntries = nil
            continue
        }

        // Failure or unreported — add to window
        distinctIntents[p.IntentDigest] = true
        windowEntries = append(windowEntries, p)

        if len(distinctIntents) >= ThrashingThreshold {
            for _, w := range windowEntries {
                eventIDs = append(eventIDs, w.EventID)
            }
            // Reset to avoid double counting
            distinctIntents = make(map[string]bool)
            windowEntries = nil
        }
    }

    return SignalResult{
        Name:     "thrashing",
        Count:    len(eventIDs),
        EventIDs: eventIDs,
    }
}
```

One sentence: **"Agent tried 3+ different approaches, none succeeded."**

**Scoring impact:**

| Signal | Type | Score Effect |
|--------|------|-------------|
| `repair_loop` | Positive | **Bonus** — reduces penalty (agent adapted) |
| `thrashing` | Negative | Penalty — agent has no strategy |

Add to weight map in `internal/score/score.go`. repair_loop weight = -0.05 (small bonus). thrashing weight = 0.15 (penalty between retry_loop and blast_radius).

**Architecture note:** Both signals are simple functions over `[]Entry`. No graph. The sequence + intent_digest + artifact_digest + exit_code is sufficient. Graph model can be added later as an optimization, but these signals don't require it.

Register both in the signal pipeline alongside existing 5 signals. Update `validate-signals-engine.sh` to add:

- Sequence F: repair scenario (fail with artifact A → change artifact → succeed) — expect repair_loop fires, bonus applied
- Sequence G: thrashing (3 different artifacts, all fail) — expect thrashing fires, penalty applied

Output becomes:
```json
{
  "score": 62,
  "band": "fair",
  "signal_profiles": {
    "retry_loop": { "level": "high" },
    "protocol_violation": { "level": "none" },
    "artifact_drift": { "level": "none" },
    "blast_radius": { "level": "low" },
    "new_scope": { "level": "medium" }
  }
}
```

More actionable than raw counts for users.

### 5.2: Session Model Documentation

Add a doc comment to `pkg/evidence/` clarifying session semantics:

```go
// Session represents one automation attempt — a CI pipeline run,
// an AI agent task, or a human operator's work session.
//
// All operations within a session share a session_id.
// If session_id is omitted in prescribe, a new one is generated
// per prescribe call (each operation becomes its own session).
//
// For meaningful signal detection, callers should generate one
// session_id at the start of their automation task and reuse it
// across all prescribe/report calls in that task.
//
// Session boundaries affect:
//   - retry_loop detection (retries within same session)
//   - new_scope detection (first-seen combos within session)
//   - scorecard computation (score is per-session)
```

This is documentation, not code change. But it prevents misuse.

---

## Task 6: TagProducer Interface (unifies native detectors + external scanners)

### Goal

Introduce `TagProducer` as the universal interface for anything that generates risk tags. Native detectors are one implementation. External scanners (Trivy, Checkov, Kubescape) become another. The signals engine never knows which producer generated a tag.

### Why now

The current `Detector` interface works for Go-native pattern matching. But the product needs external scanner integration — Trivy alone covers 800+ rules we'll never rewrite. TagProducer lets us plug in scanner output alongside native detectors with zero changes to the signals engine.

### Step 6.1: Define interface (`internal/detectors/producer.go`)

```go
package detectors

import "samebits.com/evidra-benchmark/internal/canon"

// TagProducer generates risk tags from an infrastructure operation.
// Implementations include native Go detectors and external scanner adapters.
type TagProducer interface {
    // Name identifies this producer (for provenance tracking).
    Name() string

    // ProduceTags returns risk tags detected in the given operation.
    // Tags must be dot-namespace format: "k8s.privileged_container"
    ProduceTags(action canon.CanonicalAction, raw []byte) []string
}
```

### Step 6.2: Wrap native detectors as TagProducer

```go
// internal/detectors/native_producer.go
package detectors

import "samebits.com/evidra-benchmark/internal/canon"

// NativeProducer wraps all registered Detector instances as a TagProducer.
type NativeProducer struct{}

func (p *NativeProducer) Name() string { return "native" }

func (p *NativeProducer) ProduceTags(action canon.CanonicalAction, raw []byte) []string {
    return RunAll(action, raw)
}
```

This is a thin wrapper. `RunAll` already iterates registered detectors.

### Step 6.3: Create SARIF/scanner producer (scaffold)

```go
// internal/detectors/sarif_producer.go
package detectors

import (
    "encoding/json"
    "samebits.com/evidra-benchmark/internal/canon"
)

// SARIFProducer extracts risk tags from SARIF scan results.
// Maps scanner rule IDs to Evidra canonical tags.
type SARIFProducer struct {
    // RuleMapping maps scanner ruleId to Evidra tag.
    // e.g., "KSV012" → "k8s.privileged_container"
    RuleMapping map[string]string
}

func (p *SARIFProducer) Name() string { return "sarif" }

func (p *SARIFProducer) ProduceTags(_ canon.CanonicalAction, raw []byte) []string {
    // Try to parse as SARIF
    var sarif struct {
        Runs []struct {
            Results []struct {
                RuleID string `json:"ruleId"`
                Level  string `json:"level"`
            } `json:"results"`
        } `json:"runs"`
    }
    if err := json.Unmarshal(raw, &sarif); err != nil {
        return nil // not SARIF, skip silently
    }

    seen := make(map[string]bool)
    var tags []string
    for _, run := range sarif.Runs {
        for _, result := range run.Results {
            if tag, ok := p.RuleMapping[result.RuleID]; ok && !seen[tag] {
                tags = append(tags, tag)
                seen[tag] = true
            }
        }
    }
    return tags
}
```

### Step 6.4: Producer registry

```go
// internal/detectors/producers.go
package detectors

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "sync"
)

var (
    prodMu    sync.RWMutex
    producers []TagProducer
)

func init() {
    // Native detectors are always registered
    RegisterProducer(&NativeProducer{})
}

// RegisterProducer adds a tag producer to the global chain.
func RegisterProducer(p TagProducer) {
    prodMu.Lock()
    defer prodMu.Unlock()
    producers = append(producers, p)
}

// ProduceAll runs all registered producers and returns merged, deduplicated tags.
func ProduceAll(action canon.CanonicalAction, raw []byte) []string {
    prodMu.RLock()
    defer prodMu.RUnlock()

    seen := make(map[string]bool)
    var tags []string
    for _, p := range producers {
        for _, tag := range p.ProduceTags(action, raw) {
            if !seen[tag] {
                tags = append(tags, tag)
                seen[tag] = true
            }
        }
    }
    return tags
}
```

### Step 6.5: Update caller

In `internal/lifecycle/service.go`:

```go
// OLD (after Task 1 refactor)
riskTags := detectors.RunAll(cr.CanonicalAction, input.RawArtifact)

// NEW
riskTags := detectors.ProduceAll(cr.CanonicalAction, input.RawArtifact)
```

`ProduceAll` calls `RunAll` internally via `NativeProducer`. Zero behavioral change if no external producers are registered.

### Step 6.6: Example Trivy mapping (shipped as config, not code)

```yaml
# config/scanner-mappings/trivy.yaml
scanner: trivy
version: "0.50"
mappings:
  KSV012: k8s.privileged_container
  KSV013: k8s.host_namespace_escape
  KSV023: k8s.hostpath_mount
  KSV029: k8s.run_as_root
  KSV001: k8s.dangerous_capabilities
  # ... extend as needed
```

Loaded at startup if present. Not required — native detectors work without it.

### When to do this

**After Task 3 (signal validation) passes.** TagProducer is additive — it doesn't change native detector behavior. But doing it before signal validation adds unnecessary variables.

```
Task 1 (refactor) → Task 2 (new detectors) → Task 3 (SIGNAL VALIDATION GATE)
→ Task 4 (prompts) → Task 5 (enhancements) → Task 6 (TagProducer)
```

