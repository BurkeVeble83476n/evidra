#!/usr/bin/env bash

raw_tools=$(call_list_tools) || {
  fail "list_tools" "tools/list failed"
  return
}

for t in run_command collect_diagnostics prescribe_full prescribe_smart report get_event; do
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

run_command_tool=$(tool_from_list "$raw_tools" "run_command")
if [[ "$(echo "$run_command_tool" | jq -r '.description // empty')" == *"Investigate before fixing"* ]]; then
  pass "list_tools/run_command_description_guidance"
else
  fail "list_tools/run_command_description_guidance" "missing diagnosis guidance"
fi

if [[ "$(echo "$run_command_tool" | jq -r '.description // empty')" == *"kubectl rollout status"* ]]; then
  pass "list_tools/run_command_description_examples"
else
  fail "list_tools/run_command_description_examples" "missing verify example"
fi

run_command_output_required=$(echo "$run_command_tool" | jq -c '.outputSchema.required // []')
for field in ok output exit_code mutation; do
  found=$(echo "$run_command_output_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
  if [[ "$found" -gt 0 ]]; then
    pass "list_tools/run_command_output_requires_${field}"
  else
    fail "list_tools/run_command_output_requires_${field}" "missing required field"
  fi
done

collect_diagnostics_required=$(tool_from_list "$raw_tools" "collect_diagnostics" | jq -c '.inputSchema.required // []')
for field in namespace workload; do
  found=$(echo "$collect_diagnostics_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
  if [[ "$found" -gt 0 ]]; then
    pass "list_tools/collect_diagnostics_requires_${field}"
  else
    fail "list_tools/collect_diagnostics_requires_${field}" "missing required field"
  fi
done

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
