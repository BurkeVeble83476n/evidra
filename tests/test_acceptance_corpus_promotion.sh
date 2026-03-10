#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

CATALOG="tests/artifacts/catalog.yaml"
E2E_TEST="tests/e2e/real_world_test.go"

[[ -f "$CATALOG" ]] || fail "missing $CATALOG"
[[ -f "$E2E_TEST" ]] || fail "missing $E2E_TEST"

if rg -q 'tests/artifacts/real/k8s_app_stack.yaml|tests/artifacts/real/tf_infra_plan.json' "$CATALOG"; then
  fail "acceptance catalog still references low-provenance k8s/terraform fixtures"
fi

required_catalog_paths=(
  "tests/benchmark/corpus/k8s/kubescape-hostpath-mount-fail.yaml"
  "tests/benchmark/corpus/k8s/kubescape-non-root-deployment-pass.yaml"
  "tests/benchmark/corpus/terraform/checkov-s3-public-access-fail.tfplan.json"
  "tests/benchmark/corpus/terraform/checkov-iam-wildcard-fail.tfplan.json"
)

for path in "${required_catalog_paths[@]}"; do
  rg -Fq "$path" "$CATALOG" || fail "catalog missing promoted corpus artifact $path"
done

if rg -q 'k8s_app_stack.yaml|tf_infra_plan.json' "$E2E_TEST"; then
  fail "real_world_test.go still references curated k8s/terraform fixtures"
fi

for needle in \
  'kubescape-hostpath-mount-fail.yaml' \
  'kubescape-non-root-deployment-pass.yaml' \
  'checkov-s3-public-access-fail.tfplan.json' \
  'checkov-iam-wildcard-fail.tfplan.json'
do
  rg -Fq "$needle" "$E2E_TEST" || fail "real_world_test.go missing promoted corpus fixture $needle"
done

echo "PASS: test_acceptance_corpus_promotion"
