#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

VALIDATOR="tests/benchmark/scripts/validate-provenance.sh"

[[ -x "$VALIDATOR" ]] || fail "missing executable $VALIDATOR"

bash "$VALIDATOR"

echo "PASS: test_oss_dataset_provenance"
