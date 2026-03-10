# Shared Artifact Fixtures

This directory is the single physical home for vendored real-world artifacts
used by Evidra's acceptance and benchmark workflows.

Rules:

- benchmark cases and acceptance tests both reference files from this root
- provenance classification belongs in metadata, not in directory ownership
- organize files by artifact family (`k8s/`, `terraform/`, `helm/`, etc.)
- imports come from reviewed upstream sources, not runtime downloads in CI
- prefer adding new shared fixtures here instead of creating duplicate copies in
  test-suite-specific directories

The authoritative inventory for acceptance-facing fixtures remains:

- `tests/artifacts/catalog.yaml`

Benchmark case metadata should reference these files directly through
`artifact_ref` paths.
