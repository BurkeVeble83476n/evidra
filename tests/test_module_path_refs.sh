#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

if git grep -n "samebits.com/evidra-benchmark" -- . ":(exclude)docs/plans/**" >/tmp/test-module-path-refs.out 2>/dev/null; then
  cat /tmp/test-module-path-refs.out >&2
  fail "old module path references remain"
fi

grep -Eq '^module samebits.com/evidra$' go.mod \
  || fail "go.mod missing module samebits.com/evidra"

grep -Fq 'samebits.com/evidra/cmd/evidra-mcp@latest' docs/guides/mcp-setup.md \
  || fail "mcp-setup missing new go install path"

grep -Fq 'samebits.com/evidra/cmd/evidra-mcp@latest' ui/src/pages/Landing.tsx \
  || fail "landing page missing new go install path"

echo "PASS: test_module_path_refs"
