# Anonymized Evidence Export Design

**Date:** 2026-03-15
**Status:** Proposed
**Repo:** evidra (core)

---

## Problem

When users hit issues with evidra or want to share benchmark results, they need
to send evidence chains. But evidence contains sensitive data: namespace names,
resource names, actor identities, and sometimes artifact content with secrets
or configuration values.

There's no way to share evidence safely without manual redaction, which is
error-prone and discourages reporting.

## Goal

`evidra export --anonymize` produces a shareable bundle where:
- All behavioral signals and scores are preserved exactly
- All structural patterns are preserved (operation class, resource count, scope)
- All identifying information is replaced with deterministic hashes
- Raw artifact content is stripped (only digests kept)
- The bundle is self-contained and can be scored/audited by the evidra team

## Why the Canonicalization Layer Makes This Easy

The existing canonicalization already separates structure from content:

| Already safe (canonical form) | Needs anonymization |
|-------------------------------|---------------------|
| `tool` (kubectl, terraform) | `ResourceIdentity[].namespace` |
| `operation` (apply, delete) | `ResourceIdentity[].name` |
| `operation_class` (mutate, destroy) | `actor.id` |
| `scope_class` (production, staging) | `actor.instance_id` |
| `resource_count` | `actor.origin` |
| `resource_shape_hash` | `session_id` (correlatable) |
| `artifact_digest` (SHA256) | `operation_id` (correlatable) |
| `intent_digest` (SHA256) | `trace_id`, `span_id` |
| All signals and signal counts | Raw artifact content (if stored) |
| Score, band, scoring profile | Evidence file paths |
| Risk inputs, effective risk | Scope dimensions values |
| Timestamps | |
| Entry ordering and hash chain | |

The anonymization surface is small — most of the value is already in the
safe canonical form.

## Proposed CLI

```bash
# Export anonymized evidence bundle
evidra export \
  --evidence-dir /path/to/evidence \
  --anonymize \
  --output evidence-bundle.tar.gz

# Export with scorecard included
evidra export \
  --evidence-dir /path/to/evidence \
  --anonymize \
  --include-scorecard \
  --output evidence-bundle.tar.gz

# Export from infra-bench run artifacts
evidra export \
  --evidence-dir runs/e2e/broken-deployment-sonnet/evidence \
  --run-dir runs/e2e/broken-deployment-sonnet/20260315-172308-broken-deployment-cli \
  --anonymize \
  --output issue-report.tar.gz
```

## Anonymization Rules

### Deterministic Hashing

All identifiers are replaced with `SHA256(salt + original)[:8]` so that:
- Same original → same hash (preserves correlations within one export)
- Different exports use different salts (can't cross-correlate)
- 8 hex chars is enough for uniqueness within a single evidence chain

### What Gets Anonymized

| Field | Original | Anonymized |
|-------|----------|------------|
| `ResourceIdentity[].namespace` | `payments-prod` | `ns-a1b2c3d4` |
| `ResourceIdentity[].name` | `api-gateway` | `res-e5f6a7b8` |
| `actor.id` | `claude-code` | `actor-9c0d1e2f` |
| `actor.instance_id` | `runner-pod-abc` | `inst-3a4b5c6d` |
| `actor.origin` | `infra-bench` | `origin-7e8f9a0b` |
| `session_id` | `01KKS50SMA3ATY...` | `sess-c1d2e3f4` |
| `operation_id` | `op-deploy-fix` | `op-5a6b7c8d` |
| `trace_id` | `abc123...` | `trace-9e0f1a2b` |
| `span_id` | `span-xyz` | `span-3c4d5e6f` |
| `scope_dimensions` values | `{cluster: prod-us-1}` | `{cluster: dim-7a8b9c0d}` |
| Evidence file paths | `/home/user/...` | stripped |

### What Stays Unchanged

| Field | Why it's safe |
|-------|---------------|
| `tool` | Generic (kubectl, terraform, helm) |
| `operation` | Generic (apply, delete, patch) |
| `operation_class` | Normalized (mutate, destroy, read) |
| `scope_class` | Normalized (production, staging, development, unknown) |
| `resource_count` | Numeric, no PII |
| `ResourceIdentity[].kind` | Generic (Deployment, Service, Pod) |
| `ResourceIdentity[].api_version` | Generic (apps/v1, v1) |
| `artifact_digest` | Already a hash |
| `intent_digest` | Already a hash |
| `resource_shape_hash` | Already a hash |
| `risk_inputs`, `effective_risk` | Risk assessment, no PII |
| `risk_tags` | Generic tags (k8s.run_as_root) |
| All timestamps | Temporal pattern matters for signals |
| Entry ordering | Hash chain structure matters |
| `type` (prescribe, report, signal) | Entry type |
| `verdict` (success, failure, declined) | Outcome |
| `exit_code` | Numeric |
| `spec_version`, `scoring_version` | Version metadata |
| `canon_version` | Adapter version |
| Signal names and counts | Behavioral data |
| Score, band, confidence | Assessment data |

### What Gets Stripped Entirely

| Field | Why |
|-------|-----|
| Raw artifact content | May contain secrets, config values, API keys |
| `payload` in signal entries | May contain diagnostic text with resource names |
| Evidence `signature` fields | Tied to signing key identity |
| `previous_hash` chain | Rebuild from anonymized entries |

## Bundle Format

```
evidence-bundle/
  manifest.json          # bundle metadata
  evidence.jsonl         # anonymized evidence entries
  scorecard.json         # scorecard (if --include-scorecard)
  metadata.json          # version info, entry count, signal summary
```

### manifest.json

```json
{
  "bundle_version": "1.0",
  "anonymized": true,
  "salt_hint": "random-per-export",
  "entry_count": 12,
  "evidra_version": "0.4.9",
  "spec_version": "v1.1.0",
  "scoring_profile_id": "default.v1.1.0",
  "exported_at": "2026-03-15T20:00:00Z",
  "original_session_count": 1
}
```

### metadata.json

```json
{
  "total_operations": 5,
  "signal_summary": {
    "protocol_violation": 1,
    "retry_loop": 0,
    "artifact_drift": 0
  },
  "actors": ["actor-9c0d1e2f"],
  "tools": ["kubectl"],
  "scope_classes": ["development"],
  "time_range": {
    "first": "2026-03-15T18:00:00Z",
    "last": "2026-03-15T18:05:00Z"
  }
}
```

## Implementation

### In evidra core (pkg/export/)

```go
type ExportOptions struct {
    EvidenceDir     string
    RunDir          string   // optional: include run artifacts
    Anonymize       bool
    IncludeScorecard bool
    OutputPath      string
}

type Anonymizer struct {
    salt []byte
    cache map[string]string  // original → anonymized (deterministic within export)
}

func (a *Anonymizer) AnonymizeEntry(entry evidence.EvidenceEntry) evidence.EvidenceEntry
func (a *Anonymizer) AnonymizeResourceID(id canon.ResourceID) canon.ResourceID
func (a *Anonymizer) Hash(original string) string
```

### In evidra CLI (cmd/evidra/export.go)

```
evidra export [flags]

Flags:
  --evidence-dir    evidence directory to export
  --run-dir         optional run artifact directory to include
  --anonymize       anonymize identifiers (default: true)
  --include-scorecard  generate and include scorecard
  --output          output file path (default: evidence-export.tar.gz)
  --format          output format: tar.gz (default) or directory
```

### In infra-bench (optional integration)

```bash
# After a bench run, export anonymized bundle for sharing
infra-bench export --run-dir runs/bench/20260315-195927/broken-deployment_sonnet_r1
```

This would call the evidra export library internally.

## What Users Can Do With It

### Report an issue

```bash
# Something went wrong with scoring
evidra export --evidence-dir ~/.evidra/evidence --anonymize --output issue.tar.gz
# Attach issue.tar.gz to GitHub issue
```

### Share benchmark results

```bash
# After running infra-bench
evidra export \
  --evidence-dir runs/e2e/broken-deployment-sonnet/evidence \
  --anonymize \
  --include-scorecard \
  --output benchmark-broken-deployment.tar.gz
```

### Evidra team analysis

```bash
# Unpack and score
tar xzf issue.tar.gz
evidra scorecard --evidence-dir evidence-bundle/ --ttl 1s
evidra explain --evidence-dir evidence-bundle/ --ttl 1s
# All signals preserved, no PII exposed
```

## What This Enables For Evidra

1. **Bug reports with evidence** — users share signal-preserving bundles instead of screenshots
2. **Community benchmarks** — publish anonymized benchmark results that others can verify
3. **Dataset collection** — accumulate anonymized evidence chains for signal detector research
4. **Partner onboarding** — "send us your first run" becomes safe and easy

## Security Considerations

- Salt is random per export, not derived from user identity
- Hash length (8 hex = 32 bits) is intentionally short — enough for uniqueness within
  one chain, not enough to brute-force original values
- Signatures are stripped — the bundle is not cryptographically verifiable after
  anonymization (this is intentional; the original evidence is the source of truth)
- Raw artifacts are never included — only digests
- Timestamps are preserved — if timing correlation is a concern, a future
  `--jitter-timestamps` flag could add random offsets while preserving relative ordering

## Scope

### In scope
- Evidence JSONL anonymization
- Bundle creation with manifest
- Scorecard inclusion
- CLI command in evidra core

### Out of scope (future)
- Selective anonymization (keep some fields, redact others)
- Timestamp jittering
- Multi-session export
- Streaming export (for large evidence chains)
- Web UI for export
- Automatic export on issue creation
