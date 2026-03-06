# Benchmark Source Manifest

```yaml
source_id: kubescape-regolibrary
source_type: oss
source_composition: real-derived
source_url: https://github.com/kubescape/regolibrary
source_path: rules/ and examples/
source_commit_or_tag: repo-snapshot-2026-03-06
source_license: Apache-2.0
retrieved_at: 2026-03-06
retrieved_by: @agent
transformation_notes: |
  Used as provenance for Kubernetes security misconfiguration coverage.
  This P0 baseline references semantics; direct fixture ingestion happens in later phases.
reviewer: @agent
linked_cases:
  - k8s-run-as-root-fail
  - k8s-run-as-root-pass
```

