# Benchmark Source Manifest

```yaml
source_id: k8s-pod-security-standards
source_type: oss
source_composition: real-derived
source_url: https://kubernetes.io/docs/concepts/security/pod-security-standards/
source_path: docs/concepts/security/pod-security-standards/
source_commit_or_tag: docs-snapshot-2026-03-06
source_license: Apache-2.0
retrieved_at: 2026-03-06
retrieved_by: @agent
transformation_notes: |
  Used as normative context for privileged security controls.
  No direct upstream file vendored in this step.
reviewer: @agent
linked_cases:
  - k8s-privileged-container-fail
  - k8s-privileged-container-pass
```

