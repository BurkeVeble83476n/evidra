#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -Fq "prescribe_full" README.md \
  || fail "README should mention prescribe_full"

grep -Fq "prescribe_smart" README.md \
  || fail "README should mention prescribe_smart"

grep -Fq "prescribe_full" docs/guides/mcp-setup.md \
  || fail "mcp-setup should mention prescribe_full"

grep -Fq "prescribe_smart" docs/guides/mcp-setup.md \
  || fail "mcp-setup should mention prescribe_smart"

grep -Fq "prescribe_full" docs/guides/skill-setup.md \
  || fail "skill-setup should mention prescribe_full"

grep -Fq "prescribe_smart" docs/guides/skill-setup.md \
  || fail "skill-setup should mention prescribe_smart"

grep -Fq "prescribe_full" docs/ARCHITECTURE.md \
  || fail "architecture overview should mention prescribe_full"

grep -Fq "prescribe_smart" docs/ARCHITECTURE.md \
  || fail "architecture overview should mention prescribe_smart"

grep -Fq "default \`v1.1.0\`" docs/integrations/cli-reference.md \
  || fail "CLI reference should document v1.1.0 as the prompt default"

echo "PASS: test_split_prescribe_docs"
