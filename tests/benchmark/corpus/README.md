# Benchmark Corpus

This directory is the raw artifact corpus used to promote benchmark cases.

Rules:
- Corpus files are append-only. Do not mutate files in place.
- If an artifact needs changes, create a new file with a version suffix.
- Every promoted case must reference corpus provenance through `source_refs`.
- This repository currently uses a `limited-contract-baseline` dataset label; corpus imports in this phase are intentionally limited and must be marked as such in case metadata.

See [EVIDRA_DATASET_ARCHITECTURE](../../../docs/system-design/EVIDRA_DATASET_ARCHITECTURE.md) for full collection and promotion rules.
