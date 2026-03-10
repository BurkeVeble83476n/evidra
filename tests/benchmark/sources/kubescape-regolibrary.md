# Benchmark Source Manifest

```yaml
source_id: kubescape-regolibrary
source_type: oss
source_composition: real-derived
source_url: https://github.com/kubescape/regolibrary
source_path: rules/rule-privileged-container/test/workloads/input/deployment.yaml; controls/examples/c045.yaml; rules/non-root-containers/test/deployment-fail/input/deployment.yaml; rules/non-root-containers/test/deployment-pass/input/deployment.yaml
source_commit_or_tag: e7639f6653b4a4b274bb8de5aa6a0db3a4c85926
source_license: Apache-2.0
retrieved_at: 2026-03-10
retrieved_by: @agent
transformation_notes: |
  Kubernetes benchmark fixtures are copied from upstream Kubescape rule test
  inputs and control examples. The corpus keeps the raw upstream manifests and
  allows small deterministic slices only when isolating one benchmark pattern
  per case.
reviewer: @agent
linked_cases:
  - k8s-privileged-container-fail
  - k8s-privileged-container-pass
  - k8s-hostpath-mount-fail
  - k8s-hostpath-mount-pass
  - k8s-run-as-root-fail
  - k8s-run-as-root-pass
```
