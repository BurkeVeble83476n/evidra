# Benchmark Source Manifest

```yaml
source_id: k8s-linux-kernel-security
source_type: oss
source_composition: real-derived
source_url: https://kubernetes.io/docs/concepts/security/linux-kernel-security-constraints/
source_path: docs/concepts/security/linux-kernel-security-constraints/
source_commit_or_tag: docs-snapshot-2026-03-06
source_license: Apache-2.0
retrieved_at: 2026-03-06
retrieved_by: @agent
transformation_notes: |
  Used as provenance for hostPath and privileged security semantics.
  No direct upstream file vendored in this step.
reviewer: @agent
linked_cases:
  - k8s-hostpath-mount-fail
  - k8s-hostpath-mount-pass
```

