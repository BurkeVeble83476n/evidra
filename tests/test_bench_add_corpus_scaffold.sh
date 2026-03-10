#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

CASE_ID="tmp-bench-add-corpus-scaffold"
CASE_DIR="tests/benchmark/cases/${CASE_ID}"
SOURCE_FILE="tests/benchmark/sources/tmp-bench-add-corpus-source.md"
ARTIFACT="tests/artifacts/fixtures/k8s/kubescape-hostpath-mount-fail.yaml"

cleanup() {
  rm -rf "$CASE_DIR" "$SOURCE_FILE"
}
trap cleanup EXIT
cleanup

bash scripts/bench-add.sh \
  "$CASE_ID" \
  --artifact "$ARTIFACT" \
  --source tmp-bench-add-corpus-source \
  --tool kubectl \
  --no-process >/tmp/test-bench-add-corpus-scaffold.log

EXPECTED_JSON="$CASE_DIR/expected.json"
README_FILE="$CASE_DIR/README.md"

[[ -f "$EXPECTED_JSON" ]] || fail "missing $EXPECTED_JSON"
[[ -f "$README_FILE" ]] || fail "missing $README_FILE"
[[ -f "$SOURCE_FILE" ]] || fail "missing $SOURCE_FILE"

artifact_ref="$(jq -r '.artifact_ref' "$EXPECTED_JSON")"
[[ "$artifact_ref" == "../../../artifacts/fixtures/k8s/kubescape-hostpath-mount-fail.yaml" ]] \
  || fail "artifact_ref should point at shared fixtures, got $artifact_ref"

jq -e '
  .scenario_class == "normal_mutate" and
  .operation_class == "deploy_rollout" and
  .environment_class == "sandbox"
' "$EXPECTED_JSON" >/dev/null || fail "expected.json missing default scenario metadata"

rg -q '\*\*Scenario class:\*\* normal_mutate' "$README_FILE" \
  || fail "README missing scenario class"
rg -q '\*\*Operation class:\*\* deploy_rollout' "$README_FILE" \
  || fail "README missing operation class"
rg -q '\*\*Environment class:\*\* sandbox' "$README_FILE" \
  || fail "README missing environment class"

[[ ! -d "$CASE_DIR/artifacts" ]] || fail "bench-add should not create case-local artifacts dir"

echo "PASS: test_bench_add_corpus_scaffold"
