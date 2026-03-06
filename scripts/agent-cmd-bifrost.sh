#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

MODEL_ID="${EVIDRA_MODEL_ID:-${BIFROST_MODEL_ID:-}}"
PROMPT_FILE="${EVIDRA_PROMPT_FILE:-$ROOT_DIR/prompts/experiments/runtime/system_instructions.txt}"
BASE_URL="${EVIDRA_BIFROST_BASE_URL:-http://localhost:8080/openai}"
PYTHON_BIN="${BIFROST_PYTHON_BIN:-}"

if [[ -z "$PYTHON_BIN" ]]; then
  if [[ -n "${VIRTUAL_ENV:-}" && -x "${VIRTUAL_ENV}/bin/python3" ]]; then
    PYTHON_BIN="${VIRTUAL_ENV}/bin/python3"
  elif [[ -x "${ROOT_DIR}/.venv/bin/python3" ]]; then
    PYTHON_BIN="${ROOT_DIR}/.venv/bin/python3"
  else
    PYTHON_BIN="python3"
  fi
fi

if [[ -z "$MODEL_ID" ]]; then
  echo "agent-cmd-bifrost: FAIL set EVIDRA_MODEL_ID (example: anthropic/claude-3-5-haiku)" >&2
  exit 1
fi

cmd=(
  "$PYTHON_BIN" "$ROOT_DIR/scripts/bifrost-risk-agent.py"
  --model-id "$MODEL_ID"
  --artifact "${EVIDRA_ARTIFACT_PATH:?missing EVIDRA_ARTIFACT_PATH}"
  --expected-json "${EVIDRA_EXPECTED_JSON:-}"
  --output "${EVIDRA_AGENT_OUTPUT:?missing EVIDRA_AGENT_OUTPUT}"
  --prompt-file "$PROMPT_FILE"
  --base-url "$BASE_URL"
)

if [[ -n "${EVIDRA_BIFROST_VK:-}" ]]; then
  cmd+=(--bifrost-vk "$EVIDRA_BIFROST_VK")
fi

if [[ -n "${EVIDRA_BIFROST_AUTH_BEARER:-}" ]]; then
  cmd+=(--auth-bearer "$EVIDRA_BIFROST_AUTH_BEARER")
fi

if [[ -n "${EVIDRA_BIFROST_EXTRA_HEADERS_JSON:-}" ]]; then
  cmd+=(--extra-headers-json "$EVIDRA_BIFROST_EXTRA_HEADERS_JSON")
fi

"${cmd[@]}"
