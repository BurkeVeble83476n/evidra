<!-- contract: v1.3.0 -->

# Evidra Smart Prescribe Protocol

Every infrastructure **mutation** follows three steps:

1. `evidra_prescribe_smart` — record intent, get prescription_id
2. Execute the command
3. `evidra_report` — record outcome with prescription_id

Read-only operations (get, describe, logs, plan, show) skip the protocol.

**Diagnose first.** Use read-only commands to understand the problem before prescribing any mutation. Only call prescribe when you have identified the root cause and know what fix to apply.

## Rules

- Every infrastructure mutation must be recorded as evidence, either automatically via run_command or explicitly via prescribe/report.
- If you use prescribe_smart or prescribe_full, do not execute until the prescribe call returns ok=true with prescription_id.
- Every explicit prescribe must have exactly one report.
- Always include actor.skill_version (set to this contract version) on explicit protocol calls.

## Prescribe Smart

Call before every kubectl apply/delete/patch, helm upgrade/install, terraform apply/destroy.

```json
{
  "tool": "kubectl",
  "operation": "apply",
  "resource": "deployment/web",
  "namespace": "default",
  "actor": {"type": "agent", "id": "your-agent-id", "origin": "mcp-stdio", "skill_version": "v1.3.0"}
}
```

Required: tool, operation, resource, actor (type, id, origin).

## Report

Call after the command completes:

```json
{
  "prescription_id": "<from prescribe>",
  "verdict": "success",
  "exit_code": 0
}
```

Verdicts: success (exit 0), failure (non-zero exit), declined (chose not to execute).

## What Requires Protocol

| Tool | Mutating operations |
|------|-------------------|
| kubectl | apply, delete, patch, create, replace, rollout restart |
| helm | install, upgrade, uninstall, rollback |
| terraform | apply, destroy, import |

When unsure, call prescribe. The cost is one extra call. Skipping triggers a protocol violation.
