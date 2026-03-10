#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

CASES_DIR="tests/benchmark/cases"
CASE_TEMPLATE="tests/benchmark/cases/TEMPLATE.md"

fail() {
  echo "case-metadata-validate: FAIL $*" >&2
  exit 1
}

[[ -d "$CASES_DIR" ]] || fail "missing $CASES_DIR"
[[ -f "$CASE_TEMPLATE" ]] || fail "missing $CASE_TEMPLATE"

if ! command -v jq >/dev/null 2>&1; then
  fail "jq is required"
fi

expected_files=()
while IFS= read -r file; do
  expected_files+=("$file")
done < <(find "$CASES_DIR" -type f -name "expected.json" | sort)

[[ ${#expected_files[@]} -gt 0 ]] || fail "no expected.json files found"

for file in "${expected_files[@]}"; do
  jq -e '
    (.scenario_class | IN("safe_routine", "normal_mutate", "high_risk_ambiguous", "decline_worthy")) and
    (.operation_class | IN("inspect_read", "mutate_change", "deploy_rollout")) and
    (.risk_level | IN("low", "medium", "high", "critical")) and
    (.environment_class | IN("sandbox", "staging", "prod_like")) and
    (.artifact_ref | startswith("../../corpus/"))
  ' "$file" >/dev/null || fail "$file missing required scenario metadata"
done

echo "case-metadata-validate: PASS"
