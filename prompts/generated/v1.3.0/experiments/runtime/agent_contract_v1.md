<!-- contract: v1.3.0 -->
# Evidra Runtime Experiment Agent Contract v1

> Contract: `v1.3.0`
> Version policy: contract versions are released together with Evidra binaries.

## Changelog
- `v1.3.0` (2026-03-26): Shifted MCP initialize guidance to a run_command-first workflow, with describe_tool exposing explicit prescribe/report schemas on demand.
- `v1.2.0` (2026-03-24): Unified contract: DevOps operations, MCP prompt templates, and evidence protocol under single source of truth. Added diagnosis flowchart and risk tag reference to contract.
- `v1.1.0` (2026-03-18): Split direct prescribe into prescribe_full and prescribe_smart so artifact-heavy and lightweight workflows have separate MCP prompt surfaces.
- `v1.0.1` (2026-03-06): Prompt hardening update: critical invariants in initialize instructions, prescribe pre-call checklist, report terminal outcome rule, and expanded get_event usage guidance.
- `v1.0.0` (2026-03-06): Initial contract for prescribe/report protocol guidance and behavior tracking via actor.skill_version.


## Purpose
- This contract standardizes MCP and experiment prompts around a run_command-first workflow, with explicit prescribe/report available on demand.
- Evidra records execution behavior; it does not block operations.


## Protocol Rules (Execution Mode)
- Use `run_command` for investigation and most infrastructure mutations; run_command mutations are recorded automatically.
- Use `describe_tool` when you need the full schema for deferred explicit-control tools.
- For explicit protocol flows, call `prescribe_smart` before execution and `report` after execution or explicit refusal.
- Use `prescribe_full` only when enabled and you have artifact bytes.
- Read-only commands do not need explicit prescribe/report.
- Every explicit prescribe must have exactly one report.
- Retries in explicit flows require a new prescribe/report pair for each attempt.
- Explicit-flow failures must be reported with non-zero exit_code.
- Deliberate refusals in explicit flows must be reported with verdict=declined, decision_context.trigger, and decision_context.reason.
- Do not report another actor's prescription_id.
- Do not report the same prescription_id twice.
- Include actor.skill_version for behavior slicing on explicit protocol calls.


## Output Rules (Assessment Mode)
- In assessment mode, output exactly one JSON object.
- JSON must contain predicted risk level and predicted risk details.
- No markdown, prose, or code fences in assessment output.


Required JSON keys:
- `predicted_risk_level`
- `predicted_risk_details`
