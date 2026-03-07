#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MCP_CONFIG="${EVIDRA_MCP_CONFIG:-$ROOT_DIR/tests/inspector/mcp-config.json}"
MCP_SERVER="${EVIDRA_MCP_SERVER:-evidra}"
MCP_ENV_LABEL="${EVIDRA_ENVIRONMENT:-}"
RAW_STREAM_OUT="${EVIDRA_AGENT_RAW_STREAM:-}"
INSPECTOR_BIN="${EVIDRA_MCP_INSPECTOR_BIN:-npx}"

TOOL="${EVIDRA_EXEC_TOOL:?missing EVIDRA_EXEC_TOOL}"
OPERATION="${EVIDRA_EXEC_OPERATION:?missing EVIDRA_EXEC_OPERATION}"
ARTIFACT_PATH="${EVIDRA_EXEC_ARTIFACT:?missing EVIDRA_EXEC_ARTIFACT}"
EXECUTE_CMD="${EVIDRA_EXEC_COMMAND:?missing EVIDRA_EXEC_COMMAND}"
OUTPUT_PATH="${EVIDRA_AGENT_OUTPUT:?missing EVIDRA_AGENT_OUTPUT}"

ACTOR_TYPE="${EVIDRA_ACTOR_TYPE:-agent}"
ACTOR_ID="${EVIDRA_ACTOR_ID:-${EVIDRA_MODEL_ID:-execution-agent}}"
ACTOR_ORIGIN="${EVIDRA_ACTOR_ORIGIN:-execution-runner}"
ACTOR_SKILL_VERSION="${EVIDRA_PROMPT_CONTRACT_VERSION:-unknown}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "agent-cmd-mcp-kubectl: FAIL missing required command: $1" >&2
    exit 1
  }
}

write_raw_event() {
  local phase="$1" payload="$2"
  [[ -n "$RAW_STREAM_OUT" ]] || return 0
  mkdir -p "$(dirname "$RAW_STREAM_OUT")"
  jq -n --arg phase "$phase" --arg payload "$payload" '{phase:$phase,payload:$payload}' >>"$RAW_STREAM_OUT"
}

inspector_call_tool() {
  local tool="$1" args_json="$2"
  local -a cmd=(
    "$INSPECTOR_BIN" -y @modelcontextprotocol/inspector --cli
    --config "$MCP_CONFIG" --server "$MCP_SERVER"
  )

  if [[ -n "$MCP_ENV_LABEL" ]]; then
    cmd+=( -e "EVIDRA_ENVIRONMENT=${MCP_ENV_LABEL}" )
  fi

  cmd+=( --method tools/call --tool-name "$tool" )

  while IFS= read -r key; do
    local value_type val
    value_type="$(echo "$args_json" | jq -r --arg k "$key" '.[$k] | type')"
    if [[ "$value_type" == "string" ]]; then
      val="$(echo "$args_json" | jq -r --arg k "$key" '.[$k]')"
    else
      val="$(echo "$args_json" | jq -c --arg k "$key" '.[$k]')"
    fi
    cmd+=( --tool-arg "${key}=${val}" )
  done < <(echo "$args_json" | jq -r 'keys[]')

  "${cmd[@]}" 2>/dev/null
}

extract_body() {
  jq '.structuredContent // (.content[0].text | fromjson) // .' 2>/dev/null
}

ensure_evidra_mcp() {
  if command -v evidra-mcp >/dev/null 2>&1; then
    return 0
  fi

  if [[ -x "$ROOT_DIR/bin/evidra-mcp" ]]; then
    export PATH="$ROOT_DIR/bin:$PATH"
    return 0
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "agent-cmd-mcp-kubectl: FAIL evidra-mcp not found and go is unavailable to build it." >&2
    exit 1
  fi

  (cd "$ROOT_DIR" && go build -o bin/evidra-mcp ./cmd/evidra-mcp)
  export PATH="$ROOT_DIR/bin:$PATH"
}

require_cmd jq
require_cmd "$INSPECTOR_BIN"
require_cmd bash
require_cmd kubectl
ensure_evidra_mcp

[[ -f "$ARTIFACT_PATH" ]] || {
  echo "agent-cmd-mcp-kubectl: FAIL artifact not found: $ARTIFACT_PATH" >&2
  exit 1
}

[[ -f "$MCP_CONFIG" ]] || {
  echo "agent-cmd-mcp-kubectl: FAIL MCP config not found: $MCP_CONFIG" >&2
  exit 1
}

export EVIDRA_SIGNING_MODE="${EVIDRA_SIGNING_MODE:-optional}"
raw_artifact="$(cat "$ARTIFACT_PATH")"

prescribe_args="$(
  jq -n \
    --arg tool "$TOOL" \
    --arg operation "$OPERATION" \
    --arg raw_artifact "$raw_artifact" \
    --arg actor_type "$ACTOR_TYPE" \
    --arg actor_id "$ACTOR_ID" \
    --arg actor_origin "$ACTOR_ORIGIN" \
    --arg actor_skill_version "$ACTOR_SKILL_VERSION" \
    '{
      tool:$tool,
      operation:$operation,
      raw_artifact:$raw_artifact,
      actor:{
        type:$actor_type,
        id:$actor_id,
        origin:$actor_origin,
        skill_version:$actor_skill_version
      }
    }'
)"

if ! prescribe_resp="$(inspector_call_tool prescribe "$prescribe_args")"; then
  echo "agent-cmd-mcp-kubectl: FAIL prescribe call failed" >&2
  exit 1
fi

write_raw_event "prescribe" "$prescribe_resp"
prescribe_body="$(echo "$prescribe_resp" | extract_body)"
prescribe_ok="$(echo "$prescribe_body" | jq -c '.ok // false')"

if [[ "$prescribe_ok" != "true" ]]; then
  jq -n \
    --argjson prescribe_ok false \
    --argjson report_ok false \
    --arg tool "$TOOL" \
    --arg operation "$OPERATION" \
    --arg artifact_path "$ARTIFACT_PATH" \
    --arg execute_cmd "$EXECUTE_CMD" \
    --arg response "$prescribe_body" \
    '{
      prescribe_ok:$prescribe_ok,
      report_ok:$report_ok,
      tool:$tool,
      operation:$operation,
      artifact_path:$artifact_path,
      execute_cmd:$execute_cmd,
      error_phase:"prescribe",
      response:$response
    }' >"$OUTPUT_PATH"
  echo "agent-cmd-mcp-kubectl: FAIL prescribe returned ok=false" >&2
  exit 1
fi

prescription_id="$(echo "$prescribe_body" | jq -r '.prescription_id // ""')"
risk_level="$(echo "$prescribe_body" | jq -r '.risk_level // "unknown"')"
risk_tags="$(echo "$prescribe_body" | jq -c '.risk_tags // []')"

[[ -n "$prescription_id" ]] || {
  echo "agent-cmd-mcp-kubectl: FAIL prescribe response missing prescription_id" >&2
  exit 1
}

set +e
bash -lc "$EXECUTE_CMD"
command_exit=$?
set -e

report_args="$(
  jq -n \
    --arg prescription_id "$prescription_id" \
    --argjson exit_code "$command_exit" \
    --arg actor_type "$ACTOR_TYPE" \
    --arg actor_id "$ACTOR_ID" \
    --arg actor_origin "$ACTOR_ORIGIN" \
    --arg actor_skill_version "$ACTOR_SKILL_VERSION" \
    '{
      prescription_id:$prescription_id,
      exit_code:$exit_code,
      actor:{
        type:$actor_type,
        id:$actor_id,
        origin:$actor_origin,
        skill_version:$actor_skill_version
      }
    }'
)"

if ! report_resp="$(inspector_call_tool report "$report_args")"; then
  jq -n \
    --argjson prescribe_ok true \
    --argjson report_ok false \
    --argjson exit_code "$command_exit" \
    --arg prescription_id "$prescription_id" \
    --arg risk_level "$risk_level" \
    --argjson risk_tags "$risk_tags" \
    '{
      prescribe_ok:$prescribe_ok,
      report_ok:$report_ok,
      exit_code:$exit_code,
      prescription_id:$prescription_id,
      risk_level:$risk_level,
      risk_tags:$risk_tags,
      error_phase:"report"
    }' >"$OUTPUT_PATH"
  echo "agent-cmd-mcp-kubectl: FAIL report call failed" >&2
  exit 1
fi

write_raw_event "report" "$report_resp"
report_body="$(echo "$report_resp" | extract_body)"
report_ok="$(echo "$report_body" | jq -c '.ok // false')"
report_id="$(echo "$report_body" | jq -r '.report_id // ""')"

jq -n \
  --argjson prescribe_ok true \
  --argjson report_ok "$report_ok" \
  --argjson exit_code "$command_exit" \
  --arg prescription_id "$prescription_id" \
  --arg report_id "$report_id" \
  --arg risk_level "$risk_level" \
  --argjson risk_tags "$risk_tags" \
  --arg tool "$TOOL" \
  --arg operation "$OPERATION" \
  --arg artifact_path "$ARTIFACT_PATH" \
  --arg execute_cmd "$EXECUTE_CMD" \
  --arg report_response "$report_body" \
  '{
    prescribe_ok:$prescribe_ok,
    report_ok:$report_ok,
    exit_code:$exit_code,
    prescription_id:$prescription_id,
    report_id:$report_id,
    risk_level:$risk_level,
    risk_tags:$risk_tags,
    tool:$tool,
    operation:$operation,
    artifact_path:$artifact_path,
    execute_cmd:$execute_cmd,
    report_response:$report_response
  }' >"$OUTPUT_PATH"

if [[ "$report_ok" != "true" ]]; then
  echo "agent-cmd-mcp-kubectl: FAIL report returned ok=false" >&2
  exit 1
fi

exit 0
