#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

DATASET_JSON="tests/benchmark/dataset.json"
BENCHMARK_YAML="tests/benchmark/benchmark.yaml"
CASES_DIR="tests/benchmark/cases"
SOURCES_DIR="tests/benchmark/sources"

fail() {
  echo "dataset-validate: FAIL $*" >&2
  exit 1
}

warn_count=0
warn() {
  echo "dataset-validate: WARN $*" >&2
  warn_count=$((warn_count + 1))
}

has_pattern() {
  local pattern="$1"
  local file="$2"
  if command -v rg >/dev/null 2>&1; then
    rg -q -e "$pattern" "$file"
    return
  fi
  grep -Eq "$pattern" "$file"
}

if ! command -v jq >/dev/null 2>&1; then
  fail "jq is required"
fi

[[ -f "$DATASET_JSON" ]] || fail "missing $DATASET_JSON"
[[ -f "$BENCHMARK_YAML" ]] || fail "missing $BENCHMARK_YAML"
[[ -d "$CASES_DIR" ]] || fail "missing $CASES_DIR"
[[ -d "$SOURCES_DIR" ]] || fail "missing $SOURCES_DIR"

bash tests/benchmark/scripts/validate-provenance.sh >/dev/null || fail "provenance validation failed"
bash tests/benchmark/scripts/validate-case-metadata.sh >/dev/null || fail "case metadata validation failed"

# Dataset metadata and limited-label contract.
jq -e '
  .dataset_version and
  .schema_version and
  .evidra_version_processed and
  .generated_at and
  (.case_count | type=="number") and
  (.case_count >= 10) and
  (.dataset_label == "limited-contract-baseline") and
  (.dataset_scope == "limited") and
  (.dataset_track == "contract-validation") and
  (.dataset_not_for | type=="array") and
  (.dataset_not_for | index("leaderboard")) and
  (.dataset_not_for | index("public-comparison")) and
  (.dataset_not_for | index("final-benchmark-score")) and
  (.scenario_axes.scenario_class == ["safe_routine", "normal_mutate", "high_risk_ambiguous", "decline_worthy"]) and
  (.scenario_axes.operation_class == ["inspect_read", "mutate_change", "deploy_rollout"]) and
  (.scenario_axes.risk_level == ["low", "medium", "high", "critical"]) and
  (.scenario_axes.environment_class == ["sandbox", "staging", "prod_like"])
' "$DATASET_JSON" >/dev/null || fail "dataset.json missing required fields or limited label contract"

dataset_processed_version="$(jq -r '.evidra_version_processed // empty' "$DATASET_JSON")"
[[ -n "$dataset_processed_version" ]] || fail "dataset.json missing evidra_version_processed"

# Minimal benchmark.yaml label contract (without yq dependency).
if ! has_pattern '^[[:space:]]*profile:[[:space:]]+limited-contract-baseline[[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing profile: limited-contract-baseline"
fi
if ! has_pattern '^[[:space:]]*maturity:[[:space:]]+pre-benchmark[[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing maturity: pre-benchmark"
fi
if ! has_pattern '^[[:space:]]*scenario_class:[[:space:]]+\[safe_routine, normal_mutate, high_risk_ambiguous, decline_worthy\][[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing scenario_class axis"
fi
if ! has_pattern '^[[:space:]]*operation_class:[[:space:]]+\[inspect_read, mutate_change, deploy_rollout\][[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing operation_class axis"
fi
if ! has_pattern '^[[:space:]]*risk_level:[[:space:]]+\[low, medium, high, critical\][[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing risk_level axis"
fi
if ! has_pattern '^[[:space:]]*environment_class:[[:space:]]+\[sandbox, staging, prod_like\][[:space:]]*$' "$BENCHMARK_YAML"; then
  fail "benchmark.yaml missing environment_class axis"
fi

expected_files=()
while IFS= read -r file; do
  expected_files+=("$file")
done < <(find "$CASES_DIR" -type f -name "expected.json" | sort)
if [[ ${#expected_files[@]} -lt 10 ]]; then
  fail "expected >=10 cases, found ${#expected_files[@]}"
fi

seen_case_ids=""

for file in "${expected_files[@]}"; do
  jq -e '
    .case_id and
    (.case_id | type=="string") and
    .dataset_label and
    .case_kind and
    .category and
    .difficulty and
    .scenario_class and
    .operation_class and
    .environment_class and
    .ground_truth_pattern and
    .artifact_ref and
    .source_refs and
    (.source_refs | type=="array") and
    (.source_refs | length > 0) and
    .risk_level and
    .risk_details_expected and
    (.risk_details_expected | type=="array")
  ' "$file" >/dev/null || fail "$file missing required expected.json fields"

  jq -e '.dataset_label == "limited-contract-baseline"' "$file" >/dev/null \
    || fail "$file missing dataset_label=limited-contract-baseline"

  case_id="$(jq -r '.case_id' "$file")"
  dir_name="$(basename "$(dirname "$file")")"
  [[ "$case_id" == "$dir_name" ]] || fail "$file case_id ($case_id) must match directory ($dir_name)"

  if printf '%s\n' "$seen_case_ids" | grep -Fxq "$case_id"; then
    fail "duplicate case_id detected: $case_id"
  fi
  seen_case_ids="${seen_case_ids}"$'\n'"${case_id}"

  artifact_ref="$(jq -r '.artifact_ref' "$file")"
  [[ "$artifact_ref" == ../../corpus/* ]] || fail "$file artifact_ref must point into ../../corpus/, got: $artifact_ref"
  artifact_path="$(dirname "$file")/$artifact_ref"
  [[ -f "$artifact_path" ]] || fail "$file artifact_ref does not resolve: $artifact_ref"

  contract_path="$(dirname "$file")/golden/contract.json"
  [[ -f "$contract_path" ]] || fail "$file missing golden contract: $(dirname "$file")/golden/contract.json"
  jq -e '
    .case_id and
    .risk_level and
    .risk_details and
    (.risk_details | type=="array") and
    .artifact_digest and
    .evidra_version and
    .processing and
    .processing.dataset_evidra_version and
    .processing.processed_at and
    .processing.tool and
    .processing.operation
  ' "$contract_path" >/dev/null || fail "$contract_path missing required contract fields"

  expected_processing_version="$(jq -r '.processing.evidra_version // empty' "$file")"
  contract_dataset_version="$(jq -r '.processing.dataset_evidra_version // empty' "$contract_path")"
  if [[ -n "$expected_processing_version" && "$expected_processing_version" != "$dataset_processed_version" ]]; then
    warn "$file processing.evidra_version=$expected_processing_version differs from dataset.evidra_version_processed=$dataset_processed_version"
  fi
  if [[ -n "$contract_dataset_version" && "$contract_dataset_version" != "$dataset_processed_version" ]]; then
    warn "$contract_path processing.dataset_evidra_version=$contract_dataset_version differs from dataset.evidra_version_processed=$dataset_processed_version"
  fi
  if [[ -n "$expected_processing_version" && -n "$contract_dataset_version" && "$expected_processing_version" != "$contract_dataset_version" ]]; then
    warn "$file processing.evidra_version=$expected_processing_version differs from $contract_path processing.dataset_evidra_version=$contract_dataset_version"
  fi

  expected_digest="$(jq -r '.artifact_digest // empty' "$file")"
  contract_digest="$(jq -r '.artifact_digest // empty' "$contract_path")"
  if [[ -n "$expected_digest" && "$expected_digest" != "TODO" && "$expected_digest" != "$contract_digest" ]]; then
    fail "$contract_path artifact_digest mismatch (expected.json=$expected_digest contract=$contract_digest)"
  fi

  source_ids=()
  while IFS= read -r source_id; do
    source_ids+=("$source_id")
  done < <(jq -r '.source_refs[] | (.source_id // .id // .source // empty)' "$file")
  [[ ${#source_ids[@]} -gt 0 ]] || fail "$file source_refs has no source ids"

  for source_id in "${source_ids[@]}"; do
    [[ -n "$source_id" ]] || fail "$file contains empty source id"
    [[ -f "$SOURCES_DIR/${source_id}.md" ]] || fail "$file references missing source manifest: $SOURCES_DIR/${source_id}.md"
  done
done

# Reuse existing ratio gate.
bash tests/benchmark/scripts/validate-source-composition.sh >/dev/null || fail "source-composition validation failed"

echo "dataset-validate: PASS cases=${#expected_files[@]} warnings=$warn_count"
