# Benchmark Source Manifest

```yaml
source_id: checkov-terraform
source_type: oss
source_composition: real-derived
source_url: https://github.com/bridgecrewio/checkov
source_path: tests/terraform/
source_commit_or_tag: repo-snapshot-2026-03-06
source_license: Apache-2.0
retrieved_at: 2026-03-06
retrieved_by: @agent
transformation_notes: |
  Terraform risk patterns are derived from canonical Checkov test fixtures.
  Case artifacts in this repo are normalized to stable JSON inputs.
reviewer: @agent
linked_cases:
  - tf-s3-public-access-fail
  - tf-s3-public-access-pass
  - tf-iam-wildcard-policy-fail
  - tf-iam-wildcard-policy-pass
```

