#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT_DIR"

CASES_DIR="tests/benchmark/cases"
SOURCES_DIR="tests/benchmark/sources"

if [[ ! -d "$CASES_DIR" ]]; then
  echo "source-composition: skip (no $CASES_DIR directory)"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "source-composition: jq is required" >&2
  exit 2
fi

mapfile -t expected_files < <(find "$CASES_DIR" -type f -name "expected.json" | sort)
if [[ ${#expected_files[@]} -eq 0 ]]; then
  echo "source-composition: skip (no expected.json cases yet)"
  exit 0
fi

total_cases=0
real_derived_cases=0
custom_only_cases=0

for file in "${expected_files[@]}"; do
  total_cases=$((total_cases + 1))

  # source_refs is mandatory once cases exist.
  if ! jq -e '.source_refs and (.source_refs | type=="array") and (.source_refs | length > 0)' "$file" >/dev/null; then
    echo "source-composition: $file missing non-empty source_refs[]" >&2
    exit 1
  fi

  # All source refs must resolve to a source manifest.
  mapfile -t source_ids < <(jq -r '.source_refs[] | (.source_id // .id // .source // empty)' "$file")
  if [[ ${#source_ids[@]} -eq 0 ]]; then
    echo "source-composition: $file has source_refs but no source_id/id/source fields" >&2
    exit 1
  fi
  for source_id in "${source_ids[@]}"; do
    if [[ -z "$source_id" ]]; then
      echo "source-composition: $file contains empty source id" >&2
      exit 1
    fi
    if [[ ! -f "$SOURCES_DIR/${source_id}.md" ]]; then
      echo "source-composition: missing source manifest $SOURCES_DIR/${source_id}.md (referenced from $file)" >&2
      exit 1
    fi
  done

  real_count="$(jq '[.source_refs[] | select((.composition // .source_composition // "") == "real-derived")] | length' "$file")"
  custom_count="$(jq '[.source_refs[] | select((.composition // .source_composition // "") == "custom-only")] | length' "$file")"
  refs_count="$(jq '.source_refs | length' "$file")"

  if [[ "$real_count" -gt 0 ]]; then
    real_derived_cases=$((real_derived_cases + 1))
  fi
  if [[ "$custom_count" -eq "$refs_count" ]]; then
    custom_only_cases=$((custom_only_cases + 1))
  fi
done

real_pct="$(awk -v n="$real_derived_cases" -v d="$total_cases" 'BEGIN { printf "%.2f", (n/d)*100 }')"
custom_pct="$(awk -v n="$custom_only_cases" -v d="$total_cases" 'BEGIN { printf "%.2f", (n/d)*100 }')"

echo "source-composition: total_cases=$total_cases real_derived_cases=$real_derived_cases (${real_pct}%) custom_only_cases=$custom_only_cases (${custom_pct}%)"

# v1.0 targets from benchmark proposal:
# - >=80% cases include at least one real-derived source
# - <=20% cases are custom-only
if (( real_derived_cases * 100 < total_cases * 80 )); then
  echo "source-composition: FAIL real-derived ratio below 80%" >&2
  exit 1
fi
if (( custom_only_cases * 100 > total_cases * 20 )); then
  echo "source-composition: FAIL custom-only ratio above 20%" >&2
  exit 1
fi

echo "source-composition: PASS"
