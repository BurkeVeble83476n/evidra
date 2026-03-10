#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  tests/benchmark/scripts/import-kubescape-fixtures.sh <kubescape-regolibrary-root> [dest-root]

Copies a curated first-wave slice of Kubescape Kubernetes fixtures into the
shared artifact fixture root.
EOF
}

fail() {
  echo "import-kubescape: $*" >&2
  exit 1
}

[[ $# -ge 1 ]] || { usage >&2; exit 1; }
[[ "$1" == "-h" || "$1" == "--help" ]] && { usage; exit 0; }

SRC_ROOT="$1"
DEST_ROOT="${2:-tests/artifacts/fixtures}"

[[ -d "$SRC_ROOT" ]] || fail "source root not found: $SRC_ROOT"

DEST_DIR="${DEST_ROOT%/}/k8s"
mkdir -p "$DEST_DIR"

copy_fixture() {
  local rel_src="$1"
  local dest_name="$2"
  local src_path="$SRC_ROOT/$rel_src"
  [[ -f "$src_path" ]] || fail "missing upstream fixture: $src_path"
  cp "$src_path" "$DEST_DIR/$dest_name"
}

copy_fixture "rules/rule-privileged-container/test/workloads/input/deployment.yaml" \
  "kubescape-privileged-container-fail.yaml"
copy_fixture "controls/examples/c045.yaml" \
  "kubescape-hostpath-mount-fail.yaml"
copy_fixture "rules/non-root-containers/test/deployment-fail/input/deployment.yaml" \
  "kubescape-non-root-deployment-fail.yaml"
copy_fixture "rules/non-root-containers/test/deployment-pass/input/deployment.yaml" \
  "kubescape-non-root-deployment-pass.yaml"

echo "import-kubescape: wrote 4 fixtures to $DEST_DIR"
