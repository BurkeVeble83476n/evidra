#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

MODEL_ID="${LITELLM_MODEL_ID:-}"
PROMPT_FILE="${EVIDRA_PROMPT_FILE:-$ROOT_DIR/prompts/experiments/litellm/system_instructions.txt}"
TEMPERATURE="${LITELLM_TEMPERATURE:-0}"

if [[ -z "$MODEL_ID" ]]; then
  echo "agent-cmd-litellm: FAIL set LITELLM_MODEL_ID (example: anthropic/claude-3-5-haiku)" >&2
  exit 1
fi

python3 "$ROOT_DIR/scripts/litellm-risk-agent.py" \
  --model-id "$MODEL_ID" \
  --artifact "${EVIDRA_ARTIFACT_PATH:?missing EVIDRA_ARTIFACT_PATH}" \
  --expected-json "${EVIDRA_EXPECTED_JSON:-}" \
  --output "${EVIDRA_AGENT_OUTPUT:?missing EVIDRA_AGENT_OUTPUT}" \
  --prompt-file "$PROMPT_FILE" \
  --temperature "$TEMPERATURE"
