# Evidra LLM Risk Prediction — Prompt Contract

**Status:** Working specification — partially implemented
**Date:** March 2026
**Scope:** Prompt design for LLM-based risk prediction (REST API + evidra-exp)

### Implementation Status

| Section | Status | Dependency |
|---------|--------|-----------|
| §1 Three-Layer Architecture | **Design only** | — |
| §2 Tag Format Contract | **Apply now** to evidra-exp | — |
| §3 Tag Registry (YAML) | **After refactor** | V1_ARCHITECTURE task: "Detector registry export + prompt contract integration (Task 1 work plan)" |
| §4 System Prompt | **Apply now** to evidra-exp | — |
| §5 Few-Shot Examples | **Apply now** to evidra-exp | — |
| §6 API Call Parameters | **Apply now** to evidra-exp | — |
| §7 Server-Side Validation | **Apply now** to evidra-exp | — |
| §8 Layer 3 Scoring | **Apply now** to evidra-exp evaluator (`evaluation.go`) | — |
| §9–10 Promotion / Clustering | **After launch** | Production telemetry data |
| §11–13 Monitoring / Versioning | **After launch** | — |

**Apply now** = change `adapter_common.go` + `evaluation.go` in evidra-exp.
**After refactor** = depends on V1_ARCHITECTURE work-plan item: "Detector registry export + prompt contract integration (Task 1 work plan)".
**After launch** = requires production usage data.

---

## 1. Three-Layer Risk Detection

Evidra has three layers, each with distinct authority:

**Layer 1 — Deterministic detectors** (`internal/detectors/`). Go code that pattern-matches YAML/JSON structure. Returns registered tags. Always correct when it fires, limited coverage.

**Layer 2 — LLM prediction** (REST API). Model analyzes artifacts and predicts risk tags. Broader coverage — discovers patterns not yet in Go detectors. Output validated server-side. **Returns tags only, not risk level.**

**Layer 3 — Server-side scoring** (REST API). Computes `risk_level` from the union of Layer 1 + validated Layer 2 tags, using the registry's severity definitions and policy rules. This is the **sole authority** for risk level computation.

```
Artifact (YAML/JSON)
       │
       ├──► Layer 1: Go detectors (deterministic)
       │    Returns: registered tags
       │
       └──► Layer 2: LLM prediction (probabilistic)
            Returns: candidate + registered tag IDs (no risk level)
            │
            ▼
       Layer 3: Server validation + scoring
            │
            ├──► normalize namespaces (terraform.* → tf.*)
            ├──► classify: registered vs candidate
            ├──► merge: L1 tags ∪ L2 registered tags → risk_details
            ├──► compute: risk_level from tag severities + policy rules
            └──► store: candidates with provenance for analytics
```

**Source of truth is the server-validated ontology** — not Layer 1 alone, and not Layer 2 alone. Layer 1 provides deterministic detection. Layer 2 provides breadth. Layer 3 provides authority over what goes into scoring, evidence, and benchmarks.

**Why LLM does not return risk_level:** LLM risk levels are less stable than tags across models and temperatures. The same artifact might get "high" from Haiku and "critical" from Sonnet. Computing risk_level server-side from registered tag severities makes it deterministic and auditable. The formula is: `risk_level = max(severity of all registered tags in risk_details) elevated by scope_class via risk matrix`.

### Evaluator Compatibility (evidra-exp)

> **Breaking change:** `evaluation.go` currently expects `predicted_risk_level` from the model output. This contract removes it. The evaluator MUST be updated to accept server-computed risk_level instead.

**Required code change in evaluation.go:**

```go
// BEFORE (broken with this contract — LLM no longer returns risk_level):
//   predicted_level := llmOutput.PredictedRiskLevel
//   levelMatch := predicted_level == expected.RiskLevel

// AFTER (risk_level computed from validated tags):
//   validatedTags := validate(llmOutput.PredictedRiskTags)
//   predicted_level := computeRiskLevel(validatedTags)
//   levelMatch := predicted_level == expected.RiskLevel
```

Server-side severity map:

```go
var tagSeverity = map[string]string{
    "k8s.privileged_container":  "critical",
    "k8s.hostpath_mount":        "high",
    "k8s.host_namespace_escape": "critical",
    "k8s.docker_socket":         "critical",
    "k8s.run_as_root":           "medium",
    "k8s.dangerous_capabilities":"high",
    "ops.mass_delete":           "critical",
    "ops.kube_system":           "high",
    "aws.iam_wildcard_policy":   "critical",
    "aws.s3_public_access":      "high",
    "tf.iam_wildcard_policy":    "high",
    "tf.s3_public_access":       "high",
}

func computeRiskLevel(tags []string) string {
    order := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}
    max := "low"
    for _, tag := range tags {
        if sev, ok := tagSeverity[tag]; ok {
            if order[sev] > order[max] {
                max = sev
            }
        }
    }
    return max
}
```

**This change applies to BOTH the REST API path AND evidra-exp.** The prompt and validation pipeline are identical for both. The only difference: REST API may additionally merge with Go detector output (Layer 1 + Layer 2), while evidra-exp runs LLM-only.

---

## 2. Tag Format Contract (normative)

### Format Rule

Every tag MUST follow dot-namespace format:

```
{namespace}.{pattern_name}
```

### Namespace Taxonomy (normative)

Namespaces are organized by **artifact domain** — the tool ecosystem that produces the artifact:

| Namespace | Domain | Examples |
|-----------|--------|---------|
| `k8s` | Kubernetes workloads | k8s.privileged_container, k8s.hostpath_mount |
| `tf` | Terraform / IaC | tf.iam_wildcard, tf.s3_public_access |
| `helm` | Helm charts | helm.kube_system_install |
| `aws` | AWS-specific patterns | aws.iam_wildcard_policy, aws.s3_public_bucket |
| `gcp` | GCP-specific patterns | gcp.iam_wildcard (future) |
| `ops` | Operational patterns (tool-agnostic) | ops.mass_delete, ops.namespace_delete |

**Design rationale:** Artifact domain is chosen over risk domain (workload/network/iam) because it matches how users think ("I'm deploying Terraform") and how adapters are structured in code (`internal/canon/`). Provider-specific namespaces (`aws`, `gcp`) replace the old `aws_iam` convention to avoid the scaling problem (`aws_iam`, `aws_s3`, `aws_ec2`... → just `aws`).

**Legacy aliases:** `terraform.*` and `aws_iam.*` are accepted by the server normalizer and mapped to `tf.*` and `aws.*` respectively. They MUST NOT appear in the prompt, registry, or new code. See section 7 normalization.

**Pattern name:** snake_case, lowercase, no spaces, descriptive.

Examples of valid tags:
```
k8s.privileged_container       ← registered
k8s.docker_socket              ← candidate (LLM discovered)
tf.rds_public                  ← candidate
aws.iam_wildcard_policy        ← registered
ops.kube_system                ← candidate
```

Examples of INVALID tags (rejected by server):
```
"Allows arbitrary container creation"        ← prose, no namespace
"Docker socket mount enables escape"         ← prose
"K8s.HostPath"                               ← uppercase
"hostpath_mount"                             ← missing namespace
"security.misconfiguration"                  ← unknown namespace
"terraform.s3_public_access"                 ← legacy namespace (normalized to tf.s3_public_access)
"aws_iam.wildcard_policy"                    ← legacy namespace (normalized to aws.iam_wildcard_policy)
```

Note: legacy-namespaced tags are not rejected — they are normalized. But the LLM should never produce them because they don't appear in the prompt.

### Registered vs Candidate Tags

| Type | Definition | Usage |
|------|-----------|-------|
| **Registered** | In the tag registry (section 3) | Scoring, benchmark comparison, evidence chain, percentiles |
| **Candidate** | Valid format but not in registry | Logged, analyzed, promoted to registry when frequent |

Server determines registration status, not the LLM. The LLM outputs tags; the server classifies them.

### No Prose Field (design decision)

The output format intentionally has NO field for free-text reasoning, explanation, or description. No `"reasoning"`, no `"observations"`, no `"details"`.

Why: when the output schema includes a text field alongside a tag field, models conflate them. The original `predicted_risk_details` field was intended for tags but models wrote prose because the name suggested "details." Even renaming to `predicted_risk_tags` doesn't fully prevent contamination when a prose field exists next to it.

If an explanation is needed (debugging, audit), make a separate API call with a different prompt that asks only for reasoning. Do not mix classification and explanation in one response.

Rejected tags from server validation already serve as a signal when the model drifts toward prose — monitor the `rejected_tags` rate.

---

## 3. Tag Registry

The registry is the source of truth for established risk patterns. Tags enter the registry through the promotion process (section 9).

> **Current code divergence:** Existing Go detectors still emit legacy tag IDs (`terraform.iam_wildcard_policy`, `terraform.s3_public_access`, `aws_iam.wildcard_policy`). The canonical names below (`tf.*`, `aws.*`) take effect after the detector refactor tracked in V1_ARCHITECTURE as "Detector registry export + prompt contract integration (Task 1 work plan)". Until then, the server validation layer (§7) normalizes legacy → canonical at runtime.

### Current Registry (v1.0)

> **Implementation note:** This YAML file does not exist in the repo yet. After the detector architecture refactor (see V1_ARCHITECTURE task: "Detector registry export + prompt contract integration (Task 1 work plan)"), the registry is auto-generated from Go code via `evidra detectors list --format yaml`. The YAML below is the target schema.

```yaml
# prompts/risk-prediction/tag-registry.yaml  (to be generated)
version: "1.0"

namespaces: [k8s, tf, aws, ops, helm]

legacy_aliases:
  terraform: tf
  aws_iam: aws

tags:
  k8s.privileged_container:
    severity: critical
    description: Container with securityContext.privileged=true
    go_detector: true

  k8s.hostpath_mount:
    severity: high
    description: Pod with hostPath volume mount
    go_detector: true

  k8s.host_namespace_escape:
    severity: critical
    description: Pod with hostPID, hostIPC, or hostNetwork
    go_detector: true

  ops.mass_delete:
    severity: critical
    description: Destroy operation affecting >10 resources
    go_detector: true

  aws.iam_wildcard_policy:
    severity: critical
    description: IAM with both Action:* AND Resource:*
    go_detector: true
    legacy_id: aws_iam.wildcard_policy

  tf.iam_wildcard_policy:
    severity: high
    description: IAM with Action:* OR Resource:*
    go_detector: true
    legacy_id: terraform.iam_wildcard_policy

  tf.s3_public_access:
    severity: high
    description: S3 bucket without complete public access block
    go_detector: true
    legacy_id: terraform.s3_public_access
```

This list will grow organically as candidate tags are promoted.

---

## 4. System Prompt

```
You are an infrastructure security risk analyzer for the Evidra benchmark system.

Analyze the provided infrastructure artifact (Kubernetes YAML, Terraform plan, Helm chart) and identify security risk patterns.

OUTPUT FORMAT (strict JSON, no preamble):
{
  "predicted_risk_tags": ["namespace.pattern_name", ...]
}

Return ONLY predicted_risk_tags. Risk level is computed server-side.

TAG FORMAT — every tag must be:
  {namespace}.{pattern_name}
  Namespaces: k8s, tf, aws, ops, helm
  Pattern: snake_case, lowercase, no spaces, descriptive

KNOWN TAGS (use EXACT strings when they match):
  k8s.privileged_container — securityContext.privileged=true
  k8s.hostpath_mount — hostPath volume mount
  k8s.host_namespace_escape — hostPID, hostIPC, or hostNetwork
  ops.mass_delete — operation deleting >10 resources
  aws.iam_wildcard_policy — IAM with Action:* AND Resource:*
  tf.iam_wildcard_policy — IAM with Action:* OR Resource:*
  tf.s3_public_access — S3 bucket without public access block

You may ALSO identify patterns NOT in this list.
Use the same namespace.pattern format. Examples:
  k8s.docker_socket — mounts /var/run/docker.sock
  k8s.run_as_root — runs as UID 0
  k8s.dangerous_capabilities — SYS_ADMIN, NET_ADMIN, or ALL
  tf.security_group_open — ingress 0.0.0.0/0 on sensitive ports
  ops.namespace_delete — deletes an entire namespace

RULES:
  1. Tags must be namespace.pattern format. No prose, no sentences, no explanations.
  2. If a pattern matches a known tag, use the EXACT known tag string. Do not create variants.
  3. If no risk detected, return: {"predicted_risk_tags": []}
  4. Multiple tags allowed for compound risks.
  5. Return ONLY the JSON object. No preamble, no markdown fences.
```

Key design decisions:
- **No `predicted_risk_level` in output.** Risk level is computed server-side from tag severities in the registry. This makes scoring deterministic and auditable, independent of which LLM ran the prediction.
- "You may ALSO identify patterns NOT in this list" — turns the LLM from classifier into discovery engine.
- Rule 2 prevents cross-model fragmentation — all models converge on known tag strings.
- No prose field — keeps classification clean (see section 2).

---

## 5. Few-Shot Examples

### Example 1: Known Tag (Privileged Container)

**User:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: debug-pod
spec:
  containers:
    - name: debug
      image: ubuntu:22.04
      securityContext:
        privileged: true
```

**Assistant:**
```json
{"predicted_risk_tags": ["k8s.privileged_container"]}
```

### Example 2: Candidate Tag (Docker Socket — Not in Registry)

**User:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: builder
spec:
  containers:
    - name: docker
      image: docker:24-dind
      volumeMounts:
        - name: sock
          mountPath: /var/run/docker.sock
  volumes:
    - name: sock
      hostPath:
        path: /var/run/docker.sock
```

**Assistant:**
```json
{"predicted_risk_tags": ["k8s.hostpath_mount", "k8s.docker_socket"]}
```

Note: `k8s.hostpath_mount` is registered, `k8s.docker_socket` is a candidate. Both use correct format. Server classifies them.

### Example 3: No Risk (empty tags)

**User:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: staging
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: nginx:1.25
          securityContext:
            runAsNonRoot: true
            readOnlyRootFilesystem: true
```

**Assistant:**
```json
{"predicted_risk_tags": []}
```

### Example 4: Compound (Multiple Known + Candidate)

**User:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: admin
  namespace: kube-system
spec:
  hostNetwork: true
  hostPID: true
  containers:
    - name: admin
      image: ubuntu:22.04
      securityContext:
        privileged: true
        capabilities:
          add: ["SYS_ADMIN"]
      volumeMounts:
        - name: root
          mountPath: /host
  volumes:
    - name: root
      hostPath:
        path: /
```

**Assistant:**
```json
{"predicted_risk_tags": ["k8s.privileged_container", "k8s.host_namespace_escape", "k8s.hostpath_mount", "k8s.dangerous_capabilities", "ops.kube_system"]}
```

### Example 5: Terraform IAM Wildcard

**User:**
```json
{"resource_changes": [{"type": "aws_iam_policy", "change": {"actions": ["create"], "after": {"policy": "{\"Statement\":[{\"Effect\":\"Allow\",\"Action\":\"*\",\"Resource\":\"*\"}]}"}}}]}
```

**Assistant:**
```json
{"predicted_risk_tags": ["aws.iam_wildcard_policy", "tf.iam_wildcard_policy"]}
```

### Example 6: Tempting Prose — Correct Answer Is Tags Only (negative example)

**User:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: monitoring-agent
spec:
  containers:
    - name: agent
      image: datadog/agent:7
      env:
        - name: DD_API_KEY
          value: "abc123def456"
      volumeMounts:
        - name: proc
          mountPath: /host/proc
          readOnly: true
  volumes:
    - name: proc
      hostPath:
        path: /proc
```

**Assistant:**
```json
{"predicted_risk_tags": ["k8s.hostpath_mount"]}
```

Note: the hardcoded API key is a concern, but it is not an infrastructure risk pattern in our taxonomy. Do NOT output "hardcoded credential" or "secret in env" — there is no matching namespace.pattern for secrets. Return only tags that match the format.

### Example 7: ConfigMap — No Risk At All (negative example)

**User:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  LOG_LEVEL: info
  MAX_RETRIES: "3"
```

**Assistant:**
```json
{"predicted_risk_tags": []}
```

---

## 6. API Call Parameters

```python
response = client.messages.create(
    model="claude-haiku-4-5-20251001",
    max_tokens=150,
    temperature=0,
    system=SYSTEM_PROMPT,
    messages=[
        {"role": "user", "content": example_1_yaml},
        {"role": "assistant", "content": example_1_json},
        {"role": "user", "content": example_2_yaml},
        {"role": "assistant", "content": example_2_json},
        {"role": "user", "content": example_3_yaml},
        {"role": "assistant", "content": example_3_json},
        {"role": "user", "content": example_6_yaml},
        {"role": "assistant", "content": example_6_json},
        {"role": "user", "content": actual_artifact},
    ]
)
```

- **temperature=0** — deterministic, eliminates prose drift
- **max_tokens=150** — tags-only JSON is ~30-80 tokens, lower cap than before
- **Haiku** — cheapest model, sufficient for structured tag prediction
- **Few-shot includes negative examples** — examples 3, 6, 7 teach the model when NOT to tag

Cost: ~$0.0005 per prediction. 1000/day = $0.50.

---

## 7. Server-Side Validation & Normalization

Never trust LLM output. Pipeline: normalize → validate → classify → store.

```python
import re
import json

CANONICAL_NAMESPACES = {"k8s", "tf", "aws", "ops", "helm"}

# Legacy namespace aliases — accepted, normalized, never in prompt
NAMESPACE_ALIASES = {
    "terraform": "tf",
    "aws_iam": "aws",
}

TAG_PATTERN = re.compile(r'^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$')

def load_registry(path="prompts/risk-prediction/tag-registry.yaml") -> dict:
    """Load registry with tag metadata."""
    import yaml
    with open(path) as f:
        data = yaml.safe_load(f)
    return data.get("tags", {})

REGISTRY = load_registry()
REGISTERED_TAGS = set(REGISTRY.keys())

# --- Step 1: Normalize ---

def normalize_tag(tag: str) -> str | None:
    """Normalize namespace aliases, trim, lowercase."""
    tag = tag.strip().lower()
    if not TAG_PATTERN.match(tag):
        return None  # prose, uppercase, spaces — reject

    namespace, pattern = tag.split(".", 1)

    # Apply namespace alias
    if namespace in NAMESPACE_ALIASES:
        namespace = NAMESPACE_ALIASES[namespace]

    if namespace not in CANONICAL_NAMESPACES:
        return None  # unknown namespace

    return f"{namespace}.{pattern}"

# --- Step 2: Validate & Classify ---

def validate_prediction(
    raw_json: str,
    model_family: str = "unknown",
    model_name: str = "unknown",
    artifact_kind: str = "unknown",
    artifact_excerpt: str = "",
    prompt_contract_version: str = "unknown",
) -> dict:
    """Validate, normalize, and classify LLM risk prediction output."""
    clean = raw_json.strip().removeprefix("```json").removesuffix("```").strip()
    data = json.loads(clean)

    raw_tags = data.get("predicted_risk_tags", [])
    registered_tags = []
    candidate_tags = []
    rejected = []

    for raw_tag in raw_tags:
        normalized = normalize_tag(raw_tag)

        if normalized is None:
            rejected.append(raw_tag)
            continue

        if normalized in REGISTERED_TAGS:
            registered_tags.append(normalized)
        else:
            candidate_tags.append({
                "tag": normalized,
                "raw_tag": raw_tag,  # before normalization
                "model_family": model_family,
                "model_name": model_name,
                "artifact_kind": artifact_kind,
                "artifact_excerpt": artifact_excerpt[:500],
                "prompt_contract_version": prompt_contract_version,
            })

    # Deduplicate
    registered_tags = sorted(set(registered_tags))

    if rejected:
        log.warn(f"LLM ({model_name}) returned invalid tags: {rejected}")

    return {
        "registered_tags": registered_tags,
        "candidate_tags": candidate_tags,
        "rejected_tags": rejected,
    }
```

Pipeline:
1. **Normalize** — trim, lowercase, map legacy namespaces (`terraform.` → `tf.`, `aws_iam.` → `aws.`)
2. **Validate** — regex format check, namespace whitelist
3. **Classify** — registry lookup: registered vs candidate
4. **Deduplicate** — sort + dedup registered tags

No `predicted_risk_level` in output. Risk level computed in Layer 3 (section 8).

### Candidate Storage

Candidates include full context for reviewability:

```sql
CREATE TABLE candidate_tags (
    id                       SERIAL PRIMARY KEY,
    tag                      TEXT NOT NULL,        -- normalized
    raw_tag                  TEXT,                 -- before normalization
    model_family             TEXT NOT NULL,
    model_name               TEXT,                 -- e.g. "claude-haiku-4-5"
    artifact_kind            TEXT,                 -- e.g. "k8s/Deployment", "terraform/plan"
    artifact_excerpt         TEXT,                 -- first 500 chars for review context
    surrounding_registered   TEXT[],               -- registered tags from same prediction
    prompt_contract_version  TEXT,
    session_id               TEXT,
    created_at               TIMESTAMPTZ DEFAULT now()
);
```

This enables promotion review without re-analyzing the original artifact.

Three outputs:
- **registered_tags** → `["k8s.hostpath_mount"]` — scoring, evidence, benchmark
- **candidate_tags** → `[{"tag": "k8s.docker_socket", "model_family": "claude", ...}]` — logged with context for review
- **rejected_tags** → `["Docker socket mount enables escape"]` — prompt debugging, discarded

---

## 8. Layer 3: Server-Side Scoring

Risk level is computed entirely server-side from registered tag severities. No LLM input to risk_level.

> **Current runtime divergence:** The algorithm below is the target. Current `internal/risk/matrix.go` uses a simpler model: `RiskLevel(operationClass, scopeClass)` returns a base level from the op×scope matrix, then `ElevateRiskLevel()` bumps by one step if ANY risk tags are present (regardless of tag severity). The target algorithm below is more precise: base = max(individual tag severities), then elevate by scope. For evidra-exp evaluation, use the simpler `computeRiskLevel(tags)` from the Evaluator Compatibility section (§1) which just takes max tag severity — this is sufficient and avoids scope complexity in experiments where scope is often unknown.

```python
def compute_risk_assessment(
    go_tags: list[str],
    llm_registered: list[str],
    llm_candidates: list[dict],
    operation_class: str,
    scope_class: str,
    registry: dict,
) -> dict:
    """Layer 3: merge tags and compute risk_level from registry severities."""

    # Final tags: Go detector + LLM registered (union, deduped, sorted)
    risk_details = sorted(set(go_tags + llm_registered))

    # Risk level: max severity from registered tags in risk_details
    SEVERITY_ORDER = {"low": 0, "medium": 1, "high": 2, "critical": 3}
    base_level = "low"
    for tag in risk_details:
        tag_severity = registry.get(tag, {}).get("severity", "low")
        if SEVERITY_ORDER.get(tag_severity, 0) > SEVERITY_ORDER.get(base_level, 0):
            base_level = tag_severity

    # Elevate by scope (same logic as Go risk matrix)
    final_level = elevate_risk_level(base_level, operation_class, scope_class)

    return {
        "risk_level": final_level,
        "risk_details": risk_details,
        "candidate_tags": [c["tag"] for c in llm_candidates],
        "provenance": {
            "detector_tags": go_tags,
            "llm_registered_tags": llm_registered,
            "llm_candidate_tags": [c["tag"] for c in llm_candidates],
            "base_severity": base_level,
            "scope_elevation": final_level != base_level,
        }
    }

def elevate_risk_level(base: str, op_class: str, scope: str) -> str:
    """Apply risk matrix elevation: production + mutate → escalate."""
    if scope == "production" and op_class in ("mutate", "destroy"):
        ELEVATE = {"low": "medium", "medium": "high", "high": "critical", "critical": "critical"}
        return ELEVATE.get(base, base)
    return base
```

**Three authorities, clearly separated:**
- **Layer 1** (Go detectors) → deterministic tags
- **Layer 2** (LLM) → additional registered tags + candidate tags
- **Layer 3** (Server) → risk_level from `max(tag severities) × risk_matrix(scope)`

**Candidates are NOT in risk_details.** They do not affect scoring until promoted to the registry. This prevents an LLM hallucination from affecting a user's reliability score.

---

## 9. Tag Promotion Flywheel

```
LLM predicts k8s.docker_socket (not in registry)
  → server classifies as candidate_tag
  → stored in provenance, aggregated weekly
  
After 4 weeks:
  k8s.docker_socket appeared 47 times across 23 sessions
  Agreement: when k8s.docker_socket appears, k8s.hostpath_mount also appears 100% of time
  Manual review: yes, this is a real pattern

Promotion:
  1. Add to tag-registry.yaml (go_detector: false)
  2. Update system prompt (moves from "you may also identify" examples to known tags list)
  3. Optionally: implement Go detector for deterministic detection
  4. Bump registry version (1.0 → 1.1)

Result:
  k8s.docker_socket is now registered
  → appears in scoring, benchmark, percentiles
  → few-shot example already exists (example 2 above)
```

### Promotion Criteria

Promotion is evaluated on **clusters**, not individual tag strings. A cluster may contain 4 variants from 4 models — the total occurrences across all variants count.

| Criterion | Threshold |
|-----------|-----------|
| Cluster total occurrences | ≥ 20 across ≥ 10 sessions |
| Multi-model agreement | ≥ 2 models produced tags in the cluster |
| Manual review | Confirms real security pattern |
| False positive rate | < 15% on manual sample of 10 artifacts |

The **canonical name** for the promoted tag is the most frequent variant in the cluster.

### Demotion

Candidate clusters with < 5 total occurrences over 90 days are pruned from active analytics. Raw data is not deleted from stored provenance — just no longer tracked in weekly reports.

---

## 10. Cross-Model Fragmentation & Candidate Clustering

### The Problem

Different models name the same pattern differently:

| Model | Artifact | Candidate Tag |
|-------|----------|--------------|
| Haiku | docker.sock mount | `k8s.docker_socket` |
| Gemini Flash | docker.sock mount | `k8s.docker_sock_mount` |
| DeepSeek | docker.sock mount | `k8s.host_docker_access` |
| GPT-4o-mini | docker.sock mount | `k8s.docker_socket_mount` |

All four refer to the same risk pattern. Without clustering, analytics shows four separate candidates with 12 occurrences each instead of one cluster with 48.

### Solution: Registry as Attractor + Server-Side Clustering

**Before promotion:** fragmentation is expected. Server clusters similar candidates weekly.

**After promotion:** the canonical tag appears in "known tags" in the prompt. All models converge on the exact string because they see it. Fragmentation disappears for that pattern.

### Candidate Storage

Uses the `candidate_tags` table defined in section 7, which includes `model_name`, `artifact_kind`, `artifact_excerpt`, and `surrounding_registered` for review context.

### Raw Analytics (per-tag)

```sql
SELECT
    tag,
    count(*) as occurrences,
    count(DISTINCT session_id) as sessions,
    array_agg(DISTINCT model_family) as models,
    min(created_at) as first_seen
FROM candidate_tags
WHERE created_at > now() - interval '30 days'
GROUP BY tag
ORDER BY occurrences DESC
LIMIT 30;
```

### Clustering Logic

```python
from difflib import SequenceMatcher

def cluster_candidates(raw_analytics: list[dict]) -> list[dict]:
    """Group candidate tags that likely refer to the same risk pattern."""
    clusters = []
    used = set()

    # Sort by occurrences descending — most popular tag becomes canonical
    sorted_tags = sorted(raw_analytics, key=lambda x: -x["occurrences"])

    for item in sorted_tags:
        if item["tag"] in used:
            continue

        cluster = [item]
        used.add(item["tag"])

        for other in sorted_tags:
            if other["tag"] in used:
                continue

            # Same namespace + similar pattern name → likely same risk
            tag_ns = item["tag"].split(".")[0]
            other_ns = other["tag"].split(".")[0]

            if tag_ns == other_ns:
                similarity = SequenceMatcher(
                    None, item["tag"], other["tag"]
                ).ratio()
                if similarity > 0.65:
                    cluster.append(other)
                    used.add(other["tag"])

        # Canonical = most frequent variant
        canonical = cluster[0]["tag"]
        total = sum(c["occurrences"] for c in cluster)
        all_models = set()
        for c in cluster:
            all_models.update(c.get("models", []))

        clusters.append({
            "canonical": canonical,
            "variants": [c["tag"] for c in cluster],
            "total_occurrences": total,
            "sessions": sum(c["sessions"] for c in cluster),
            "models": sorted(all_models),
            "fragmentation": len(cluster),  # 1 = clean, 4 = highly fragmented
        })

    return sorted(clusters, key=lambda x: -x["total_occurrences"])
```

### Weekly Report Output

```
=== Candidate Tag Clusters (last 30 days) ===

Cluster: k8s.docker_socket (48 total, 4 variants, 3 models)
  k8s.docker_socket          15  [claude]
  k8s.docker_socket_mount    12  [gpt]
  k8s.docker_sock_mount      12  [gemini]
  k8s.host_docker_access      9  [deepseek]
  → PROMOTE as: k8s.docker_socket (meets threshold: ≥20 occ, ≥10 sessions)

Cluster: k8s.run_as_root (34 total, 1 variant, 4 models)
  k8s.run_as_root            34  [claude, gpt, gemini, deepseek]
  → PROMOTE as: k8s.run_as_root (clean — all models agree on name)

Cluster: k8s.dangerous_capabilities (31 total, 2 variants, 3 models)
  k8s.dangerous_capabilities 22  [claude, gpt]
  k8s.unsafe_capabilities     9  [deepseek]
  → PROMOTE as: k8s.dangerous_capabilities (review: is deepseek variant same pattern?)

Cluster: tf.rds_public (12 total, 1 variant, 2 models)
  tf.rds_public              12  [claude, gpt]
  → Below threshold (need ≥20). Continue monitoring.

Cluster: k8s.emptydir_no_limit (3 total, 1 variant, 1 model)
  k8s.emptydir_no_limit       3  [claude]
  → Noise. Will prune if no growth in 60 days.
```

### Model Quality Insights

From the same data, derive which models produce the cleanest candidates:

```sql
SELECT
    model_family,
    count(*) as total_candidates,
    count(DISTINCT tag) as unique_tags,
    count(*) / count(DISTINCT tag) as consistency_ratio
FROM candidate_tags
WHERE created_at > now() - interval '30 days'
GROUP BY model_family
ORDER BY consistency_ratio DESC;
```

```
model_family | total_candidates | unique_tags | consistency_ratio
claude       | 120              | 18          | 6.7  (consistent)
gpt          | 95               | 22          | 4.3  (moderate)
gemini       | 78               | 31          | 2.5  (fragmented)
deepseek     | 64               | 29          | 2.2  (fragmented)
```

Higher consistency_ratio = model reuses the same tag strings. Lower = model invents new names for the same patterns. This informs which model to weight more heavily in promotion decisions.

---

## 11. Monitoring

| Metric | What It Measures | Target |
|--------|-----------------|--------|
| Format rejection rate | % of LLM tags failing namespace/format check | < 5% |
| Registered tag hit rate | % of LLM tags that match registry | 60-80% |
| Candidate discovery rate | Unique new candidate clusters per week | 2-5 |
| Candidate fragmentation | Average variants per cluster | < 3 |
| Model consistency ratio | Tags per unique tag string per model | > 4 |
| L1/L2 agreement rate | % of artifacts where Go and LLM agree | > 80% |
| LLM-only contribution | % of final risk_details from LLM only | 10-30% |

**Format rejection > 10%** → prompt needs more format enforcement (add few-shot for the failing pattern types).

**Registered hit rate > 95%** → LLM is not discovering anything new. Consider reducing Layer 2 usage or running against novel artifact types.

**Registered hit rate < 50%** → LLM is hallucinating too many candidates. Tighten prompt, add negative examples.

**Fragmentation > 4 variants/cluster** → models are not converging on names. Add the most common variant to "known tags" examples in prompt even before formal promotion — this acts as an attractor without requiring full registry promotion.

---

## 12. Contract Versioning

The model returns only tags. The server appends contract version to the stored prediction:

```json
{
  "predicted_risk_tags": ["k8s.hostpath_mount", "k8s.docker_socket"],
  "prompt_contract_version": "v1.1.0",
  "_note": "prompt_contract_version is added SERVER-SIDE, not by the model"
}
```

The model output is strictly `{"predicted_risk_tags": [...]}`. The server knows which prompt version was used and attaches it for traceability.

| Change | Version Bump |
|--------|-------------|
| Wording, few-shot examples | **patch** (v1.0.0 → v1.0.1) |
| New tags promoted to registry | **minor** (v1.0 → v1.1) |
| Namespace added/removed, format change | **major** (v1 → v2) |

Store `prompt_contract_version` with every prediction.

---

## 13. Tag Registry Update Process

> **Implementation note:** The files and scripts below (`prompts/risk-prediction/`, `scripts/generate-risk-prompt.py`) do not exist yet. After the detector refactor (tracked in V1_ARCHITECTURE task: "Detector registry export + prompt contract integration (Task 1 work plan)"), registry is generated from Go code: `evidra detectors list --format yaml`. The script below shows the target automation for prompt generation.

When a candidate tag is promoted:

```bash
# 1. Add Go detector (canonical source of truth)
# internal/detectors/k8s/docker_socket.go (self-registering via init())

# 2. Generate registry YAML from code (future command, available after Task 1)
evidra detectors list --format yaml > prompts/risk-prediction/tag-registry.yaml

# 3. Generate system prompt from registry
python scripts/generate-risk-prompt.py > prompts/risk-prediction/system-prompt.txt

# 4. Bump registry version
# version: "1.0" → "1.1"

# 5. Bump contract version
# prompt_contract_version: v1.0.1 → v1.1.0

# 6. Add benchmark case
# tests/benchmark/cases/k8s-docker-socket-fail/
```

Step 3 is the key automation: the system prompt's "known tags" list is generated from the registry YAML (which itself is generated from Go code). One source of truth: the Detector's `Metadata()`.

```python
# scripts/generate-risk-prompt.py (to be created)
import yaml

with open("prompts/risk-prediction/tag-registry.yaml") as f:
    registry = yaml.safe_load(f)

known_tags_section = "KNOWN TAGS (use when they match):\n"
for tag_id, meta in registry["tags"].items():
    known_tags_section += f"  {tag_id} — {meta['description']}\n"

# Insert into system prompt template
template = open("prompts/risk-prediction/system-prompt.template").read()
prompt = template.replace("{{KNOWN_TAGS}}", known_tags_section)
open("prompts/risk-prediction/system-prompt.txt", "w").write(prompt)
```
