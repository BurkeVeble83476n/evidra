#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "provenance-validate: FAIL $*" >&2
  exit 1
}

SOURCES_DIR="tests/benchmark/sources"
CATALOG_FILE="tests/artifacts/catalog.yaml"

[[ -d "$SOURCES_DIR" ]] || fail "missing $SOURCES_DIR"
[[ -f "$CATALOG_FILE" ]] || fail "missing $CATALOG_FILE"

if command -v rg >/dev/null 2>&1; then
  rg -n 'TODO|repo-snapshot-|local-workspace-snapshot-' "$SOURCES_DIR" \
    && fail "source manifests contain placeholder provenance"
else
  grep -REn 'TODO|repo-snapshot-|local-workspace-snapshot-' "$SOURCES_DIR" \
    && fail "source manifests contain placeholder provenance"
fi

tmp_catalog="$(mktemp)"
trap 'rm -f "$tmp_catalog"' EXIT

awk '
  $1 == "-" && $2 == "id:" {
    id = $3
    source_type = ""
    upstream_project = ""
  }
  $1 == "source_type:" {
    source_type = $2
  }
  $1 == "upstream_project:" {
    upstream_project = $2
    if (source_type != "curated_local" && upstream_project == "unknown") {
      print id
    }
  }
' "$CATALOG_FILE" > "$tmp_catalog"

if [[ -s "$tmp_catalog" ]]; then
  sed 's/^/artifact catalog placeholder upstream: /' "$tmp_catalog" >&2
  fail "artifact catalog contains unknown upstream project placeholders for non-local artifacts"
fi

echo "provenance-validate: PASS"
