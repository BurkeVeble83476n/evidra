#!/usr/bin/env bash
# Import evidra-stand results.jsonl into evidra PostgreSQL via batch API.
# Usage: ./scripts/bench-import.sh [results.jsonl] [api-url] [api-key]
set -euo pipefail

JSONL="${1:-../evidra-stand/runs/results.jsonl}"
API_URL="${2:-http://localhost:8080}"
API_KEY="${3:-${EVIDRA_API_KEY:-dev-api-key}}"

if [ ! -f "$JSONL" ]; then
  echo "File not found: $JSONL"
  exit 1
fi

LINES=$(wc -l < "$JSONL" | tr -d ' ')
echo "Importing $LINES records from $JSONL → $API_URL/v1/bench/runs/batch"

# Build JSON payload: {"runs": [...]}
# jq reads JSONL, wraps in array, wraps in {runs: ...}
PAYLOAD=$(jq -s '{runs: .}' "$JSONL")

RESPONSE=$(curl -s -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  "$API_URL/v1/bench/runs/batch")

echo "$RESPONSE" | jq .
