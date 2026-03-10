#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  tests/benchmark/scripts/import-k8s-doc-examples.sh <kubernetes-website-root> [dest-root]

Copies a curated first-wave slice of Kubernetes website examples into the
benchmark corpus.
EOF
}

fail() {
  echo "import-k8s-docs: $*" >&2
  exit 1
}

[[ $# -ge 1 ]] || { usage >&2; exit 1; }
[[ "$1" == "-h" || "$1" == "--help" ]] && { usage; exit 0; }

SRC_ROOT="$1"
DEST_ROOT="${2:-tests/benchmark/corpus}"

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

copy_fixture "content/en/examples/application/nginx/nginx-deployment.yaml" \
  "k8s-website-nginx-deployment.yaml"
copy_fixture "content/en/examples/pods/security/security-context.yaml" \
  "k8s-website-security-context-pod.yaml"

echo "import-k8s-docs: wrote 2 fixtures to $DEST_DIR"
