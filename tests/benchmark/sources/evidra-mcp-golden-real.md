# Benchmark Source Manifest

```yaml
source_id: evidra-mcp-golden-real
source_type: seed
source_composition: real-derived
source_url: ../evidra-mcp/tests/golden_real/
source_path: tests/golden_real/
source_commit_or_tag: local-workspace-snapshot-2026-03-06
source_license: Apache-2.0
retrieved_at: 2026-03-06
retrieved_by: @agent
transformation_notes: |
  Seed corpus for P0 bootstrap. Selected allow/deny pairs are copied into
  tests/benchmark/cases with case-level expected contract metadata.
reviewer: @agent
linked_cases:
  - k8s-privileged-container-fail
  - k8s-privileged-container-pass
  - k8s-hostpath-mount-fail
  - k8s-hostpath-mount-pass
  - k8s-run-as-root-fail
  - k8s-run-as-root-pass
  - tf-s3-public-access-fail
  - tf-s3-public-access-pass
  - tf-iam-wildcard-policy-fail
  - tf-iam-wildcard-policy-pass
```

