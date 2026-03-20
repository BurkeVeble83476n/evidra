#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -Fq "POST /v1/evidence/findings" docs/system-design/EVIDRA_PROTOCOL_V1.md \
  || fail "protocol doc should use /v1/evidence/findings"

grep -Fq "POST /v1/evidence/forward" docs/system-design/EVIDRA_PROTOCOL_V1.md \
  || fail "protocol doc should mention /v1/evidence/forward"

grep -Fq "POST /v1/evidence/ingest/prescribe" docs/system-design/EVIDRA_PROTOCOL_V1.md \
  || fail "protocol doc should mention typed ingest prescribe route"

grep -Fq "EVIDRA_PROTOCOL_V1.md" docs/system-design/EVIDRA_PROMPT_FACTORY_SPEC_V1.md \
  || fail "prompt factory spec should reference EVIDRA_PROTOCOL_V1.md"

! grep -Fq "POST /v1/findings" docs/system-design/EVIDRA_PROTOCOL_V1.md \
  || fail "protocol doc should not mention legacy /v1/findings route"

! grep -Fq "Example schema for \`/v1/events\`" docs/system-design/EVIDRA_PROTOCOL_V1.md \
  || fail "protocol doc should not document a /v1/events schema"

! grep -Fq "EVIDRA_PROTOCOL.md" docs/system-design/EVIDRA_PROMPT_FACTORY_SPEC_V1.md \
  || fail "prompt factory spec should not reference stale EVIDRA_PROTOCOL.md"

echo "PASS: test_protocol_docs"
