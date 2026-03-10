# Benchmark Source Manifest

```yaml
source_id: checkov-terraform
source_type: oss
source_composition: real-derived
source_url: https://github.com/bridgecrewio/checkov
source_path: tests/terraform/checks/resource/aws/example_S3SecureDataTransport/main.tf; tests/terraform/checks/resource/aws/example_IAMAdminPolicyDocument/iam.tf
source_commit_or_tag: 8bd89be03d239ff1f118a79a821f989fb119c16c
source_license: Apache-2.0
retrieved_at: 2026-03-10
retrieved_by: @agent
transformation_notes: |
  Terraform benchmark fixtures are derived from canonical Checkov Terraform
  check fixtures. This wave vendors the upstream files into the corpus and also
  permits small deterministic slices of those files when a benchmark case needs
  one pass/fail pattern isolated from a larger example.
reviewer: @agent
linked_cases:
  - tf-s3-public-access-fail
  - tf-s3-public-access-pass
  - tf-iam-wildcard-policy-fail
  - tf-iam-wildcard-policy-pass
```
