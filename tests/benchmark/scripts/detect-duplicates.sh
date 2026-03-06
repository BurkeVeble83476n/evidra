#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

CASES_DIR="tests/benchmark/cases"

fail() {
  echo "detect-duplicates: FAIL $*" >&2
  exit 1
}

if ! command -v jq >/dev/null 2>&1; then
  fail "jq is required"
fi

[[ -d "$CASES_DIR" ]] || fail "missing $CASES_DIR"

expected_files=()
while IFS= read -r file; do
  expected_files+=("$file")
done < <(find "$CASES_DIR" -type f -name "expected.json" | sort)

if [[ ${#expected_files[@]} -eq 0 ]]; then
  fail "no expected.json files found"
fi

tmp_pairs="$(mktemp)"
trap 'rm -f "$tmp_pairs"' EXIT

for expected in "${expected_files[@]}"; do
  case_id="$(jq -r '.case_id // empty' "$expected")"
  if [[ -z "$case_id" ]]; then
    echo "detect-duplicates: WARN skip $expected (missing case_id)" >&2
    continue
  fi

  contract_path="$(dirname "$expected")/golden/contract.json"
  if [[ ! -f "$contract_path" ]]; then
    echo "detect-duplicates: WARN skip $case_id (missing $contract_path)" >&2
    continue
  fi

  ground_truth_pattern="$(jq -r '.ground_truth_pattern // "unknown"' "$expected")"
  expected_risk_level="$(jq -r '.risk_level // "unknown"' "$expected")"
  expected_risk_tags="$(jq -r '.risk_details_expected // [] | sort | join(",")' "$expected")"

  operation_class="$(jq -r '.operation_class // "unknown"' "$contract_path")"
  scope_class="$(jq -r '.scope_class // "unknown"' "$contract_path")"
  canon_version="$(jq -r '.canon_version // "unknown"' "$contract_path")"
  adapter_version="$(jq -r '.evidra_version // "unknown"' "$contract_path")"
  resource_shape_hash="$(jq -r '.resource_shape_hash // ""' "$contract_path")"
  if [[ -z "$resource_shape_hash" || "$resource_shape_hash" == "null" ]]; then
    resource_shape_hash="$(jq -r '.artifact_digest // "unknown"' "$contract_path")"
  fi

  fp="${ground_truth_pattern}|${expected_risk_level}|${expected_risk_tags}|${resource_shape_hash}|${operation_class}|${scope_class}|${canon_version}|${adapter_version}"
  printf '%s\t%s\n' "$fp" "$case_id" >> "$tmp_pairs"
done

if [[ ! -s "$tmp_pairs" ]]; then
  echo "detect-duplicates: WARN no comparable cases found"
  echo "detect-duplicates: summary fingerprints=0 duplicate_groups=0 duplicate_cases=0"
  exit 0
fi

awk -F'\t' '
{
  fp=$1
  case_id=$2
  if (!(fp in first)) {
    first[fp]=case_id
    groups[fp]=case_id
    count[fp]=1
    order[++n]=fp
    next
  }
  groups[fp]=groups[fp]", "case_id
  count[fp]++
}
END {
  unique=0
  dup_groups=0
  dup_cases=0
  for (i=1; i<=n; i++) {
    fp=order[i]
    unique++
    if (count[fp] > 1) {
      dup_groups++
      dup_cases += (count[fp] - 1)
      printf("detect-duplicates: WARN DUPLICATE cases=[%s]\n", groups[fp])
      printf("detect-duplicates: WARN fingerprint=%s\n", fp)
    }
  }
  printf("detect-duplicates: summary fingerprints=%d duplicate_groups=%d duplicate_cases=%d\n", unique, dup_groups, dup_cases)
}
' "$tmp_pairs"
