# Benchmark Source Manifest Template

Use this template for every benchmark data source.

Save as:

`tests/benchmark/sources/<source-id>.md`

## Manifest

```yaml
source_id: <kebab-case-id>
source_type: <seed|oss|incident|custom>
source_composition: <real-derived|custom-only>
source_url: <https://... or local path reference>
source_path: <upstream directory/file path used>
source_commit_or_tag: <exact git sha or released tag>
source_license: <Apache-2.0|MIT|BSD-2-Clause|BSD-3-Clause|MPL-2.0>
retrieved_at: <YYYY-MM-DD>
retrieved_by: <name or handle>
transformation_notes: |
  <what was kept, removed, or normalized for determinism and safety>
reviewer: <name or handle>
linked_cases:
  - <case-id-1>
  - <case-id-2>
```

## Validation Checklist

- License is in the allowed list.
- No secrets, credentials, or real cloud account IDs are present.
- URL/path + commit/tag + date are sufficient to reproduce retrieval.
- `source_commit_or_tag` is an exact git SHA or concrete released tag, never a
  placeholder snapshot label.
- `source_composition` accurately reflects how this source contributes to case provenance.
- `linked_cases` references only existing `tests/benchmark/cases/<case-id>` entries.
- `transformation_notes` explains all non-trivial edits.
