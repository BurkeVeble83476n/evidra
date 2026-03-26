#!/usr/bin/env bash

BAD_SMART_INPUT='{"actor":{"type":"agent","id":"bad","origin":"mcp"},"tool":"kubectl","operation":"apply"}'
BAD_FULL_INPUT='{"actor":{"type":"agent","id":"bad","origin":"mcp"},"tool":"kubectl","operation":"apply"}'

case "$MODE" in
  local-mcp)
    smart_body=$(call_named_tool "prescribe_smart" "$BAD_SMART_INPUT" "" || true)
    if [[ "$(echo "$smart_body" | jq -r '.error.code // empty')" == "invalid_input" ]]; then
      pass "schema_error/prescribe_smart_invalid_input"
    else
      fail "schema_error/prescribe_smart_invalid_input" "expected error.code=invalid_input, got: $smart_body"
    fi
    if [[ "$(echo "$smart_body" | jq -r '.error.message // empty')" == *"one of raw_artifact, resource, or canonical_action is required"* ]]; then
      pass "schema_error/prescribe_smart_missing_resource_message"
    else
      fail "schema_error/prescribe_smart_missing_resource_message" "missing validation message"
    fi

    raw_output=$(inspector_call_tool "prescribe_full" "$BAD_FULL_INPUT" "" "1" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/prescribe_full_jsonrpc_-32602"
    else
      fail "schema_error/prescribe_full_jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    if echo "$raw_output" | grep -qi "invalid params"; then
      pass "schema_error/prescribe_full_invalid_params_text"
    else
      fail "schema_error/prescribe_full_invalid_params_text" "missing invalid params text"
    fi
    ;;
  hosted-mcp)
    smart_body=$(call_named_tool "prescribe_smart" "$BAD_SMART_INPUT" "" || true)
    if [[ "$(echo "$smart_body" | jq -r '.error.code // empty')" == "invalid_input" ]]; then
      pass "schema_error/prescribe_smart_invalid_input"
    else
      fail "schema_error/prescribe_smart_invalid_input" "expected error.code=invalid_input, got: $smart_body"
    fi
    if [[ "$(echo "$smart_body" | jq -r '.error.message // empty')" == *"one of raw_artifact, resource, or canonical_action is required"* ]]; then
      pass "schema_error/prescribe_smart_missing_resource_message"
    else
      fail "schema_error/prescribe_smart_missing_resource_message" "missing validation message"
    fi

    raw_output=$(_hosted_call_tool_raw "prescribe_full" "$BAD_FULL_INPUT" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/prescribe_full_jsonrpc_-32602"
    else
      fail "schema_error/prescribe_full_jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    ;;
esac
