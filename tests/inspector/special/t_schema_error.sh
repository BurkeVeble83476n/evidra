#!/usr/bin/env bash

BAD_SMART_INPUT='{"actor":{"type":"agent","id":"bad","origin":"mcp"},"tool":"kubectl","operation":"apply"}'
BAD_FULL_INPUT='{"actor":{"type":"agent","id":"bad","origin":"mcp"},"tool":"kubectl","operation":"apply"}'

case "$MODE" in
  local-mcp)
    raw_output=$(inspector_call_tool "prescribe_smart" "$BAD_SMART_INPUT" "" "1" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/prescribe_smart_jsonrpc_-32602"
    else
      fail "schema_error/prescribe_smart_jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    if echo "$raw_output" | grep -qi "invalid params"; then
      pass "schema_error/prescribe_smart_invalid_params_text"
    else
      fail "schema_error/prescribe_smart_invalid_params_text" "missing invalid params text"
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
    raw_output=$(_hosted_call_tool_raw "prescribe_smart" "$BAD_SMART_INPUT" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/prescribe_smart_jsonrpc_-32602"
    else
      fail "schema_error/prescribe_smart_jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi

    raw_output=$(_hosted_call_tool_raw "prescribe_full" "$BAD_FULL_INPUT" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/prescribe_full_jsonrpc_-32602"
    else
      fail "schema_error/prescribe_full_jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    ;;
esac
