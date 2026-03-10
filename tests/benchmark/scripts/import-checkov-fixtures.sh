#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  tests/benchmark/scripts/import-checkov-fixtures.sh <checkov-root> [dest-root]

Copies a curated first-wave slice of Checkov Terraform fixtures into the
benchmark corpus. The corpus keeps small HCL slices for provenance inspection
and also emits minimal Terraform plan JSON files that match Evidra's current
Terraform runtime input path.
EOF
}

fail() {
  echo "import-checkov: $*" >&2
  exit 1
}

[[ $# -ge 1 ]] || { usage >&2; exit 1; }
[[ "$1" == "-h" || "$1" == "--help" ]] && { usage; exit 0; }

SRC_ROOT="$1"
DEST_ROOT="${2:-tests/benchmark/corpus}"

[[ -d "$SRC_ROOT" ]] || fail "source root not found: $SRC_ROOT"

DEST_DIR="${DEST_ROOT%/}/terraform"
mkdir -p "$DEST_DIR"

extract_block() {
  local file="$1"
  local pattern="$2"
  local out="$3"
  [[ -f "$file" ]] || fail "missing upstream file: $file"
  awk -v pattern="$pattern" '
    BEGIN { capture = 0; depth = 0 }
    $0 ~ pattern {
      capture = 1
    }
    capture {
      print
      opens = gsub(/\{/, "{")
      closes = gsub(/\}/, "}")
      depth += opens - closes
      if (depth == 0) {
        exit
      }
    }
  ' "$file" > "$out"
  [[ -s "$out" ]] || fail "failed to extract pattern $pattern from $file"
}

S3_FILE="$SRC_ROOT/tests/terraform/checks/resource/aws/example_S3SecureDataTransport/main.tf"
IAM_FILE="$SRC_ROOT/tests/terraform/checks/resource/aws/example_IAMAdminPolicyDocument/iam.tf"
S3_PASS_PLAN="$SRC_ROOT/tests/terraform/parser/resources/plan_module_with_connected_resources/tfplan.json"
S3_FAIL_PLAN="$SRC_ROOT/tests/terraform/runner/resources/plan_nested_child_modules_with_connections/tfplan.json"

extract_block "$S3_FILE" 'resource "aws_s3_bucket_public_access_block" "fail1"' \
  "$DEST_DIR/checkov-s3-public-access-fail.tf"
extract_block "$S3_FILE" 'resource "aws_s3_bucket_public_access_block" "pass_restricted"' \
  "$DEST_DIR/checkov-s3-public-access-pass.tf"
extract_block "$IAM_FILE" 'resource "aws_iam_policy" "fail3"' \
  "$DEST_DIR/checkov-iam-wildcard-fail.tf"
extract_block "$IAM_FILE" 'resource "aws_iam_policy" "pass2"' \
  "$DEST_DIR/checkov-iam-wildcard-pass.tf"

extract_policy_doc() {
  local file="$1"
  local resource_name="$2"
  awk -v resource_name="$resource_name" '
    $0 ~ "resource \"aws_iam_policy\" \"" resource_name "\"" { in_resource = 1 }
    in_resource && /policy = <<POLICY/ { in_policy = 1; next }
    in_policy && /^POLICY$/ { exit }
    in_policy { print }
  ' "$file"
}

build_iam_plan() {
  local resource_name="$1"
  local policy_doc="$2"
  local out="$3"
  jq -n \
    --arg name "$resource_name" \
    --arg policy "$policy_doc" \
    '{
      format_version: "1.2",
      terraform_version: "1.5.7",
      planned_values: {root_module: {resources: []}},
      resource_changes: [
        {
          address: ("aws_iam_policy." + $name),
          mode: "managed",
          type: "aws_iam_policy",
          name: $name,
          change: {
            actions: ["create"],
            after: {
              name: $name,
              path: "/",
              policy: $policy
            }
          }
        }
      ]
    }' > "$out"
}

jq '{
  format_version: .format_version,
  terraform_version: .terraform_version,
  planned_values: {root_module: {resources: []}},
  resource_changes: [
    .resource_changes[]
    | select(
        .address == "module.s3-bucket.aws_s3_bucket.this[0]"
        or .address == "module.s3-bucket.aws_s3_bucket_public_access_block.this[0]"
      )
  ]
}' "$S3_PASS_PLAN" > "$DEST_DIR/checkov-s3-public-access-pass.tfplan.json"

jq '{
  format_version: .format_version,
  terraform_version: .terraform_version,
  planned_values: {root_module: {resources: []}},
  resource_changes: [
    .resource_changes[]
    | select(
        .address == "module.s3_bucket.aws_s3_bucket.this[0]"
        or .address == "module.s3_bucket.aws_s3_bucket_acl.this[0]"
        or .address == "module.s3_bucket.aws_s3_bucket_public_access_block.this[0]"
      )
  ]
}' "$S3_FAIL_PLAN" > "$DEST_DIR/checkov-s3-public-access-fail.tfplan.json"

iam_fail_policy="$(extract_policy_doc "$IAM_FILE" "fail3")"
iam_pass_policy="$(extract_policy_doc "$IAM_FILE" "pass2")"

[[ -n "$iam_fail_policy" ]] || fail "failed to extract fail1 IAM policy"
[[ -n "$iam_pass_policy" ]] || fail "failed to extract pass1 IAM policy"

build_iam_plan "fail3" "$iam_fail_policy" \
  "$DEST_DIR/checkov-iam-wildcard-fail.tfplan.json"
build_iam_plan "pass2" "$iam_pass_policy" \
  "$DEST_DIR/checkov-iam-wildcard-pass.tfplan.json"

echo "import-checkov: wrote 4 HCL slices and 4 plan JSON fixtures to $DEST_DIR"
