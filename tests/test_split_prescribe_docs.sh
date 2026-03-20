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

grep -Fq "prescribe_full" docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md \
  || fail "core data model should mention prescribe_full"

grep -Fq "prescribe_smart" docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md \
  || fail "core data model should mention prescribe_smart"

grep -Fq "prescribe_full" docs/system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md \
  || fail "end-to-end example should mention prescribe_full"

grep -Fq "prescribe_smart" docs/system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md \
  || fail "end-to-end example should mention prescribe_smart"

grep -Fq "\"origin\":\"mcp\"" docs/system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md \
  || fail "end-to-end MCP example should use actor.origin"

! grep -Fq "MCP tool endpoint for agent calls (\`prescribe\`, \`report\`)" docs/system-design/EVIDRA_END_TO_END_EXAMPLE_V1.md \
  || fail "end-to-end example should not describe a single prescribe MCP tool"

! grep -Fq "The MCP tools \`prescribe\` and \`report\` accept caller-provided" docs/system-design/EVIDRA_CORE_DATA_MODEL_V1.md \
  || fail "core data model should not describe the MCP surface as a single prescribe tool"

grep -Fq "default \`v1.1.0\`" docs/integrations/cli-reference.md \
  || fail "CLI reference should document v1.1.0 as the prompt default"

echo "PASS: test_split_prescribe_docs"
