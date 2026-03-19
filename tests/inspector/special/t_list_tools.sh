#!/usr/bin/env bash

raw_tools=$(call_list_tools) || {
  fail "list_tools" "tools/list failed"
  return
}

for t in prescribe_full prescribe_smart report get_event; do
  has_tool=$(echo "$raw_tools" | jq --arg t "$t" '[.tools[]? | select(.name == $t)] | length')
  if [[ "$has_tool" -gt 0 ]]; then
    pass "list_tools/${t}_registered"
  else
    fail "list_tools/${t}_registered" "tool not found"
  fi
done

legacy_prescribe=$(echo "$raw_tools" | jq --arg t "prescribe" '[.tools[]? | select(.name == $t)] | length')
if [[ "$legacy_prescribe" == "0" ]]; then
  pass "list_tools/legacy_prescribe_absent"
else
  fail "list_tools/legacy_prescribe_absent" "legacy prescribe tool should not be registered"
fi

prescribe_full_required=$(echo "$raw_tools" | jq -c '[.tools[]? | select(.name == "prescribe_full")][0].inputSchema.required // []')
for field in tool operation raw_artifact actor; do
  found=$(echo "$prescribe_full_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
  if [[ "$found" -gt 0 ]]; then
    pass "list_tools/prescribe_full_requires_${field}"
  else
    fail "list_tools/prescribe_full_requires_${field}" "missing required field"
  fi
done

prescribe_smart_required=$(echo "$raw_tools" | jq -c '[.tools[]? | select(.name == "prescribe_smart")][0].inputSchema.required // []')
for field in tool operation resource actor; do
  found=$(echo "$prescribe_smart_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
  if [[ "$found" -gt 0 ]]; then
    pass "list_tools/prescribe_smart_requires_${field}"
  else
    fail "list_tools/prescribe_smart_requires_${field}" "missing required field"
  fi
done

report_required=$(echo "$raw_tools" | jq -c '[.tools[]? | select(.name == "report")][0].inputSchema.required // []')
for field in prescription_id verdict; do
  found=$(echo "$report_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
  if [[ "$found" -gt 0 ]]; then
    pass "list_tools/report_requires_${field}"
  else
    fail "list_tools/report_requires_${field}" "missing required field"
  fi
done

get_event_required=$(echo "$raw_tools" | jq -c '[.tools[]? | select(.name == "get_event")][0].inputSchema.required // []')
found=$(echo "$get_event_required" | jq '[.[]? | select(. == "event_id")] | length')
if [[ "$found" -gt 0 ]]; then
  pass "list_tools/get_event_requires_event_id"
else
  fail "list_tools/get_event_requires_event_id" "missing required field"
fi
