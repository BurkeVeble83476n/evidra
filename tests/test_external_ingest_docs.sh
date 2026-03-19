#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local pattern="$1"
  local path="$2"
  if ! rg -q --fixed-strings -- "$pattern" "$path"; then
    fail "missing '$pattern' in $path"
  fi
}

assert_contains "/v1/evidence/ingest/prescribe" "cmd/evidra-api/static/openapi.yaml"
assert_contains "/v1/evidence/ingest/report" "cmd/evidra-api/static/openapi.yaml"
assert_contains "Typed shared external lifecycle ingest route for prescribe entries" "cmd/evidra-api/static/openapi.yaml"
assert_contains "Typed shared external lifecycle ingest route for report entries" "cmd/evidra-api/static/openapi.yaml"
assert_contains "contract_version" "cmd/evidra-api/static/openapi.yaml"
assert_contains "session_id" "cmd/evidra-api/static/openapi.yaml"
assert_contains "operation_id" "cmd/evidra-api/static/openapi.yaml"
assert_contains "trace_id" "cmd/evidra-api/static/openapi.yaml"
assert_contains "evidence:" "cmd/evidra-api/static/openapi.yaml"
assert_contains "source:" "cmd/evidra-api/static/openapi.yaml"
assert_contains "canonical_action" "cmd/evidra-api/static/openapi.yaml"
assert_contains "smart_target" "cmd/evidra-api/static/openapi.yaml"
assert_contains "prescription_id" "cmd/evidra-api/static/openapi.yaml"
assert_contains "artifact_digest" "cmd/evidra-api/static/openapi.yaml"
assert_contains "verdict" "cmd/evidra-api/static/openapi.yaml"
assert_contains "decision_context" "cmd/evidra-api/static/openapi.yaml"
assert_contains "exit_code" "cmd/evidra-api/static/openapi.yaml"
assert_contains "Compatibility wrapper over the shared lifecycle ingest service" "cmd/evidra-api/static/openapi.yaml"
assert_contains "workflow" "cmd/evidra-api/static/openapi.yaml"
assert_contains "declared" "cmd/evidra-api/static/openapi.yaml"
assert_contains "observed" "cmd/evidra-api/static/openapi.yaml"
assert_contains "translated" "cmd/evidra-api/static/openapi.yaml"

assert_contains "/v1/evidence/ingest/prescribe" "docs/guides/self-hosted-setup.md"
assert_contains "/v1/evidence/ingest/report" "docs/guides/self-hosted-setup.md"
assert_contains "compatibility wrapper" "docs/guides/self-hosted-setup.md"
assert_contains "/v1/evidence/forward" "docs/guides/self-hosted-setup.md"
assert_contains "/v1/evidence/batch" "docs/guides/self-hosted-setup.md"
assert_contains "workflow" "docs/guides/self-hosted-setup.md"
assert_contains "declared" "docs/guides/self-hosted-setup.md"
assert_contains "observed" "docs/guides/self-hosted-setup.md"
assert_contains "translated" "docs/guides/self-hosted-setup.md"

assert_contains "Webhook routes are compatibility wrappers" "docs/ARCHITECTURE.md"
assert_contains "payload.flavor" "docs/ARCHITECTURE.md"
assert_contains "declared" "docs/ARCHITECTURE.md"
assert_contains "observed" "docs/ARCHITECTURE.md"
assert_contains "translated" "docs/ARCHITECTURE.md"

assert_contains "workflow" "docs/system-design/EVIDRA_PROTOCOL_V1.md"
assert_contains "declared" "docs/system-design/EVIDRA_PROTOCOL_V1.md"
assert_contains "observed" "docs/system-design/EVIDRA_PROTOCOL_V1.md"
assert_contains "translated" "docs/system-design/EVIDRA_PROTOCOL_V1.md"

assert_contains "workflow" "docs/system-design/EVIDRA_ARCHITECTURE_V1.md"
assert_contains "declared" "docs/system-design/EVIDRA_ARCHITECTURE_V1.md"
assert_contains "observed" "docs/system-design/EVIDRA_ARCHITECTURE_V1.md"
assert_contains "translated" "docs/system-design/EVIDRA_ARCHITECTURE_V1.md"

echo "PASS: test_external_ingest_docs"
