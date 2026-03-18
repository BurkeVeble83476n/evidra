#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -Fq "Direct full mode" README.md \
  || fail "README should describe direct full mode"

grep -Fq "Direct smart mode" README.md \
  || fail "README should describe direct smart mode"

grep -Fq "Proxy mode" README.md \
  || fail "README should describe proxy mode"

grep -Fq "Direct full mode" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe direct full mode"

grep -Fq "Direct smart mode" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe direct smart mode"

grep -Fq "Proxy mode" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe proxy mode"

grep -Fq "All three modes feed the same evidence chain" docs/ARCHITECTURE.md \
  || fail "architecture overview should explain the shared evidence chain"

grep -Fq "smart prescribe" docs/guides/skill-setup.md \
  || fail "skill setup should mention smart prescribe"

echo "PASS: test_smart_prescribe_docs"
