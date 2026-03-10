# Evidra Benchmark Dataset — Architecture and Pipeline Design

**Status:** Proposal (extends benchmark-dataset-proposal)
**Date:** March 2026

---

## 1. Three-Layer Dataset Architecture

The dataset splits into three layers with different collection methods, validation rules, and update frequencies.

### Layer A — Artifact Corpus

Raw infrastructure artifacts collected from open-source repositories with minimal transformation.

**Contents:**
- Kubernetes manifests (from Kubescape, Kyverno, Polaris, K8s docs)
- Terraform configurations (from Checkov, tfsec, terraform-provider-aws)
- SARIF reports (from Trivy, Kubescape, Checkov scanner output)
- Helm rendered templates (via `helm template` → YAML)

**Rules:**
- Minimal transformation — strip secrets, pin commit, preserve structure
- Every artifact has a source manifest in `tests/benchmark/sources/`
- Artifacts are not modified to "fit" detectors — they are ground truth
- License verified before import (Apache-2.0, MIT, BSD, MPL-2.0 only)

**Directory:**
```
tests/artifacts/fixtures/
  k8s/
    kubescape-C0057-privileged.yaml
    kyverno-disallow-host-path.yaml
    polaris-run-as-root.yaml
  terraform/
    checkov-s3-public.tf
    checkov-iam-wildcard.tf
    tfsec-rds-no-encryption.tf
  sarif/
    trivy-nginx-scan.sarif
    kubescape-nsa-profile.sarif
  helm/
    bitnami-nginx-privileged-rendered.yaml
```

**Why a separate corpus matters:** Cases come and go, detectors change, but the raw artifacts are stable. A corpus artifact can back multiple cases. When a new detector is added, you scan the entire corpus for matches — instant new cases without new collection.

### Layer B — Benchmark Cases

Each case wraps one corpus artifact (or a small set) with ground truth, narrative, and expected outcomes.

**Structure per case:**
```
tests/benchmark/cases/{case-id}/
  README.md              # Scenario narrative
  expected.json          # Ground truth, artifact ref, expected signals/risk_tags
  snapshots/
    contract.json        # Frozen contract fields only (not full prescribe output)
```

**Artifact references live in expected.json, not a separate file.** Cases point to corpus artifacts via the `artifact_ref` field in expected.json. One file, one source of truth, simpler validation:

```json
{
  "artifact_ref": "corpus/k8s/privileged/kubescape-C0057-deployment.yaml",
  ...
}
```

**One case = one risk pattern.** A privileged container is one case. A privileged container with hostPath is two cases (or one "compound" case explicitly tagged).

### Layer C — Scenarios

Multi-step sequences that produce behavioral signals. These cannot be tested with a single artifact — they require a sequence of prescribe/report calls with specific timing and outcomes.

**Structure per scenario:**
```
tests/benchmark/scenarios/{scenario-id}/
  README.md
  scenario.yaml           # Executable step sequence
  expectations.yaml       # Expected signals (separate schema from cases' expected.json)
  artifacts/              # Scenario-specific artifacts (step variants, modified copies)
    step-1-manifest.yaml
    step-2-manifest-modified.yaml
  snapshots/
    signals.json          # Frozen signal output
```

**Why `expectations.yaml` instead of `expected.json`:** Cases and scenarios test fundamentally different things. Cases test risk detectors (output: risk_level, risk_tags). Scenarios test behavioral signals (output: signal counts, event sequences). Sharing `expected.json` for both forces an overly complex schema. Separate files, separate validation logic.

```yaml
# expectations.yaml
scenario_id: retry-failed-deploy
signals_expected:            # PRIMARY validation — always required
  retry_loop:
    expected: true
    min_count: 3
  protocol_violation:
    expected: false
score_range:                 # OPTIONAL — only meaningful when ops >= MinOperations (100)
  min: 0.0
  max: 0.60
```

**Validation rule:** `signals_expected` is the primary assertion for scenarios. `score_range` is optional and only validated when total operations exceed MinOperations threshold. Most per-case scenarios have < 100 operations and should use signal counts only.

**Why separate from cases:** A case tests "does the detector see the risk in this artifact?" A scenario tests "does the signal fire when the agent behaves this way?" Different questions, different validation logic, different collection methods.

| Property | Layer B (Case) | Layer C (Scenario) |
|----------|---------------|-------------------|
| Input | Single artifact (ref to corpus) | Sequence of actions |
| Tests | Risk detectors, canonicalization | Signal detectors (5 signals) |
| Ground truth | `expected.json` | `expectations.yaml` |
| Validation | `evidra prescribe` → check risk_tags | `evidra scorecard` → check signal counts |
| MinOperations | N/A (single op) | May need special handling (< 100 ops) |
| Source | OSS fixtures (real-derived) | Mostly custom-built |
| Execution | Stateless | Requires state (evidence chain) |

### Critique: Is Three Layers Over-Engineering?

For 30-50 cases, the full structure might seem premature. But the deduplication strategy (section 4) requires a corpus from day one — the dataset collector skill needs somewhere to dump raw artifacts before curation. The pragmatic approach: corpus is a flat dump directory with minimal structure, cases are the curated product, scenarios come later.

**Recommendation:** Start with Layer B (cases) as the primary working directory. Create Layer A (corpus) from day one as the greedy collection target — the dataset collector skill dumps everything here. Promote from corpus to cases using the fingerprint deduplication process (section 4). Add Layer C (scenarios) when you have 5+ multi-step sequences. The expected.json schema supports all three layers from day one.

---

## 2. Case Taxonomy

### Current Fields (from benchmark proposal)

- `case_id` (string)
- `category` (kubernetes | terraform | helm | argocd | agent)
- `difficulty` (easy | medium | hard | catastrophic)
- `tags` (string array)

### New Field: case_kind

Classifies the structure of the case:

| case_kind | Description | Example |
|-----------|-------------|---------|
| `artifact` | Single artifact, tests risk detectors | k8s-privileged-container |
| `scenario` | Multi-step, tests behavioral signals | retry-failed-deploy |
| `scanner` | SARIF-driven, tests finding ingestion | trivy-critical-findings |
| `pair` | PASS + FAIL variant of same pattern | k8s-hostpath-mount-pass / -fail |

The `pair` kind is particularly valuable. Security tool fixtures naturally come as pairs: a manifest that violates a rule and one that doesn't. Both are useful — the FAIL case tests detector sensitivity, the PASS case tests specificity (no false positive).

### New Field: primary_signal

The single signal or risk tag this case exists to test. **Derived, not stored separately** — for artifact cases, `primary_signal` is `first(risk_details_expected)`. For scenario cases, it is the first key in `signals_expected`. This avoids a separate field that drifts out of sync.

The derivation rule:

```
if risk_details_expected is non-empty:
    primary_signal = risk_details_expected[0]
elif signals_expected is non-empty:
    primary_signal = first key in signals_expected
else:
    primary_signal = "risk_matrix"  (case tests risk level, not a specific detector)
```

This field drives the coverage dashboard. It answers: "if we remove this case, do we lose coverage of this signal?"

### New Field: ground_truth_pattern

**Tool-agnostic pattern identifier.** The benchmark should be usable by any tool, not just Evidra. `ground_truth_pattern` describes WHAT the risk IS, independent of how any tool detects it.

Uses dot-namespace convention (consistent with OTel, Evidra signals, and industry practice):

```json
{
  "ground_truth_pattern": "k8s.privileged_container"
}
```

For Evidra, the mapping is identity — `ground_truth_pattern` matches the Evidra signal name. Other tools map differently:

| ground_truth_pattern | Evidra signal | Checkov check | Kubescape control |
|---------------------|---------------|---------------|-------------------|
| `k8s.privileged_container` | `k8s.privileged_container` | `CKV_K8S_16` | `C-0057` |
| `k8s.hostpath_mount` | `k8s.hostpath_mount` | `CKV_K8S_29` | `C-0048` |
| `tf.s3_public_access` | `terraform.s3_public_access` | `CKV_AWS_19` | N/A |
| `tf.iam_wildcard` | `terraform.iam_wildcard_policy` | `CKV_AWS_1` | N/A |
| `ops.mass_delete` | `ops.mass_delete` | N/A | N/A |

The mapping lives outside the dataset — each tool maintains its own `ground_truth_pattern → tool_signal` mapping file. Evidra's mapping is trivial (mostly 1:1); other tools need a lookup table.

### Pattern Namespace Convention (normative)

```
k8s.*        Kubernetes workload and cluster security
tf.*         Terraform / IaC misconfigurations
helm.*       Helm chart and release risks
ops.*        Operational patterns (mass_delete, etc.)
scanner.*    Scanner-specific finding patterns
agent.*      AI agent behavioral patterns
```

Examples: `k8s.privileged_container`, `tf.s3_public_access`, `tf.iam_wildcard`, `ops.mass_delete`, `agent.retry_loop`.

Short prefixes (`k8s`, `tf`, not `kubernetes`, `terraform`) — consistent, compact, already used in Evidra signal names. New namespaces require a doc update to this list.

This decoupling makes the benchmark vendor-neutral. Any tool can be evaluated against the same ground truth. No existing infrastructure security benchmark provides this.

### Updated expected.json Schema

```json
{
  "case_id": "k8s-privileged-container-fail",
  "case_kind": "artifact",
  "category": "kubernetes",
  "difficulty": "medium",

  "ground_truth_pattern": "k8s.privileged_container",

  "ground_truth": {
    "infrastructure_risk": "critical",
    "blast_radius_resources": 1,
    "security_impact": "high",
    "attack_surface": "container-escape"
  },

  "artifact_ref": "corpus/k8s/privileged/kubescape-C0057-deployment.yaml",
  "artifact_digest": "sha256:a1b2c3...",

  "risk_details_expected": ["k8s.privileged_container"],
  "risk_level": "critical",
  "signals_expected": {},

  "tags": ["kubernetes", "security", "privileged-container"],

  "source_refs": [
    {
      "source_id": "kubescape-regolibrary",
      "composition": "real-derived"
    }
  ]
}
```

Note: `primary_signal` is derived as `first(risk_details_expected)` = `k8s.privileged_container`. Since ground_truth_pattern uses the same dot-namespace as Evidra signals, `ground_truth_pattern` and `risk_details_expected[0]` will often be identical for Evidra-detected patterns. This is intentional — it means for Evidra, the benchmark "just works" without a mapping layer.

### Critique: ground_truth_pattern vs risk_details_expected

For Evidra-detected patterns, `ground_truth_pattern` ≈ `risk_details_expected[0]`. But they serve different roles: `ground_truth_pattern` is the benchmark-level identifier (one per case, used by any tool), `risk_details_expected` is the Evidra-specific validation list (can have multiple tags for compound cases). They diverge when: (a) Evidra uses a different name than the ground truth pattern (e.g., `tf.s3_public_access` → `terraform.s3_public_access`), or (b) a case produces multiple risk tags but has one primary pattern.

---

## 3. PASS/FAIL Pair Convention

Security tool fixtures come in pairs. Standardize this.

### Convention

```
cases/
  k8s-hostpath-mount-fail/          # Artifact WITH the misconfiguration
    expected.json                    # artifact_ref → corpus/..., risk_details_expected: [...]
  k8s-hostpath-mount-pass/          # Artifact WITHOUT the misconfiguration
    expected.json                    # artifact_ref → corpus/..., risk_details_expected: []
```

### Why Pairs Matter

A detector that fires on everything has 100% recall and 0% precision. The FAIL case tests recall ("does it catch the bad thing?"). The PASS case tests precision ("does it stay quiet on the good thing?"). Without PASS cases, you cannot measure false positive rate.

From the experiment design document: `false_positive_rate` is a key benchmark metric. PASS cases are required to compute it.

### Schema Addition

Use `pair_id` + `pair_role` instead of bidirectional sibling references. Simpler — no two-way links to maintain:

```json
{
  "case_kind": "pair",
  "pair_id": "k8s-hostpath-mount",
  "pair_role": "fail"
}
```

```json
{
  "case_kind": "pair",
  "pair_id": "k8s-hostpath-mount",
  "pair_role": "pass"
}
```

Pairs are linked by shared `pair_id`. Validation: every `pair_id` must have exactly one `pass` and one `fail` case.

### Critique: Doubling the Case Count

If every misconfig gets a pair, 25 unique patterns = 50 cases. This hits the v1.0 target of 50 cases but with less diversity than 50 unique patterns. Trade-off: precision measurement (pairs) vs breadth (more unique patterns).

**Recommendation:** Pairs for the top 10 most important detectors. Single FAIL-only cases for the remaining patterns. This gives ~20 paired cases + ~15 single cases + ~15 scenarios = 50 total with good coverage and precision data for key detectors.

---

## 4. Corpus Collection & Deduplication Strategy

### The Problem

For any given risk pattern (e.g., privileged container), open-source security tools contain dozens of test fixtures. Kubescape has 3 variants, Kyverno has 4, Polaris has 2, Kubernetes docs have 1. Collecting all of them is cheap. But promoting all of them to benchmark cases creates bloat without value — 10 near-identical privileged container cases don't test anything that 2 wouldn't.

The opposite problem is worse: being too selective during collection and discarding an artifact that turns out to be the only one exercising a specific code path.

### Rule: Greedy Corpus, Strict Cases

**Layer A (Corpus) — collect everything.** When the dataset collector agent finds an artifact matching a known risk pattern, it goes into `corpus/` regardless of whether a similar artifact already exists. Storage is cheap, re-collection is not.

**Layer B (Cases) — promote selectively.** From the corpus, promote only artifacts that are **distinct from the benchmark's perspective** — meaning they exercise different code paths in Evidra's detectors.

### Differentiation Axes

Two artifacts with the same `primary_signal` are distinct cases if they differ on ANY of these axes:

#### Axis 1 — Resource Type

A privileged container in a Pod, a Deployment, and a DaemonSet takes three different canonicalization paths through the `k8s/v1` adapter. Pod has no `spec.template`. Deployment does. DaemonSet runs on every node (different `resource_count` semantics). If `evidra prescribe` produces different output → distinct cases.

| Artifact | resource_count | resource_shape_hash | Distinct? |
|----------|---------------|-------------------|-----------|
| privileged-pod.yaml | 1 | sha256:aaa... | Base case |
| privileged-deployment.yaml | 1 | sha256:bbb... | Yes (different shape) |
| privileged-daemonset.yaml | 1 | sha256:ccc... | Yes (different shape) |
| privileged-deployment-v2.yaml | 1 | sha256:bbb... | No (same shape) |

#### Axis 2 — Scope Class (Environment)

Same manifest evaluated in different environments hits different cells in the risk matrix:

| Environment | operation_class | scope_class | risk_level |
|-------------|----------------|-------------|-----------|
| production | mutate | production | high → elevated to critical (with risk tag) |
| staging | mutate | staging | medium → elevated to high |
| development | mutate | development | low → elevated to medium |

One case per scope class that produces a different `risk_level`. Typically 2 cases: production + development (staging is often redundant with production for risk elevation testing).

#### Axis 3 — Compound Risk Tags

| Artifact | risk_tags | Distinct from single-tag case? |
|----------|-----------|-------------------------------|
| privileged-only.yaml | `[k8s.privileged_container]` | Base case |
| privileged-hostpath.yaml | `[k8s.privileged_container, k8s.hostpath_mount]` | Yes |
| privileged-hostpath-hostnet.yaml | `[k8s.privileged_container, k8s.hostpath_mount, k8s.host_namespace_escape]` | Yes |

Compound cases test that detectors work independently and don't interfere. One compound case per unique tag combination (not per permutation).

#### Axis 4 — Resource Count / Blast Radius

| Artifact | resource_count | Triggers blast_radius? | Distinct? |
|----------|---------------|----------------------|-----------|
| single-pod-delete.yaml | 1 | No | Base case |
| multi-doc-3-pods-delete.yaml | 3 | No (below threshold 5) | No (same outcome) |
| multi-doc-15-pods-delete.yaml | 15 | Yes (above threshold) | Yes |

Only promote a variant if it crosses a detector threshold.

#### Axis 5 — Parser Edge Cases

| Artifact | Parser behavior | Distinct? |
|----------|----------------|-----------|
| standard.yaml | Normal parse | Base case |
| multi-doc-with-empty.yaml | Empty documents between `---` separators | Yes (tests splitYAMLDocuments) |
| helm-rendered-with-hooks.yaml | Helm hook annotations, test pods | Yes (tests K8s adapter filtering) |
| comments-and-anchors.yaml | YAML anchors, aliases, comments | Yes (tests YAML parser robustness) |

One edge case per unique parser behavior. These are not risk cases — they are canonicalization robustness cases.

### Prescribe Fingerprint — Automated Deduplication

Two artifacts are duplicates if `evidra prescribe` produces identical output on the fields that matter.

The fingerprint includes `ground_truth_pattern` and `resource_shape_hash` to prevent false deduplication. Without these, two different security patterns (e.g., hostpath_mount and privileged_container) could produce the same fingerprint when they happen to share risk_level, operation_class, and resource_count.

```
fingerprint = ground_truth_pattern | risk_level | sorted(risk_tags) | resource_shape_hash | operation_class | scope_class | canon_version | adapter_version
```

Note: `canon_version` is the adapter family (`k8s/v1`), `adapter_version` is the Evidra binary version that ran the adapter. Both matter: same adapter family can produce different output across Evidra releases.

Automated check:

```bash
#!/usr/bin/env bash
# scripts/detect-duplicates.sh

declare -A fingerprints

for case_dir in tests/benchmark/cases/*/; do
  # Resolve artifact from expected.json
  artifact_ref=$(jq -r '.artifact_ref // empty' "$case_dir/expected.json" 2>/dev/null)
  if [ -n "$artifact_ref" ]; then
    artifact="tests/benchmark/$artifact_ref"
  else
    artifact=$(find "$case_dir/artifacts" -type f 2>/dev/null | head -1)
  fi
  [ -z "$artifact" ] || [ ! -f "$artifact" ] && continue

  case_id=$(basename "$case_dir")

  category=$(jq -r '.category' "$case_dir/expected.json" 2>/dev/null || echo "unknown")
  gtp=$(jq -r '.ground_truth_pattern // "unknown"' "$case_dir/expected.json" 2>/dev/null)
  tool="kubectl"
  [[ "$category" == "terraform" ]] && tool="terraform"

  prescribe_out=$(evidra prescribe --tool "$tool" --operation apply \
    --artifact "$artifact" --signing-mode optional 2>/dev/null)

  fp=$(echo "$prescribe_out" \
    | jq -r --arg gtp "$gtp" \
      '[$gtp, .risk_level, (.risk_tags // [] | sort | join(",")), .resource_shape_hash, .operation_class, .scope_class, .canon_version, .adapter_version] | join("|")')

  if [ -z "$fp" ] || [ "$fp" = "||||||" ]; then
    echo "SKIP: $case_id (prescribe failed)"
    continue
  fi

  if [ -n "${fingerprints[$fp]:-}" ]; then
    echo "DUPLICATE: $case_id ≈ ${fingerprints[$fp]}"
    echo "  fingerprint: $fp"
  else
    fingerprints[$fp]="$case_id"
  fi
done

echo ""
echo "Unique fingerprints: ${#fingerprints[@]}"
```

Output is a **warning**, not a failure. Intentional duplicates exist (PASS/FAIL pairs where both have empty risk_tags, or cases testing different environments with the same artifact).

### Promotion Decision: When Duplicates Are Found

When multiple corpus artifacts share the same fingerprint, promote the one that:

1. **Best provenance** — OSS fixture from a well-known project beats custom-written
2. **Simplest** — fewer lines, less noise, easier to understand in the case README
3. **Most realistic** — resembles a real production manifest, not a minimal test stub
4. **Best documented upstream** — artifact with an associated CVE, rule description, or policy explanation

Keep the others in `corpus/` — they may become distinct when a new detector is added (e.g., a future `k8s.capabilities_escalation` detector might split currently-identical fingerprints).

### Fixture Directory Structure

```
tests/artifacts/fixtures/
  k8s/
    privileged/
      kubescape-C0057-pod.yaml
      kubescape-C0057-deployment.yaml
      kyverno-disallow-privileged.yaml
      polaris-privileged-check.yaml       ← 4 variants, only 2 promoted to cases
    hostpath/
      kubescape-C0048-pod.yaml
      kyverno-disallow-host-path.yaml     ← 2 variants, 1 promoted
    ...
  terraform/
    iam/
      checkov-wildcard-policy.tf
      checkov-wildcard-resource.tf
      tfsec-iam-no-policy-wildcards.tf    ← 3 variants, 2 promoted (different risk_tags)
    ...
```

Each promoted case in `cases/` has an artifact that is either copied from or symlinked to the corpus file. The corpus stores the complete collection; cases store the curated selection.

### Dataset Collector Skill Addition

Add to the collector skill instructions:

```
DEDUPLICATION WORKFLOW:

When you find multiple artifacts for the same risk pattern:

1. Collect ALL into corpus/{category}/{pattern}/ — never discard at this stage
2. Run `evidra prescribe` on each artifact
3. Compute fingerprint: risk_level + sorted(risk_tags) + operation_class + resource_count
4. Group artifacts by fingerprint
5. From each fingerprint group, promote ONE artifact to cases/:
   - Prefer: well-known OSS source > custom
   - Prefer: shortest/cleanest YAML > complex
   - Prefer: realistic production-like > minimal test stub
6. If artifacts in the SAME fingerprint group have DIFFERENT resource types
   (Pod vs Deployment vs DaemonSet), promote one per resource type —
   they are distinct cases despite same fingerprint
7. Log all decisions: in the case README, note which corpus artifacts
   were considered and why this one was chosen
```

### Corpus Immutability Rule (normative)

**Corpus artifacts MUST NOT be modified in place.** Once a file is committed to `corpus/`, it is frozen. If an artifact needs fixing (strip a secret, fix encoding), create a new file with a version suffix:

```
corpus/k8s/privileged_container/
  kubescape-C0057-deployment.yaml        ← original, frozen
  kubescape-C0057-deployment-v2.yaml     ← fixed version
```

Update the case's `artifact_ref` to point to the new file. The old file stays — it may be referenced by contract snapshots, and removing it breaks reproducibility.

This is critical because `artifact_digest` in expected.json is a SHA-256 hash of the corpus file. If the file changes, the hash breaks, and `validate-dataset.sh` fails. Immutability prevents this class of errors entirely.

### Critique: Corpus Maintenance Cost

A `corpus/` directory with 200+ raw artifacts is easy to create and hard to maintain. Files rot: upstream repos move, commits get rebased, licenses change. Without periodic re-validation, the corpus becomes a liability.

**Mitigation:** Corpus files are pinned to commit SHAs, not branches. They don't change unless explicitly re-imported. A quarterly `scripts/verify-corpus-sources.sh` checks that upstream URLs and commits still exist. Artifacts that become unreachable get flagged, not deleted — the file is still valid even if the upstream moved.

**Pragmatic note:** At the 30-50 case scale, the corpus will have maybe 100-150 artifacts. This is manageable. The corpus strategy matters more when scaling to 200+ cases where manual tracking of "which variant did I already promote?" becomes impossible.

---

## 5. Golden Outputs for Stability

### The Problem

Detectors change. A new release might modify how `k8s.privileged_container` is detected, changing the prescribe output for 15 cases. Without contract snapshots, CI passes silently even when behavior changes. With contract snapshots, every change produces a visible diff.

### Implementation

For each case, store **only the contract fields** from the detector output — not the full prescribe JSON. This dramatically reduces contract snapshot churn. A refactor that changes internal field names or adds new informational fields does not break snapshots; only actual contract changes do.

```
cases/{case-id}/
  snapshots/
    contract.json          # Contract fields only (not full prescribe output)
```

**contract.json example:**
```json
{
  "ground_truth_pattern": "k8s.privileged_container",
  "risk_level": "critical",
  "risk_tags": ["k8s.privileged_container"],
  "operation_class": "mutate",
  "scope_class": "unknown",
  "resource_count": 1,
  "canon_version": "k8s/v1"
}
```

Including `ground_truth_pattern` in the snapshot makes debugging immediate: you see expected pattern vs detected tags side by side.

For scenarios, a separate snapshot:
```
scenarios/{scenario-id}/
  snapshots/
    signals.json           # Signal names and counts only
```

### CI Validation

```bash
# Resolve artifact from expected.json
artifact_ref=$(jq -r '.artifact_ref' "cases/$CASE/expected.json")
artifact="tests/benchmark/$artifact_ref"

actual=$(evidra prescribe --tool kubectl --operation apply \
  --artifact "$artifact" \
  --evidence-dir /tmp/snapshot-test \
  --signing-mode optional 2>/dev/null)

# Compare contract fields only
for field in risk_level operation_class scope_class resource_count canon_version; do
  expected=$(jq -r ".$field" "cases/$CASE/snapshots/contract.json")
  got=$(echo "$actual" | jq -r ".$field")
  if [ "$expected" != "$got" ]; then
    echo "GOLDEN MISMATCH: $CASE.$field expected=$expected actual=$got"
    exit 1
  fi
done

# Risk tags: set equality (order-independent)
expected_tags=$(jq -r '.risk_tags | sort | join(",")' "cases/$CASE/snapshots/contract.json")
actual_tags=$(echo "$actual" | jq -r '(.risk_tags // []) | sort | join(",")')
if [ "$expected_tags" != "$actual_tags" ]; then
  echo "GOLDEN MISMATCH: $CASE.risk_tags expected=$expected_tags actual=$actual_tags"
  exit 1
fi
```

### Updating Golden Files

When a detector intentionally changes:

```bash
# Regenerate all contract snapshots
make snapshot-update-benchmark

# Review the diff
git diff tests/benchmark/cases/*/snapshots/

# Commit with explanation
git commit -m "update snapshot: new hostPath detection logic in k8s adapter"
```

This makes detector changes visible at PR time. Reviewer sees: "this PR changes the risk detector and updates 7 contract snapshots" — clear, auditable.

### Contract Fields (stored in snapshot)

| Field | Comparison | Rationale |
|-------|-----------|-----------|
| `risk_level` | Strict equality | Core contract |
| `risk_tags` | Set equality (order-independent) | Tags may be detected in different order |
| `operation_class` | Strict equality | Core contract |
| `scope_class` | Strict equality | Core contract |
| `resource_count` | Exact match | Deterministic from artifact |
| `canon_version` | Exact match | Adapter version tracking |

### Non-Contract Fields (NOT stored in snapshot)

| Field | Why excluded |
|-------|-------------|
| `prescription_id` | Generated, different each run |
| `artifact_digest` | Validated separately via artifact_ref hash check |
| `intent_digest` | Derived from canonical action — changes if any contract field changes |
| `resource_shape_hash` | Internal optimization detail, not user-facing contract |
| `session_id`, `trace_id` | Runtime identifiers |

Storing only 6 contract fields means a detector refactor that changes internal JSON structure but preserves the same risk_level/risk_tags/operation_class output does NOT break any contract snapshot. This is the point.

### Critique: Golden Files Are Maintenance Burden

Every detector change touches contract snapshots. If you have 50 cases, a single detector fix could require updating 20 contract snapshots. This is tedious.

**Mitigation:** `make benchmark-refresh-contracts` regenerates all contract snapshots. The human cost is reviewing the diff, not manually editing files. This is a feature, not a bug — you want to see exactly what changed.

**Risk:** If snapshot updates become rubber-stamped ("just regenerate and commit"), they lose their value. Counter: CI should also run `validate-source-composition.sh` to ensure the updates are consistent.

---

## 6. Dataset Versioning & Evidra Version Pinning

### The Problem

The dataset is a product of Evidra's detectors at a specific point in time. When Evidra v0.6.0 adds a new detector (`k8s.capabilities_escalation`), the same artifact that produced `risk_tags: ["k8s.privileged_container"]` in v0.5.0 might now produce `risk_tags: ["k8s.privileged_container", "k8s.capabilities_escalation"]`. Without version tracking, old cases break silently or new contributions are inconsistent with existing ones.

Three things need versions: the dataset itself, the Evidra version that processed each case, and the processing pipeline for incoming contributions.

### Dataset Version

The dataset has its own semver, independent of Evidra's version:

```
dataset v1.0 — 50 cases, processed with Evidra v0.5.0
dataset v1.1 — 55 cases, 3 cases updated for new detector, processed with Evidra v0.5.2
dataset v2.0 — 80 cases, new signal added, re-processed with Evidra v0.6.0
```

Rules:

| Change | Version bump | Example |
|--------|-------------|---------|
| New cases added, no schema change | **patch** (v1.0 → v1.1) | Add 5 terraform cases |
| Cases updated for new/changed detector | **minor** (v1.1 → v1.2) | New detector changes risk_tags on 3 cases |
| Schema change in expected.json | **major** (v1.x → v2.0) | Add new required field, change ground_truth_pattern namespace |
| Full re-processing with new Evidra major | **major** (v1.x → v2.0) | Re-run all cases through Evidra v0.6.0 |

Stored in `tests/benchmark/dataset.json`:

```json
{
  "dataset_version": "1.0.0",
  "evidra_version_min": "0.5.0",
  "evidra_version_processed": "0.5.0",
  "spec_version": "v1.0.0",
  "created_at": "2026-03-20",
  "updated_at": "2026-03-25",
  "case_count": 50,
  "corpus_count": 142,
  "scenario_count": 12
}
```

### Per-Case Evidra Version

Every `expected.json` records which Evidra version produced its snapshot output and risk expectations:

```json
{
  "case_id": "k8s-privileged-container-fail",
  "processing": {
    "evidra_version": "0.5.0",
    "canon_version": "k8s/v1",
    "scoring_version": "v1.0.0",
    "processed_at": "2026-03-20T14:00:00Z"
  },
  ...
}
```

And in snapshot `contract.json`:

```json
{
  "ground_truth_pattern": "k8s.privileged_container",
  "risk_level": "critical",
  "risk_tags": ["k8s.privileged_container"],
  "operation_class": "mutate",
  "scope_class": "unknown",
  "resource_count": 1,
  "canon_version": "k8s/v1",
  "evidra_version": "0.5.0"
}
```

This answers: "which version of Evidra produced this snapshot?" If a case fails CI after an Evidra upgrade, you see immediately: snapshot was created with v0.5.0, running v0.6.0 now, expected difference.

### Contribution Processing Pipeline

When someone contributes a raw artifact (or the dataset collector skill finds one), it must be processed with a **pinned** Evidra version — the same version as the rest of the dataset.

```
Contributor sends: raw K8s manifest (privileged-pod.yaml)
                         │
                         ▼
Processing pipeline (pinned to dataset's evidra_version):
  1. evidra prescribe --tool kubectl --operation apply --artifact <file>
  2. Extract contract fields → snapshots/contract.json
  3. Compare with existing corpus → dedup fingerprint
  4. If unique → create case with expected.json
  5. Tag: processing.evidra_version = "0.5.0"
                         │
                         ▼
Output: new case, consistent with all other v1.x cases
```

The pipeline uses a **pinned Evidra binary**, not whatever the contributor has installed. This ensures determinism.

Implementation:

```bash
#!/usr/bin/env bash
# scripts/process-artifact.sh
# Uses the pinned Evidra version from dataset.json

DATASET_EVIDRA_VERSION=$(jq -r '.evidra_version_processed' tests/benchmark/dataset.json)
EVIDRA_BIN="bin/evidra-${DATASET_EVIDRA_VERSION}"

# Download pinned version if not cached
if [ ! -f "$EVIDRA_BIN" ]; then
  echo "Downloading evidra $DATASET_EVIDRA_VERSION..."
  curl -sL "https://github.com/vitas/evidra/releases/download/v${DATASET_EVIDRA_VERSION}/evidra_${DATASET_EVIDRA_VERSION}_$(uname -s)_$(uname -m).tar.gz" \
    | tar -xz -C bin/
  mv bin/evidra "$EVIDRA_BIN"
fi

ARTIFACT="$1"
TOOL="${2:-kubectl}"

# Process with pinned version
"$EVIDRA_BIN" prescribe \
  --tool "$TOOL" \
  --operation apply \
  --artifact "$ARTIFACT" \
  --signing-mode optional
```

### Dataset Upgrade (re-processing)

When a new Evidra version adds or changes a detector, the entire dataset must be re-processed to stay consistent. This is a deliberate, versioned operation:

```bash
#!/usr/bin/env bash
# scripts/upgrade-dataset.sh
# Re-processes all cases with a new Evidra version
# Usage: upgrade-dataset.sh <version> [--dry-run]

NEW_VERSION="$1"  # e.g., "0.6.0"
DRY_RUN=false
[[ "$2" == "--dry-run" ]] && DRY_RUN=true
EVIDRA_BIN="bin/evidra-${NEW_VERSION}"

echo "=== Upgrading dataset to Evidra $NEW_VERSION ==="
$DRY_RUN && echo "=== DRY RUN — no files will be modified ==="

for case_dir in tests/benchmark/cases/*/; do
  expected="$case_dir/expected.json"
  artifact_ref=$(jq -r '.artifact_ref // empty' "$expected")
  [ -z "$artifact_ref" ] && continue

  artifact="tests/benchmark/$artifact_ref"
  [ ! -f "$artifact" ] && echo "SKIP: $case_dir (missing artifact)" && continue

  category=$(jq -r '.category' "$expected")
  tool="kubectl"
  [[ "$category" == "terraform" ]] && tool="terraform"

  # Re-run prescribe with new version
  output=$("$EVIDRA_BIN" prescribe \
    --tool "$tool" --operation apply \
    --artifact "$artifact" --signing-mode optional 2>/dev/null)

  # Update snapshot contract
  if ! $DRY_RUN; then
    echo "$output" | jq '{
      ground_truth_pattern: .ground_truth_pattern,
      risk_level: .risk_level,
      risk_tags: (.risk_tags // []),
      operation_class: .operation_class,
      scope_class: .scope_class,
      resource_count: .resource_count,
      canon_version: .canon_version,
      evidra_version: "'"$NEW_VERSION"'"
    }' > "$case_dir/snapshots/contract.json"

    # Update processing metadata in expected.json
    tmp=$(mktemp)
    jq --arg v "$NEW_VERSION" --arg t "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      '.processing.evidra_version = $v | .processing.processed_at = $t' \
      "$expected" > "$tmp" && mv "$tmp" "$expected"
  fi

  # Check if risk_tags changed (always, even in dry-run)
  old_tags=$(jq -r '.risk_details_expected | sort | join(",")' "$expected")
  new_tags=$(echo "$output" | jq -r '(.risk_tags // []) | sort | join(",")')
  if [ "$old_tags" != "$new_tags" ]; then
    echo "CHANGED: $(basename "$case_dir") — risk_tags: $old_tags → $new_tags"
  fi
done

# Update dataset.json
if ! $DRY_RUN; then
  tmp=$(mktemp)
  jq --arg v "$NEW_VERSION" --arg t "$(date -u +%Y-%m-%d)" \
    '.evidra_version_processed = $v | .updated_at = $t' \
    tests/benchmark/dataset.json > "$tmp" && mv "$tmp" tests/benchmark/dataset.json
fi

$DRY_RUN && echo "=== Dry run complete. No files modified. ===" \
         || echo "=== Done. Review changes: git diff tests/benchmark/ ==="
```

Workflow:

1. Dry-run first: `scripts/upgrade-dataset.sh 0.6.0 --dry-run` — see what changes without writing
2. Run `scripts/upgrade-dataset.sh 0.6.0`
2. Review `git diff` — see which cases changed and why
3. Update `risk_details_expected` in expected.json for cases where new detectors fire
4. Bump dataset version (minor or major depending on scope)
5. Commit: `"dataset v1.2: re-processed with Evidra v0.6.0, 3 cases gained k8s.capabilities_escalation"`

### CI Validation: Version Consistency

Add to `validate-dataset.sh`:

```bash
# 11. Version consistency: all cases processed with same Evidra version
dataset_version=$(jq -r '.evidra_version_processed' tests/benchmark/dataset.json)
for f in cases/*/expected.json; do
  case_version=$(jq -r '.processing.evidra_version // empty' "$f")
  if [ -n "$case_version" ] && [ "$case_version" != "$dataset_version" ]; then
    case_id=$(jq -r '.case_id' "$f")
    echo "WARN: $case_id processed with Evidra $case_version, dataset expects $dataset_version"
    CHECKS_WARNED=$((CHECKS_WARNED + 1))
  fi
done
```

Warning, not failure — during an upgrade, cases will temporarily have mixed versions until `upgrade-dataset.sh` completes.

### Critique: Is This Over-Engineering for 50 Cases?

At 50 cases, `upgrade-dataset.sh` takes 10 seconds and the diff fits on one screen. The machinery feels heavy. But version pinning prevents a real problem: a contributor runs `evidra prescribe` with their local Evidra v0.7.0-dev while the dataset is on v0.5.0. Their snapshots/contract.json will have different risk_tags than everyone else's. CI catches this via the version consistency check.

The pinned binary download is the key practical piece. Everything else (dataset.json, processing metadata) is cheap bookkeeping that pays off when the project has 10+ contributors or 200+ cases.

---

## 7. SARIF as Separate Case Class

SARIF findings are not the same as artifact risk detection. They come from external scanners and go through a lossy projection into Evidra's `FindingPayload`.

### What Gets Lost in Projection

| SARIF Field | Evidra FindingPayload | Status |
|-------------|----------------------|--------|
| `ruleId` | `rule_id` | Preserved |
| `level` | `severity` (mapped) | Transformed (see mapping table below) |

### Normative Severity Mapping

This mapping is normative for all SARIF ingestion cases in the benchmark. It is implemented in `internal/sarif/parser.go` and must be consistent across all SARIF cases. Contributors MUST NOT use alternative mappings.

| SARIF `level` | Evidra `severity` | Notes |
|---------------|-------------------|-------|
| `critical` | `critical` | Direct mapping |
| `error` | `high` | SARIF "error" = Evidra "high", not "critical" |
| `warning` | `medium` | Standard mapping |
| `note` | `low` | Standard mapping |
| (anything else) | `info` | Fallback for unknown levels |

Different scanners use different level conventions. Trivy uses `error` for high-severity CVEs. Kubescape uses `warning` for most control failures. This mapping normalizes them into a consistent Evidra severity scale.
| `message.text` | `message` | Preserved |
| `locations[0].physicalLocation.artifactLocation.uri` | `resource` | First location only |
| `codeFlows` | — | Dropped |
| `threadFlows` | — | Dropped |
| `relatedLocations` | — | Dropped |
| `fixes` | — | Dropped |
| `properties` (tags, precision, etc.) | — | Dropped |

### SARIF Case Structure

```
cases/{case-id}/
  README.md
  expected.json
  artifacts/
    manifest.yaml              # The artifact that was scanned
    scan-results.sarif         # Original SARIF output
  snapshots/
    findings_projection.json   # What evidra ingest-findings actually extracts
```

### expected.json for Scanner Cases

```json
{
  "case_id": "trivy-critical-cve",
  "case_kind": "scanner",
  "category": "scanner",
  "primary_signal": "finding_ingestion",

  "findings_expected": {
    "total_count": 12,
    "by_severity": { "critical": 2, "high": 5, "medium": 3, "low": 2 },
    "tools": ["trivy"]
  },

  "projection_notes": "codeFlows and relatedLocations dropped; 3 findings had no ruleId and are mapped to 'unknown'"
}
```

### Critique: Is Scanner a Separate Category or a Property?

The benchmark proposal puts scanner cases in "Phase 2: Scanner SARIF integration." But SARIF findings can accompany any case — a K8s manifest might have both risk detector results AND Trivy findings. Making `scanner` a separate category forces an artificial split.

**Recommendation:** Keep `case_kind: scanner` for cases where SARIF ingestion is the primary thing being tested. For cases where SARIF is supplementary (e.g., "deploy manifest with known CVEs"), use `case_kind: hybrid` and include SARIF in `artifacts/` alongside the manifest.

---

## 8. Source → Case Pipeline

### Goal

A contributor adds a new case in 10-20 minutes without reading the entire codebase.

### Steps (Normative)

```
Step 1: Register source
  → Create/update tests/benchmark/sources/{source-id}.md
  → Include: repo URL, commit SHA, license, path, retrieval date

Step 2: Extract artifact
  → Copy minimal artifact to corpus/{category}/{pattern}/
  → Set artifact_ref in expected.json pointing to corpus path
  → Strip secrets, real account IDs, internal domain names

Step 3: Write README.md
  → Story, impact, risk, real-world parallel
  → 4-6 sentences total, not a research paper

Step 4: Create expected.json
  → Fill case_id, case_kind, category, difficulty, primary_signal
  → Set ground_truth, risk_details_expected, signals_expected
  → Add source_refs

Step 5: Run detector
  → evidra prescribe --artifact ... --signing-mode optional
  → Compare output to expected.json
  → Fix expected.json if prediction was wrong
  → Or flag detector bug if output is clearly incorrect

Step 6: Create snapshot output
  → Extract contract fields from prescribe output into snapshots/contract.json
  → Only: risk_level, risk_tags, operation_class, scope_class, resource_count, canon_version

Step 7: Validate
  → bash tests/benchmark/scripts/validate-dataset.sh
  → All checks pass → open PR
```

### CLI Helper (Future)

```bash
evidra bench add \
  --case-id k8s-hostpath-mount-fail \
  --category kubernetes \
  --difficulty medium \
  --source-id kubescape-regolibrary \
  --artifact corpus/k8s/hostpath/kyverno-hostpath-fail.yaml

# Creates:
#   cases/k8s-hostpath-mount-fail/
#     README.md          (template, needs editing)
#     expected.json      (template with case_id + artifact_ref filled)
#     snapshots/            (empty, run evidra prescribe to populate contract.json)
#   sources/kubescape-regolibrary.md (created if missing, template)
```

This reduces the mechanical work to: run one command, edit README and expected.json, run prescribe, commit. Five minutes for simple cases.

### Critique: Will Contributors Actually Show Up?

Probably not many, at this stage. The pipeline is designed for the solo dev (you) to work fast, not for community contribution. Community comes after the dataset has 50+ cases and proves its value. Design the pipeline for solo productivity first, community scalability second.

---

## 9. Dataset Validation Script

Expand `validate-source-composition.sh` into a comprehensive `validate-dataset.sh`:

```bash
#!/usr/bin/env bash
# tests/benchmark/scripts/validate-dataset.sh

CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNED=0

# 1. Schema validation: every expected.json has required fields
for f in cases/*/expected.json; do
  case_id=$(jq -r '.case_id' "$f")
  
  for field in case_id case_kind category difficulty ground_truth_pattern ground_truth source_refs; do
    if ! jq -e ".$field" "$f" >/dev/null 2>&1; then
      echo "FAIL: $f missing required field: $field"
      CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
  done
  
  # Critical string fields must be non-empty (not just present)
  for sfield in ground_truth_pattern category difficulty; do
    val=$(jq -r ".$sfield // empty" "$f")
    if [ -z "$val" ] || [ "$val" = "TODO" ]; then
      echo "FAIL: $f field $sfield is empty or TODO"
      CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
  done
  
  # case_id matches directory name
  dir_name=$(basename "$(dirname "$f")")
  if [ "$case_id" != "$dir_name" ]; then
    echo "FAIL: $f case_id '$case_id' != directory '$dir_name'"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
  
  CHECKS_PASSED=$((CHECKS_PASSED + 1))
done

# 2. Source refs resolve to existing manifests
# (existing logic from validate-source-composition.sh)

# 3. No secrets in corpus or case artifacts
for f in corpus/**/* cases/*/artifacts/*; do
  [ -f "$f" ] || continue
  if grep -qiE '(AKIA[0-9A-Z]{16}|password\s*[:=]\s*["\x27][^"\x27]{8,}|BEGIN (RSA |EC )?PRIVATE KEY)' "$f" 2>/dev/null; then
    echo "FAIL: potential secret in $f"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
done

# 4. README exists for every case
for d in cases/*/; do
  if [ ! -f "$d/README.md" ]; then
    echo "FAIL: missing README.md in $d"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
done

# 5. Artifact ref resolves and hash matches
for f in cases/*/expected.json; do
  case_dir=$(dirname "$f")
  case_id=$(basename "$case_dir")
  artifact_path=$(jq -r '.artifact_ref // empty' "$f")
  
  [ -z "$artifact_path" ] && continue

  if [ ! -f "tests/benchmark/$artifact_path" ]; then
    echo "FAIL: $case_id artifact_ref points to missing file: $artifact_path"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
    continue
  fi
  
  # Check artifact_digest (required for reproducibility)
  expected_digest=$(jq -r '.artifact_digest // empty' "$f")
  if [ -z "$expected_digest" ]; then
    echo "FAIL: $case_id missing required artifact_digest"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
    continue
  fi
  actual_digest="sha256:$(shasum -a 256 "tests/benchmark/$artifact_path" | awk '{print $1}')"
  if [ "$expected_digest" != "$actual_digest" ]; then
    echo "FAIL: $case_id artifact_digest mismatch (corpus file changed?)"
    echo "  expected: $expected_digest"
    echo "  actual:   $actual_digest"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
done

# 6. Golden contract files match (if present)
for snapshot in cases/*/snapshots/contract.json; do
  [ -f "$snapshot" ] || continue
  # ... snapshot validation logic from section 5
done

# 7. Case IDs are unique
dupes=$(find cases -name expected.json -exec jq -r '.case_id' {} \; | sort | uniq -d)
if [ -n "$dupes" ]; then
  echo "FAIL: duplicate case_ids: $dupes"
  CHECKS_FAILED=$((CHECKS_FAILED + 1))
fi

# 8. Pair validation: every pair_id has exactly one pass and one fail
declare -A pair_roles
for f in cases/*/expected.json; do
  pair_id=$(jq -r '.pair_id // empty' "$f")
  [ -z "$pair_id" ] && continue
  pair_role=$(jq -r '.pair_role' "$f")
  pair_roles["${pair_id}:${pair_role}"]=1
done
for key in "${!pair_roles[@]}"; do
  pair_id="${key%%:*}"
  role="${key##*:}"
  other=$( [ "$role" = "pass" ] && echo "fail" || echo "pass" )
  if [ -z "${pair_roles["${pair_id}:${other}"]:-}" ]; then
    echo "WARN: pair '$pair_id' has role '$role' but missing '$other'"
    CHECKS_WARNED=$((CHECKS_WARNED + 1))
  fi
done

# 9. Corpus usage: warn about unreferenced corpus files
if [ -d corpus ]; then
  referenced_files=$(find cases -name expected.json -exec jq -r '.artifact_ref // empty' {} \; | sort -u)
  for corpus_file in $(find corpus -type f | sort); do
    rel_path="${corpus_file#tests/benchmark/}"
    if ! echo "$referenced_files" | grep -qF "$rel_path"; then
      echo "WARN: unreferenced corpus file: $corpus_file"
      CHECKS_WARNED=$((CHECKS_WARNED + 1))
    fi
  done
fi

# 10. Orphan cases: every case with artifact_ref must point to existing corpus file
for f in cases/*/expected.json; do
  ref=$(jq -r '.artifact_ref // empty' "$f")
  [ -z "$ref" ] && continue
  if [ ! -f "tests/benchmark/$ref" ]; then
    case_id=$(jq -r '.case_id' "$f")
    echo "FAIL: orphan case $case_id — artifact_ref '$ref' does not exist"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
done

# 11. Duplicate artifact_ref: warn if two cases point to the same corpus file
declare -A artifact_refs
for f in cases/*/expected.json; do
  ref=$(jq -r '.artifact_ref // empty' "$f")
  [ -z "$ref" ] && continue
  case_id=$(jq -r '.case_id' "$f")
  if [ -n "${artifact_refs[$ref]:-}" ]; then
    echo "WARN: duplicate artifact_ref '$ref' used by $case_id and ${artifact_refs[$ref]}"
    CHECKS_WARNED=$((CHECKS_WARNED + 1))
  else
    artifact_refs[$ref]="$case_id"
  fi
done

# 12. Risk details consistency: snapshot risk_tags must match risk_details_expected
for case_dir in cases/*/; do
  snapshot="$case_dir/snapshots/contract.json"
  expected="$case_dir/expected.json"
  [ -f "$snapshot" ] && [ -f "$expected" ] || continue

  expected_tags=$(jq -r '(.risk_details_expected // []) | sort | join(",")' "$expected")
  snapshot_tags=$(jq -r '(.risk_tags // []) | sort | join(",")' "$snapshot")

  if [ "$expected_tags" != "$snapshot_tags" ]; then
    case_id=$(basename "$case_dir")
    echo "FAIL: $case_id risk_details_expected ($expected_tags) != snapshot risk_tags ($snapshot_tags)"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
  fi
done

echo "=== Dataset validation: $CHECKS_PASSED passed, $CHECKS_FAILED failed, $CHECKS_WARNED warnings ==="
[ "$CHECKS_FAILED" -eq 0 ]
```

---

## 10. Coverage Dashboard

A single file that shows what the dataset covers and where the gaps are.

### Format: `tests/benchmark/COVERAGE.md`

Generated automatically by a script, not maintained by hand:

```bash
#!/usr/bin/env bash
# scripts/generate-coverage.sh

echo "# Evidra Benchmark Dataset — Coverage Report"
echo ""
echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

total_cases=$(find cases -name expected.json | wc -l | tr -d ' ')
corpus_files=$(find corpus -type f 2>/dev/null | wc -l | tr -d ' ')
echo "**Cases:** $total_cases | **Corpus artifacts:** $corpus_files"
echo ""

# Derive signals dynamically from cases (not hardcoded list)
echo "## Signal Coverage"
echo ""
echo "| Signal / Risk Tag | FAIL Cases | PASS Cases | Total |"
echo "|-------------------|-----------|-----------|-------|"

# Collect all unique signals from expected.json files
all_signals=$(find cases -name expected.json -exec jq -r '
  (.risk_details_expected // [])[], 
  (.signals_expected // {} | keys[])
' {} \; | sort -u)

for signal in $all_signals; do
  fail=$(find cases -name expected.json -exec sh -c \
    'jq -e "(.risk_details_expected // [])[] == \"'"$signal"'\" or (.signals_expected.\"'"$signal"'\" // null) != null" "$1" >/dev/null 2>&1 && echo x' _ {} \; | wc -l | tr -d ' ')
  pass=$(find cases -name expected.json -exec sh -c \
    'jq -e ".pair_role == \"pass\" and .pair_id" "$1" >/dev/null 2>&1 && echo x' _ {} \; | wc -l | tr -d ' ')
  echo "| \`$signal\` | $fail | $pass | $((fail + pass)) |"
done

# Gaps
echo ""
echo "## Gaps (signals with < 2 FAIL cases)"
echo ""
# (Implemented above — any row with fail < 2 is a gap)

# By category
echo ""
echo "## By Category"
echo ""
echo "| Category | Cases |"
echo "|----------|-------|"
for cat in kubernetes terraform helm argocd scanner agent; do
  count=$(find cases -name expected.json -exec jq -r '.category' {} \; | grep -c "^${cat}$" || echo 0)
  echo "| $cat | $count |"
done

# By ground_truth_pattern
echo ""
echo "## By Ground Truth Pattern"
echo ""
echo "| Pattern | Cases |"
echo "|---------|-------|"
find cases -name expected.json -exec jq -r '.ground_truth_pattern' {} \; | sort | uniq -c | sort -rn | while read count pattern; do
  echo "| \`$pattern\` | $count |"
done

# By difficulty
echo ""
echo "## By Difficulty"
echo ""
echo "| Difficulty | Cases | % |"
echo "|-----------|-------|---|"
total=$(find cases -name expected.json | wc -l | tr -d ' ')
for diff in easy medium hard catastrophic; do
  count=$(find cases -name expected.json -exec jq -r '.difficulty' {} \; | grep -c "^${diff}$" || echo 0)
  pct=$(awk -v n="$count" -v d="$total" 'BEGIN { printf "%.0f", (d>0 ? n/d*100 : 0) }')
  echo "| $diff | $count | ${pct}% |"
done
```

Wire into CI:
```yaml
- name: Generate coverage report
  run: bash scripts/generate-coverage.sh > tests/benchmark/COVERAGE.md
```

Not committed to git (generated on every CI run) — or committed as convenience for browsing on GitHub.

### Critique: Auto-Generated vs Hand-Curated

Auto-generated coverage is accurate but lacks context. A hand-curated section explaining why certain gaps exist ("no ArgoCD cases yet because ArgoCD adapter is v0.5.0") adds value. 

**Recommendation:** Auto-generate the tables, hand-write a "Status and Gaps" section at the top that explains priorities and known limitations.

---

## 11. `evidra bench add` — CLI Scaffolding Tool

The DATASET_COLLECTOR_SKILL works for AI agents. For human contributors, a CLI tool is faster.

### Minimal Implementation

This is a shell script, not a Go command. Keep it simple:

```bash
#!/usr/bin/env bash
# scripts/bench-add.sh

set -euo pipefail

CASE_ID="${1:?Usage: bench-add.sh <case-id> [--artifact <path>] [--source <source-id>]}"
ARTIFACT=""
SOURCE=""

shift
while [[ $# -gt 0 ]]; do
  case $1 in
    --artifact) ARTIFACT="$2"; shift 2 ;;
    --source)   SOURCE="$2"; shift 2 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

CASE_DIR="tests/benchmark/cases/$CASE_ID"

if [ -d "$CASE_DIR" ]; then
  echo "Case $CASE_ID already exists at $CASE_DIR"
  exit 1
fi

mkdir -p "$CASE_DIR/snapshot"

# Generate README template
cat > "$CASE_DIR/README.md" << EOF
## Scenario: TODO — title

**Category:** TODO
**Difficulty:** TODO
**Source:** TODO

**Story:** TODO — what an agent would do and why it's risky.

**Impact:** TODO — what goes wrong.

**Risk:** TODO — concrete consequence.

**Real-world parallel:** TODO — known CVE, incident, or common misconfiguration.
EOF

# Compute artifact digest if artifact provided
ARTIFACT_DIGEST="TODO"
if [ -n "$ARTIFACT" ] && [ -f "tests/benchmark/$ARTIFACT" ]; then
  ARTIFACT_DIGEST="sha256:$(shasum -a 256 "tests/benchmark/$ARTIFACT" | awk '{print $1}')"
fi

# Generate expected.json template
cat > "$CASE_DIR/expected.json" << EOF
{
  "case_id": "$CASE_ID",
  "case_kind": "artifact",
  "category": "TODO",
  "difficulty": "TODO",
  "ground_truth_pattern": "TODO",
  "ground_truth": {
    "infrastructure_risk": "TODO",
    "blast_radius_resources": 1,
    "security_impact": "TODO",
    "attack_surface": "TODO"
  },
  "artifact_ref": "${ARTIFACT:-TODO}",
  "artifact_digest": "$ARTIFACT_DIGEST",
  "risk_details_expected": [],
  "risk_level": "TODO",
  "signals_expected": {},
  "tags": [],
  "source_refs": [
    {
      "source_id": "${SOURCE:-TODO}",
      "composition": "real-derived"
    }
  ]
}
EOF

# Create source manifest if needed
if [ -n "$SOURCE" ] && [ ! -f "tests/benchmark/sources/$SOURCE.md" ]; then
  cat > "tests/benchmark/sources/$SOURCE.md" << SRCEOF
# Benchmark Source: $SOURCE

## Manifest

\`\`\`yaml
source_id: $SOURCE
source_type: oss
source_url: TODO
source_path: TODO
source_commit_or_tag: TODO
source_license: TODO
retrieved_at: $(date -u +%Y-%m-%d)
retrieved_by: TODO
transformation_notes: |
  TODO
reviewer: pending
linked_cases:
  - $CASE_ID
\`\`\`
SRCEOF
  echo "Created source manifest: tests/benchmark/sources/$SOURCE.md (needs editing)"
fi

echo "Created case: $CASE_DIR"
echo ""
echo "Next steps:"
echo "  1. Edit $CASE_DIR/README.md — fill in story/impact/risk"
echo "  2. Edit $CASE_DIR/expected.json — fill TODOs"
echo "  3. Run: evidra prescribe --artifact $CASE_DIR/artifacts/* --signing-mode optional"
echo "  4. Save contract fields to $CASE_DIR/snapshots/contract.json"
echo "  5. Run: bash tests/benchmark/scripts/validate-dataset.sh"
```

Add to Makefile:
```makefile
bench-add:
	bash scripts/bench-add.sh $(CASE_ID) $(if $(ARTIFACT),--artifact $(ARTIFACT)) $(if $(SOURCE),--source $(SOURCE))
```

Usage:
```bash
make bench-add CASE_ID=k8s-hostpath-mount-fail ARTIFACT=~/downloaded/hostpath.yaml SOURCE=kubescape-regolibrary
```

### When to Promote to Go Command

When the shell script gets called 50+ times and people request features like auto-detection of category from artifact content, or auto-running prescribe to populate snapshot. Until then, shell is fine.

---

## 12. Implementation Priority

What to build first, what can wait.

| Priority | Item | Effort | Value |
|----------|------|--------|-------|
| **P0** | expected.json schema with case_kind + ground_truth_pattern + processing | 30 min | Everything depends on this |
| **P0** | `dataset.json` with dataset version + evidra_version_processed | 15 min | Version tracking from day one |
| **P0** | First 10 cases (hand-crafted, proves pipeline works) | 1 day | Real data before tooling |
| **P0** | `validate-dataset.sh` with all checks including version consistency | 2 hours | Catches errors at PR time |
| **P1** | `bench-add.sh` scaffolding script | 1 hour | 5x faster case creation |
| **P1** | PASS/FAIL pairs for top 5 detectors | 1 day | Precision measurement baseline |
| **P1** | Golden contract.json files for all cases (with evidra_version) | 2 hours | Stability guarantees |
| **P1** | COVERAGE.md generation script | 1 hour | Makes gaps visible |
| **P1** | `process-artifact.sh` with pinned Evidra binary | 1 hour | Consistent contribution processing |
| **P2** | Corpus directory + greedy collection via dataset collector skill | 1 day | Raw material scales |
| **P2** | `detect-duplicates.sh` fingerprint checker | 1 hour | Prevents bloat during collection |
| **P2** | Scenario (Layer C) directory structure | 2 hours | Needed when signal cases are built |
| **P2** | SARIF case class with projection validation | 2 hours | Needed for scanner integration |
| **P2** | `upgrade-dataset.sh` for Evidra version bumps | 2 hours | Needed at first Evidra release after dataset v1.0 |
| **P3** | Dedup report in COVERAGE.md (corpus size vs promoted cases) | 1 hour | Visibility into collection efficiency |
| **P3** | `evidra bench add` Go command | Later | When shell script limits are hit |
| **P3** | `verify-corpus-sources.sh` quarterly upstream check | 2 hours | Catches dead links and moved repos |

Start with P0: schema + 10 cases + validation. Cases first, tooling second. You need real data to know if the schema works, not the other way around.

---

## 13. Open Questions

| Question | Options | Recommendation |
|----------|---------|---------------|
| Store contract snapshots in git? | Yes (visible in PRs) / No (generated in CI) | **Yes** — PR visibility is the whole point |
| PASS case count per detector? | 1 / 2-3 / same as FAIL | **1 per detector** — minimal but sufficient |
| Corpus directory now or later? | Now / When 3+ cases share an artifact | **Now** — greedy collection needs a home from day one |
| Scenario minimum operations? | Bypass MinOperations / Lower threshold / Raw signal counts | **Raw signal counts** — bypass scorecard for per-case validation |
| Coverage report format? | Markdown / JSON / HTML dashboard | **Markdown** — renders on GitHub, no infra needed |
| Dedup check: warn or fail? | Warn only / Fail CI on duplicates | **Warn** — intentional duplicates exist (pairs, environment variants) |
| How cases reference artifacts? | Copy / Symlink / JSON ref | **JSON ref** — `artifact_ref` in expected.json, one file per case |
| How many axes must differ for distinct case? | Any 1 axis / At least 2 axes | **Any 1** — if prescribe output differs on any axis, it tests something different |
| Corpus size limit? | Unlimited / Cap per pattern | **No cap** — storage is cheap, re-collection is expensive |
| ground_truth_pattern namespace? | snake_case / dot-namespace | **dot-namespace** — consistent with OTel and Evidra signals |
| Dataset re-processing on Evidra upgrade? | Automatic / Manual with review | **Manual** — `upgrade-dataset.sh` + human review of git diff |
| Pinned Evidra binary storage? | Download on demand / Commit to repo / CI cache | **Download on demand** — `process-artifact.sh` fetches from GitHub Releases |
| Mixed Evidra versions during upgrade? | Block CI / Warn only | **Warn** — temporary state during upgrade, block on release tags |

---

## 14. Practical Starting Point

Do not build a dataset platform. Build a dataset.

**Week 1:**
- Finalize expected.json schema (30 min)
- Hand-craft 10 cases from existing test fixtures in the Evidra repo
- Create snapshots/contract.json for each
- Write validate-dataset.sh, run it, fix what breaks

**Week 2:**
- Add 5 PASS/FAIL pairs for top detectors
- Create 1 scenario (retry_loop — simplest multi-step)
- Generate COVERAGE.md, identify gaps

**Week 3:**
- Set up corpus, run dataset collector skill for greedy collection
- Promote best artifacts from corpus to new cases
- Run dedup checker

After 3 weeks you have ~20 cases, 5 pairs, 1 scenario, working CI validation, and — most importantly — firsthand knowledge of where the schema is awkward, where validation is too strict or too loose, and where the pipeline breaks. That knowledge is worth more than any architecture document.
