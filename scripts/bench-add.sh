#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/bench-add.sh <case-id> [--artifact <path>] [--source <source-id>]

Examples:
  scripts/bench-add.sh k8s-hostpath-mount-fail --artifact /tmp/hostpath.yaml --source kubescape-regolibrary
  scripts/bench-add.sh tf-s3-public-access-fail --source checkov-terraform
EOF
}

if [[ $# -lt 1 ]]; then
  usage >&2
  exit 1
fi

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
  usage
  exit 0
fi

CASE_ID="$1"
shift

ARTIFACT=""
SOURCE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --artifact)
      [[ $# -ge 2 ]] || { echo "bench-add: --artifact requires a value" >&2; exit 1; }
      ARTIFACT="$2"
      shift 2
      ;;
    --source)
      [[ $# -ge 2 ]] || { echo "bench-add: --source requires a value" >&2; exit 1; }
      SOURCE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "bench-add: unknown arg: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CASE_DIR="tests/benchmark/cases/$CASE_ID"
SOURCE_FILE="tests/benchmark/sources/${SOURCE:-TODO}.md"

if [[ -d "$CASE_DIR" ]]; then
  echo "bench-add: case already exists at $CASE_DIR" >&2
  exit 1
fi

mkdir -p "$CASE_DIR/artifacts" "$CASE_DIR/golden"

ARTIFACT_REF="TODO"
ARTIFACT_DIGEST="TODO"
if [[ -n "$ARTIFACT" ]]; then
  if [[ ! -f "$ARTIFACT" ]]; then
    echo "bench-add: artifact not found: $ARTIFACT" >&2
    exit 1
  fi
  ARTIFACT_BASENAME="$(basename "$ARTIFACT")"
  cp "$ARTIFACT" "$CASE_DIR/artifacts/$ARTIFACT_BASENAME"
  ARTIFACT_REF="artifacts/$ARTIFACT_BASENAME"
  if command -v shasum >/dev/null 2>&1; then
    ARTIFACT_DIGEST="sha256:$(shasum -a 256 "$CASE_DIR/artifacts/$ARTIFACT_BASENAME" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    ARTIFACT_DIGEST="sha256:$(sha256sum "$CASE_DIR/artifacts/$ARTIFACT_BASENAME" | awk '{print $1}')"
  fi
fi

cat > "$CASE_DIR/README.md" <<EOF
# $CASE_ID

## Scenario: TODO title

**Category:** TODO  
**Difficulty:** TODO  
**Dataset label:** limited-contract-baseline

**Story:** TODO describe what automation does.

**Impact:** TODO describe concrete operational impact.

**Risk:** TODO describe why this is risky.

**Real-world parallel:** TODO cite CVE/incident/pattern.
EOF

cat > "$CASE_DIR/expected.json" <<EOF
{
  "case_id": "$CASE_ID",
  "dataset_label": "limited-contract-baseline",
  "case_kind": "artifact",
  "category": "TODO",
  "difficulty": "TODO",
  "ground_truth_pattern": "TODO",
  "artifact_ref": "$ARTIFACT_REF",
  "artifact_digest": "$ARTIFACT_DIGEST",
  "risk_details_expected": [],
  "risk_level": "TODO",
  "signals_expected": {},
  "tags": [],
  "source_refs": [
    {
      "source_id": "${SOURCE:-TODO}",
      "composition": "real-derived"
    }
  ]
}
EOF

if [[ -n "$SOURCE" ]] && [[ ! -f "$SOURCE_FILE" ]]; then
  cat > "$SOURCE_FILE" <<EOF
# Benchmark Source Manifest

\`\`\`yaml
source_id: $SOURCE
source_type: oss
source_composition: real-derived
source_url: TODO
source_path: TODO
source_commit_or_tag: TODO
source_license: TODO
retrieved_at: $(date -u +%Y-%m-%d)
retrieved_by: TODO
transformation_notes: |
  TODO
reviewer: TODO
linked_cases:
  - $CASE_ID
\`\`\`
EOF
  echo "bench-add: created source manifest template: $SOURCE_FILE"
fi

echo "bench-add: created case scaffold: $CASE_DIR"
echo "bench-add: next steps:"
echo "  1) Fill TODO fields in $CASE_DIR/README.md and $CASE_DIR/expected.json"
echo "  2) Ensure source manifest is complete (if created): $SOURCE_FILE"
echo "  3) Run: bash tests/benchmark/scripts/validate-dataset.sh"
