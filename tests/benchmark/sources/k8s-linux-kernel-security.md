# Benchmark Source Manifest

```yaml
source_id: k8s-linux-kernel-security
source_type: oss
source_composition: real-derived
source_url: https://github.com/kubernetes/website
source_path: content/en/docs/concepts/security/linux-kernel-security-constraints.md
source_commit_or_tag: 9d228603998ffaa8d0c38df1ab299a0cc457e61a
source_license: Apache-2.0
retrieved_at: 2026-03-10
retrieved_by: @agent
transformation_notes: |
  Used as normative context for hostPath and privileged runtime constraints.
  The markdown source is pinned to the exact upstream website commit instead of
  a floating docs page snapshot.
reviewer: @agent
linked_cases:
  - k8s-hostpath-mount-fail
  - k8s-hostpath-mount-pass
```
