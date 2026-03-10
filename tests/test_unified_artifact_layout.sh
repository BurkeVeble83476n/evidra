#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

CATALOG="tests/artifacts/catalog.yaml"
old_benchmark_root='tests/benchmark/'"corpus"
old_acceptance_root='tests/artifacts/'"real"
legacy_helper_regex='realFixture\(|corpusFixture\('

[[ -f "$CATALOG" ]] || fail "missing $CATALOG"

if rg -n "${old_benchmark_root}/|${old_acceptance_root}/" "$CATALOG" >/dev/null; then
  fail "catalog still points at split artifact roots"
fi

if rg -n \
  -g '!docs/plans/**' \
  -g '!tests/test_unified_artifact_layout.sh' \
  "${old_benchmark_root}|${old_acceptance_root}|${legacy_helper_regex}" \
  README.md docs tests scripts cmd internal pkg .github Makefile >/dev/null
then
  fail "active repo references still use old artifact roots or split fixture helpers"
fi

echo "PASS: test_unified_artifact_layout"
