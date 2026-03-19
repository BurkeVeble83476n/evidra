#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -Fq "Full Prescribe" README.md \
  || fail "README should describe Full Prescribe"

grep -Fq "Smart Prescribe" README.md \
  || fail "README should describe Smart Prescribe"

grep -Fq "Proxy Observed" README.md \
  || fail "README should describe Proxy Observed"

! grep -Fq "Direct full mode" docs/guides/mcp-setup.md \
  || fail "mcp-setup should not use direct full mode wording"

! grep -Fq "Direct smart mode" docs/guides/mcp-setup.md \
  || fail "mcp-setup should not use direct smart mode wording"

grep -Fq "Full Prescribe" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe Full Prescribe"

grep -Fq "Smart Prescribe" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe Smart Prescribe"

grep -Fq "Proxy Observed" docs/guides/mcp-setup.md \
  || fail "mcp-setup should describe Proxy Observed"

grep -Fq "All three modes feed the same evidence chain" docs/ARCHITECTURE.md \
  || fail "architecture overview should explain the shared evidence chain"

grep -Fq "smart prescribe" docs/guides/skill-setup.md \
  || fail "skill setup should mention smart prescribe"

grep -Fq '"prescribe_full", "prescribe_smart", "report", and "get_event"' docs/guides/mcp-setup.md \
  || fail "mcp-setup system prompt should list split prescribe tools"

! grep -Fq '"prescribe", "report", and "get_event"' docs/guides/mcp-setup.md \
  || fail "mcp-setup should not list legacy prescribe tool"

! grep -Fq "pipeline stages and deploy jobs" ui/src/pages/Landing.tsx \
  || fail "landing source should use workflow wording"

! grep -Fq "pipeline stages and deploy jobs" cmd/evidra-api/static/index.html \
  || fail "static landing should use workflow wording"

echo "PASS: test_mode_labels_docs"
