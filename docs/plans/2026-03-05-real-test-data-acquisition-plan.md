# Real Test Data Acquisition Plan for Evidra (Draft)

**Status:** Draft for discussion and refinement  
**Date:** 2026-03-05  
**Baseline notes:** [real_test_data_for_evidra_ideas.md](./real_test_data_for_evidra_ideas.md)

## 1) Objective

Build a repeatable, legally safe, and high-signal process to source **real-world test datasets** for Evidra benchmark testing across:

- `local-mcp` (default)
- `local-rest` (opt-in)
- `hosted-mcp` (opt-in, network-gated)
- `hosted-rest` (opt-in, network-gated)

Current rule: hosted modes remain disabled by default and run only with `EVIDRA_ENABLE_NETWORK_TESTS=1`.

## 2) Scope and Constraints

In scope:

- Infrastructure/mutate operations only (`kubectl`, `terraform`, related policy/risk signals).
- Reusable datasets for inspector e2e, REST e2e, and regression checks.
- Source provenance and license traceability per case.

Out of scope:

- Production customer data ingestion.
- PII-bearing logs, secrets, or internal incident payloads.
- Agent chat transcripts as primary corpus.

## 3) Strategy Options

### Option A: Use only current `evidra-benchmark` fixtures

Pros:
- Fastest.
- Zero migration work.

Cons:
- Limited realism/diversity.
- Low confidence against real-world edge cases.

### Option B: Import only from external OSS corpora

Pros:
- High realism and breadth.
- Good long-term expansion path.

Cons:
- High normalization cost.
- License/provenance work starts from scratch.

### Option C (Recommended): Hybrid bootstrap

Phase 1 bootstrap from `../evidra-mcp` curated assets, then incrementally expand from external OSS sources.

Why recommended:
- Fast near-term value (existing curated/compatible assets).
- Better provenance baseline (`SOURCE.md`, manifests, corpus metadata already exist).
- Lower migration risk while still unlocking broader real-world coverage.

## 4) Data Sources: Where to Get Real Datasets

### 4.1 Immediate Source (highest priority): local legacy repo

Use `../evidra-mcp` as seed corpus:

- `../evidra-mcp/tests/golden_real/*`  
  Real-derived allow/deny fixtures by rule + `SOURCE.md`.
- `../evidra-mcp/tests/golden_real/manifest.json`  
  Canonical case index with expected decisions/rule IDs.
- `../evidra-mcp/tests/corpus/*.json`  
  Structured input/expect cases already used by policy and inspector layers.
- `../evidra-mcp/tests/e2e/fixtures/*`  
  Kubernetes/Terraform artifact fixtures.
- `../evidra-mcp/examples/*.json` and `../evidra-mcp/examples/demo/*.json`  
  Additional realistic invocation examples.
- `../evidra-mcp/tests/corpus/sources.json`  
  Candidate-source catalog per rule.

### 4.2 External OSS sources for phase 2+

Terraform / cloud misconfig:

- Checkov Terraform tests: `bridgecrewio/checkov/tests/terraform`
- Terraform AWS provider examples: `hashicorp/terraform-provider-aws/examples`
- tfsec docs examples (for deny/allow pattern pairs)

Kubernetes misconfig / workload risk:

- Kubescape examples
- OWASP Top 10 Kubernetes examples
- Falco examples (container/runtime misbehavior patterns)
- Kubernetes official security docs examples (Pod Security Standards etc.)

Operational/destructive behavior:

- Public postmortems and incident write-ups converted into synthetic reproducible fixtures (no raw proprietary data).

### 4.3 Source acceptance policy

A source is eligible only if all are true:

1. Clear license and reuse terms recorded.
2. No secrets/credentials/tokens.
3. Reproducible fetch path (URL + commit/tag/snapshot date).
4. Converts to deterministic local tests.

## 5) Proposed Dataset Layout in `evidra-benchmark`

Create a dedicated dataset area while keeping inspector runner compatibility:

```text
tests/realdata/
  README.md
  catalog.json                    # index of all accepted cases
  sources/
    <source-id>.md                # provenance + license notes
  cases/
    <case-id>/
      raw/                        # fetched snapshot or minimized raw extract
      normalized/                 # canonical Evidra payload/artifact
      inspector_case.json         # scenario for tests/inspector runner
      rest_request.json           # request body for local-rest/hosted-rest
      expected.json               # expected outcomes (risk tags, decision, etc.)
      SOURCE.md                   # final per-case attribution
  scripts/
    import_from_evidra_mcp.sh
    validate_realdata.sh
```

Integration targets:

- Inspector cases consumed directly or generated into `tests/inspector/cases/`.
- Artifact fixtures linked from `tests/inspector/fixtures/` as needed.
- `local-rest` uses same scenario intent as `local-mcp` to ensure parity.

## 6) Intake Workflow: How We Get and Convert Data

### 6.1 Intake steps (per source batch)

1. **Discover candidates** (from `evidra-mcp` and OSS repos).
2. **Record provenance** (URL, commit/tag, retrieval date, license).
3. **Minimize and normalize** into Evidra-relevant artifact/payload.
4. **Create paired cases** where possible (`deny_real_1`, `allow_real_1`).
5. **Generate test scenario files** for inspector and REST.
6. **Run validation gates** (`local-mcp` + `local-rest` mandatory).
7. **Optionally run hosted modes** only with network opt-in flags.
8. **Update catalog and changelog** (what was added, from where, why).

### 6.2 Mandatory metadata per case

- `case_id`
- `rule_targets` / `risk_tags_expected`
- `source_url`
- `source_commit_or_tag`
- `source_license`
- `retrieved_at` (UTC date)
- `transformation_notes`
- `reviewer`

## 7) Initial Backlog (Suggested First 20 Cases)

Start with cases already proven in `evidra-mcp`:

- `k8s.privileged_container` (allow+deny)
- `k8s.hostpath_mount` (allow+deny)
- `k8s.host_namespace_escape` (allow+deny)
- `k8s.run_as_root` (allow+deny)
- `k8s.protected_namespace` (allow+deny)
- `k8s.dangerous_capabilities` (allow+deny)
- `terraform.s3_public_access` (allow+deny)
- `terraform.sg_open_world` (allow+deny)
- `terraform.iam_wildcard_policy` (allow+deny)
- `ops.mass_delete` (allow+deny)

Then expand with external examples for:

- `blast_radius` style destructive plan patterns
- RBAC escalation patterns
- scanner SARIF high/critical findings (`trivy`, `kubescape`, `checkov`)

## 8) Verification Gates

Required on every dataset PR:

1. Schema/provenance validation (`tests/realdata/scripts/validate_realdata.sh`).
2. Determinism check (same input -> same expected outputs).
3. Inspector tests:
   - `EVIDRA_TEST_MODE=local-mcp`
   - `EVIDRA_TEST_MODE=local-rest` (with local API URL)
4. Hosted tests are non-blocking and opt-in:
   - `EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-mcp ...`
   - `EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-rest ...`

## 9) Rollout Plan

### Phase 0 (1 day): inventory + acceptance policy

- Inventory usable assets from `../evidra-mcp`.
- Define/approve metadata schema and source acceptance rules.

### Phase 1 (2-3 days): bootstrap import from `evidra-mcp`

- Import first 20 curated cases.
- Wire to inspector `local-mcp` and `local-rest`.
- Keep hosted modes disabled by default.

### Phase 2 (3-5 days): external OSS expansion

- Add 10-20 external-source cases with verified licenses/provenance.
- Prioritize destructive Terraform/Kubernetes misconfig patterns.

### Phase 3 (ongoing): governance + refresh cadence

- Monthly source refresh and stale-link checks.
- Rule coverage dashboard: case count by rule/risk level/source.

## 10) Risks and Mitigations

- **License ambiguity**  
  Mitigation: fail intake if license is unknown/unacceptable.

- **Fixture drift over time**  
  Mitigation: pin source commit/tag and keep normalized local copies.

- **Transport behavior mismatch (`local-mcp` vs `local-rest`)**  
  Mitigation: same scenario IDs across both modes; parity assertion in CI.

- **Network flakiness in hosted tests**  
  Mitigation: keep hosted tests opt-in/non-blocking by default.

## 11) Discussion Items / Open Questions

1. Do we keep canonical case files in `tests/realdata/cases/` and generate `tests/inspector/cases/`, or edit both directly?
2. Which licenses are explicitly allowed for imported fixtures (Apache-2.0, MIT, BSD, MPL-2.0)?
3. Should hosted mode run on nightly CI (opt-in) or manual-only at first?
4. Do we require deny/allow pairs for every rule, or allow single-sided real cases when no safe counterpart exists?

## 12) Immediate Next Step

Approve Option C and Phase 0-1 scope, then implement:

- `tests/realdata/` skeleton
- import script from `../evidra-mcp`
- first 20 curated real cases with provenance metadata
