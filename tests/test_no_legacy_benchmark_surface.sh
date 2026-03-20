#!/usr/bin/env bash
set -euo pipefail
FAILED=0

# Check for legacy benchmark routes in active non-test Go code.
if grep -rn --include='*.go' --exclude='*_test.go' '/v1/benchmark/' cmd/evidra-api internal/api pkg/client 2>/dev/null; then
  echo "ERROR: legacy benchmark route still present in code" >&2
  FAILED=1
fi

# Check for dead benchmark CLI stub
if [ -e cmd/evidra/benchmark.go ]; then
  echo "ERROR: dead benchmark CLI stub still present" >&2
  FAILED=1
fi

exit $FAILED
