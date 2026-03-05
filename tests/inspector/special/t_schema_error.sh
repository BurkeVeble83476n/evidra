#!/usr/bin/env bash

BAD_INPUT='{"actor":{"type":"agent","id":"bad","origin":"mcp"},"tool":"kubectl","operation":"apply"}'

case "$MODE" in
  local-mcp)
    raw_output=$(inspector_call_tool "prescribe" "$BAD_INPUT" "" "1" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/jsonrpc_-32602"
    else
      fail "schema_error/jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    if echo "$raw_output" | grep -qi "invalid params"; then
      pass "schema_error/invalid_params_text"
    else
      fail "schema_error/invalid_params_text" "missing invalid params text"
    fi
    ;;
  hosted-mcp)
    raw_output=$(_hosted_call_tool_raw "prescribe" "$BAD_INPUT" 2>&1 || true)
    if echo "$raw_output" | grep -q -- "-32602"; then
      pass "schema_error/jsonrpc_-32602"
    else
      fail "schema_error/jsonrpc_-32602" "expected -32602, got: $raw_output"
    fi
    ;;
  local-rest|hosted-rest)
    body=$(rest_post_json "/v1/prescribe" "$BAD_INPUT") || {
      fail "schema_error/rest_call" "REST call failed"
      return
    }
    http_code=$(echo "$body" | jq -r '._http_code // 0')
    if [[ "$http_code" == "400" ]]; then
      pass "schema_error/http_400"
    else
      fail "schema_error/http_400" "expected 400, got $http_code"
    fi
    ;;
esac
