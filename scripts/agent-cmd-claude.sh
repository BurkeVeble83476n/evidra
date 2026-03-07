#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

MODEL_ID="${EVIDRA_MODEL_ID:-${CLAUDE_MODEL_ID:-}}"
PROMPT_FILE="${EVIDRA_PROMPT_FILE:-$ROOT_DIR/prompts/experiments/runtime/system_instructions.txt}"
PYTHON_BIN="${CLAUDE_PYTHON_BIN:-}"

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
  echo "agent-cmd-claude: FAIL set EVIDRA_MODEL_ID (example: claude/haiku)" >&2
  exit 1
fi

if ! command -v claude >/dev/null 2>&1; then
  echo "agent-cmd-claude: FAIL 'claude' CLI not found in PATH." >&2
  echo "Install and login to Claude Code CLI before running this wrapper." >&2
  exit 1
fi

"$PYTHON_BIN" "$ROOT_DIR/scripts/claude-risk-agent.py" \
  --model-id "$MODEL_ID" \
  --artifact "${EVIDRA_ARTIFACT_PATH:?missing EVIDRA_ARTIFACT_PATH}" \
  --expected-json "${EVIDRA_EXPECTED_JSON:-}" \
  --output "${EVIDRA_AGENT_OUTPUT:?missing EVIDRA_AGENT_OUTPUT}" \
  --prompt-file "$PROMPT_FILE" \
  --raw-stream-out "${EVIDRA_AGENT_RAW_STREAM:-}"
