# Benchmark Dataset P0 Bootstrap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bootstrap `tests/benchmark` from empty scaffold to a validated P0 **limited contract-baseline dataset** (metadata + labels + 10 real-derived-first cases), then use it to verify signal theory and domain model contracts before full-source ingestion.

**Architecture:** Keep this phase dataset-first and deterministic: import a small curated seed set from existing fixtures (`tests/*`) and `../evidra-mcp/tests/golden_real`, represent each case with `expected.json` + local artifact copy, and enforce quality using one shell validator (`validate-dataset.sh`) plus existing source-composition checks.

**Tech Stack:** Go 1.23+, Bash, `jq`, Markdown, JSON/YAML, GitHub Actions.

---

## Execution Order (No Chicken-Egg)

This plan explicitly resolves the chicken-egg dependency:

1. Build a **small labeled limited dataset** first (contract-baseline only).
2. Use it to validate and fix signal/domain contracts (fast feedback, deterministic).
3. Freeze contract expectations in tests/validators.
4. Expand ingestion from all planned sources after contracts are stable.
5. Run expensive agent/cluster experiments on top of stable contracts.

Why this works:
- Signal theory validation does NOT require full dataset coverage.
- Full-source ingestion without stable contracts causes mass rework and churn in expectations.

## Limited Dataset Labeling Contract (Mandatory)

Until full benchmark coverage is ready, all P0 artifacts must be explicitly labeled as limited.

Required labels in `tests/benchmark/dataset.json`:
- `dataset_label: "limited-contract-baseline"`
- `dataset_scope: "limited"`
- `dataset_track: "contract-validation"`
- `dataset_not_for` includes:
  - `leaderboard`
  - `public-comparison`
  - `final-benchmark-score`

Required labels in `tests/benchmark/benchmark.yaml`:
- `profile: limited-contract-baseline`
- `maturity: pre-benchmark`
- `labels: [limited, contract-validation, non-comprehensive]`

Required label in each `tests/benchmark/cases/<case-id>/expected.json`:
- `dataset_label: "limited-contract-baseline"`

Validation rule:
- `validate-dataset.sh` MUST fail if any required limited label is missing or mismatched.

## Tracking

Status legend: `TODO` | `IN_PROGRESS` | `DONE` | `BLOCKED`

| ID | Milestone | Status | Owner | Verification Command | Evidence (commit/PR/log) |
|---|---|---|---|---|---|
| M0 | P0 scope frozen + tracker created | DONE | @agent | `test -f docs/plans/2026-03-06-benchmark-dataset-p0-bootstrap-implementation-plan.md` | plan file created (2026-03-06 local) |
| M1 | Dataset metadata/schema + limited labels created | DONE | @agent | `jq -e '.dataset_label==\"limited-contract-baseline\" and .dataset_scope==\"limited\" and .dataset_track==\"contract-validation\"' tests/benchmark/dataset.json` | `dataset.json` + `benchmark.yaml` labels validated |
| M2 | 10 seed cases imported (real-derived-first) and labeled | DONE | @agent | `find tests/benchmark/cases -name expected.json | wc -l` | 10 cases present; case labels validated via jq loop |
| M3 | Source manifests linked for all source refs | DONE | @agent | `bash tests/benchmark/scripts/validate-source-composition.sh` | PASS: real-derived=100%, custom-only=0% |
| M4 | `validate-dataset.sh` enforces hard checks + limited label contract | DONE | @agent | `bash tests/benchmark/scripts/validate-dataset.sh` | PASS: cases=10 |
| M5 | Makefile + CI include dataset validation | DONE | @agent | `make benchmark-validate` | target added + CI step in `.github/workflows/ci.yml` |
| M6 | Docs cross-links + limited-dataset warnings updated | DONE | @agent | `bash scripts/check-doc-commands.sh` | doc checks passed |
| M7 | Final verification pass | DONE | @agent | `go test ./... -count=1` | all packages passed |

Tracking update rule (mandatory during execution):
- After each milestone, set `Status`, fill `Evidence`, and include command output summary in commit message body.
- If blocked for >30 minutes, set `BLOCKED` with blocker reason and next unblocking action.

## Source Docs for This Plan

Normative priority for execution in this plan:
1. `docs/plans/2026-03-05-benchmark-dataset-proposal.md` (dataset model, source policy, P0 rollout)
2. `tests/benchmark/scripts/validate-source-composition.sh` (already implemented release gates)
3. `docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md` (P0 implementation order)
4. `docs/system-design/EVIDRA_BENCHMARK_CLI.md` (consumer expectations for dataset discovery/validation)

Task-to-doc mapping:

| Plan Task | Primary Source Docs | Why it maps |
|---|---|---|
| Task 1 (dataset metadata/schema + labels) | `docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md` §12 P0 (`expected.json` + `dataset.json` first), `docs/plans/2026-03-05-benchmark-dataset-proposal.md` §4 (manifest model) | Defines first deliverables and required metadata/manifest shape |
| Task 2 (source manifests) | `docs/plans/2026-03-05-benchmark-dataset-proposal.md` §7 + §9 (source catalog + mandatory provenance schema) | Defines required source fields and provenance policy |
| Task 3 (10 seed cases) | `docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md` §12 P0 (“first 10 cases”), `docs/plans/2026-03-05-benchmark-dataset-proposal.md` §7.1 seed from `../evidra-mcp` | Defines seed strategy and minimum bootstrap volume |
| Task 4 (validate-dataset.sh) | `docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md` §12 P0 (`validate-dataset.sh`), `docs/plans/2026-03-05-benchmark-dataset-proposal.md` §10 Validation | Defines required checks and pass/fail behavior |
| Task 5 (Makefile + CI gate) | `docs/plans/2026-03-05-benchmark-dataset-proposal.md` §10 CI integration | Requires dataset checks in CI path |
| Task 6 (docs cross-links) | `docs/system-design/EVIDRA_BENCHMARK_CLI.md` §2, §7 | Keeps user-facing CLI docs aligned with actual dataset/validation workflow |
| Task 7 (final verify + tracker evidence) | This plan `Tracking` section + `docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md` §12 P0 completion intent | Ensures auditable completion with command evidence |

Explicitly treated as context-only (not normative for this P0 plan):
- `docs/research/FAULT_INJECTION_RUNBOOK.md`
- `docs/research/EVIDRA_EXPERIMENT_REPOSITORY_STRUCTURE.md`
- `docs/research/EVIDRA_EXPERIMENT_DESIGN_V1.md`

## Seed Case Set (P0)

Use these ten cases first (5 FAIL + 5 PASS pairs) to maximize signal coverage with low curation effort:

1. `k8s-privileged-container-fail`
2. `k8s-privileged-container-pass`
3. `k8s-hostpath-mount-fail`
4. `k8s-hostpath-mount-pass`
5. `k8s-run-as-root-fail`
6. `k8s-run-as-root-pass`
7. `tf-s3-public-access-fail`
8. `tf-s3-public-access-pass`
9. `tf-iam-wildcard-policy-fail`
10. `tf-iam-wildcard-policy-pass`

Primary import source: `../evidra-mcp/tests/golden_real/*`  
Fallback source (if path unavailable): current repo fixtures in `tests/inspector/fixtures`, `tests/testdata`, and `tests/golden`.

## Task 1: Create Dataset Skeleton and Metadata

**Files:**
- Create: `tests/benchmark/dataset.json`
- Create: `tests/benchmark/benchmark.yaml`
- Create: `tests/benchmark/schema/expected.schema.json`
- Create: `tests/benchmark/schema/dataset.schema.json`
- Modify: `tests/benchmark/sources/TEMPLATE.md`

**Step 1: Write the failing test**

Run: `test -f tests/benchmark/dataset.json`  
Expected: non-zero exit (file does not exist yet).

**Step 2: Write minimal metadata and schema files**

Add `dataset.json` with:
- `dataset_version`
- `schema_version`
- `evidra_version_processed`
- `generated_at`
- `case_count`
- `dataset_label: limited-contract-baseline`
- `dataset_scope: limited`
- `dataset_track: contract-validation`
- `dataset_not_for: [leaderboard, public-comparison, final-benchmark-score]`

Add `benchmark.yaml` with:
- dataset id/version
- `profile: limited-contract-baseline`
- `maturity: pre-benchmark`
- `labels: [limited, contract-validation, non-comprehensive]`
- list placeholder for case ids (to be filled in Task 3)

Add JSON schema files with required keys only (P0 minimal).

**Step 3: Verify metadata parses**

Run: `jq -e '.dataset_version and .schema_version and .evidra_version_processed and (.dataset_label=="limited-contract-baseline") and (.dataset_scope=="limited") and (.dataset_track=="contract-validation") and (.dataset_not_for | index("leaderboard"))' tests/benchmark/dataset.json`  
Expected: exit `0`.

Run: `rg -n "profile:\\s+limited-contract-baseline|maturity:\\s+pre-benchmark|labels:\\s*\\[limited, contract-validation, non-comprehensive\\]" tests/benchmark/benchmark.yaml`  
Expected: exit `0`.

**Step 4: Commit**

```bash
git add tests/benchmark/dataset.json tests/benchmark/benchmark.yaml tests/benchmark/schema tests/benchmark/sources/TEMPLATE.md
git commit -m "feat(benchmark): add dataset metadata and p0 schema skeleton"
```

## Task 2: Add Source Provenance Manifests

**Files:**
- Create: `tests/benchmark/sources/k8s-pod-security-standards.md`
- Create: `tests/benchmark/sources/k8s-linux-kernel-security.md`
- Create: `tests/benchmark/sources/checkov-terraform.md`
- Create: `tests/benchmark/sources/kubescape-regolibrary.md`
- Create: `tests/benchmark/sources/evidra-mcp-golden-real.md`

**Step 1: Write the failing test**

Run: `bash tests/benchmark/scripts/validate-source-composition.sh`  
Expected: skip or fail (no valid cases yet).

**Step 2: Write source manifests**

Each source file must include:
- upstream URL
- license
- retrieval date
- pinned revision/tag/commit when available
- composition class (`real-derived` or `custom-only`)

**Step 3: Validate source files exist**

Run: `ls tests/benchmark/sources/*.md | wc -l`  
Expected: at least `6` (template + 5 real source manifests).

**Step 4: Commit**

```bash
git add tests/benchmark/sources
git commit -m "docs(benchmark): add p0 source provenance manifests"
```

## Task 3: Import 10 Seed Cases (Pair-Based)

**Files:**
- Create: `tests/benchmark/cases/<case-id>/README.md` (10 dirs)
- Create: `tests/benchmark/cases/<case-id>/expected.json` (10 files)
- Create: `tests/benchmark/cases/<case-id>/artifacts/*` (copied local artifacts)
- Modify: `tests/benchmark/benchmark.yaml`
- Modify: `tests/benchmark/dataset.json`

**Step 1: Write the failing test**

Run: `find tests/benchmark/cases -name expected.json | wc -l`  
Expected: `0`.

**Step 2: Copy and normalize case artifacts**

For each case:
- copy one artifact from `../evidra-mcp/tests/golden_real/<rule>/<allow|deny>_real_1.json` into case `artifacts/`
- add concise scenario `README.md`
- add `expected.json` with required fields:
  - `case_id`
  - `dataset_label` (`limited-contract-baseline`)
  - `case_kind` (`artifact` or `pair`)
  - `category`
  - `difficulty`
  - `ground_truth_pattern`
  - `artifact_ref`
  - `source_refs`
  - `risk_level`
  - `risk_details_expected` (or empty array for PASS)

**Step 3: Update dataset counters**

Set:
- `dataset.json.case_count = 10`
- `benchmark.yaml.cases` contains 10 case ids.

**Step 4: Verify import**

Run:
- `find tests/benchmark/cases -name expected.json | wc -l`
- `jq -r '.case_count' tests/benchmark/dataset.json`
- `for f in $(find tests/benchmark/cases -name expected.json); do jq -e '.dataset_label=="limited-contract-baseline"' "$f" >/dev/null; done`

Expected: first two commands return `10`, and the label-check loop exits `0`.

**Step 5: Commit**

```bash
git add tests/benchmark/cases tests/benchmark/benchmark.yaml tests/benchmark/dataset.json
git commit -m "feat(benchmark): import p0 ten-case seed set"
```

## Task 4: Implement Hard Validator (`validate-dataset.sh`)

**Files:**
- Create: `tests/benchmark/scripts/validate-dataset.sh`
- Modify: `tests/benchmark/scripts/validate-source-composition.sh` (only if shared logic extraction is needed)

**Step 1: Write the failing test**

Run: `bash tests/benchmark/scripts/validate-dataset.sh`  
Expected: non-zero exit (script missing).

**Step 2: Implement minimal validator checks**

Checks required for P0:
- `dataset.json` exists and required fields are non-empty
- limited dataset labels in `dataset.json` match exact required values
- at least 10 cases exist
- every `expected.json` has required keys and valid JSON
- every `expected.json` has `dataset_label == "limited-contract-baseline"`
- `case_id` unique and matches directory name
- each `artifact_ref` resolves to an existing file
- each `source_refs[*].source_id` resolves to `tests/benchmark/sources/<id>.md`
- `benchmark.yaml` contains `profile: limited-contract-baseline` and `maturity: pre-benchmark`
- source composition ratio gate:
  - `real-derived` cases >= 80%
  - `custom-only` cases <= 20%

**Step 3: Run validator**

Run: `bash tests/benchmark/scripts/validate-dataset.sh`  
Expected: `PASS`.

**Step 4: Commit**

```bash
git add tests/benchmark/scripts/validate-dataset.sh tests/benchmark/scripts/validate-source-composition.sh
git commit -m "test(benchmark): add p0 dataset validator with hard gates"
```

## Task 5: Wire Validation into Makefile and CI

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`

**Step 1: Write the failing test**

Run: `make benchmark-validate`  
Expected: fail (`No rule to make target`).

**Step 2: Add build target and CI step**

In `Makefile` add:
- `benchmark-validate:`
  - `bash tests/benchmark/scripts/validate-dataset.sh`

In CI add step after docs command checks:
- run `make benchmark-validate`

**Step 3: Verify**

Run:
- `make benchmark-validate`
- `bash tests/benchmark/scripts/validate-dataset.sh`

Expected: both pass.

**Step 4: Commit**

```bash
git add Makefile .github/workflows/ci.yml
git commit -m "ci(benchmark): enforce dataset validation in local and ci workflows"
```

## Task 6: Update Documentation and Cross-Links

**Files:**
- Modify: `README.md`
- Modify: `docs/system-design/EVIDRA_BENCHMARK_CLI.md`
- Modify: `docs/plans/2026-03-05-benchmark-dataset-proposal.md`

**Step 1: Write the failing test**

Run: `rg -n "benchmark-validate|tests/benchmark/dataset.json|validate-dataset.sh" README.md docs/system-design/EVIDRA_BENCHMARK_CLI.md docs/plans/2026-03-05-benchmark-dataset-proposal.md`  
Expected: missing matches.

**Step 2: Add docs updates**

Add:
- README section: where dataset lives and how to run `make benchmark-validate`
- README warning: current P0 dataset is `limited-contract-baseline`, not for leaderboard/public comparison
- Benchmark CLI doc: note current dataset metadata files and validation command
- Dataset proposal: implementation status block with link to this plan

**Step 3: Verify docs commands**

Run: `bash scripts/check-doc-commands.sh`  
Expected: pass.

**Step 4: Commit**

```bash
git add README.md docs/system-design/EVIDRA_BENCHMARK_CLI.md docs/plans/2026-03-05-benchmark-dataset-proposal.md
git commit -m "docs(benchmark): add p0 dataset usage and cross-links"
```

## Task 7: Final Verification and Handoff

**Files:**
- Modify: `docs/plans/2026-03-06-benchmark-dataset-p0-bootstrap-implementation-plan.md` (update tracker statuses/evidence)

**Step 1: Run verification suite**

Run:
- `bash tests/benchmark/scripts/validate-dataset.sh`
- `bash tests/benchmark/scripts/validate-source-composition.sh`
- `make benchmark-validate`
- `go test ./... -count=1`

Expected: all pass.

**Step 2: Update tracker statuses**

Set milestones `M0..M7` to `DONE` and fill evidence fields with commit hashes.

**Step 3: Commit tracker finalization**

```bash
git add docs/plans/2026-03-06-benchmark-dataset-p0-bootstrap-implementation-plan.md
git commit -m "docs(plan): finalize p0 dataset bootstrap tracker with evidence"
```

## Exit Criteria (Definition of Done)

1. `tests/benchmark` contains dataset metadata, schemas, and 10 curated cases.
2. Limited-dataset label contract is present and validated in `dataset.json`, `benchmark.yaml`, and every case `expected.json`.
3. The limited dataset is explicitly documented as contract-validation only (not leaderboard-ready).
4. Every case has source provenance and passes source composition ratio gate.
5. `make benchmark-validate` passes locally and in CI.
6. README/system design docs link to the implemented dataset validation workflow.
7. Tracker table in this plan has completed statuses with evidence pointers.

## Out of Scope (This Plan)

1. Scenario engine (`tests/benchmark/scenarios/*`) and multi-step signal replay.
2. `evidra benchmark run` execution engine internals.
3. External dataset repo split (`evidra-dataset`) and leaderboard publication.
