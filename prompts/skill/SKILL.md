---
name: evidra
description: "Use this skill when the user asks you to perform DevOps operations — diagnosing, fixing, or managing infrastructure using kubectl, helm, terraform, or aws commands. This includes: running kubectl get/describe/logs/apply/delete/patch, helm install/upgrade/status/list, terraform plan/apply/destroy/import, aws CLI commands, or any infrastructure investigation and remediation workflow. Also trigger when the user mentions Evidra, evidence chains, prescribe/report protocol, reliability scorecards, or run_command. DO NOT trigger for: writing Dockerfiles, writing Ansible/Terraform code without executing it, CI/CD pipeline setup, explaining infrastructure concepts, or writing tests for infrastructure tools. The key distinction is OPERATING infrastructure (both reading and mutating) vs WRITING code."
---
<!-- contract: v1.1.0 -->

# Evidra — DevOps MCP Server

evidra-mcp is your infrastructure toolkit. Use `run_command` for kubectl, helm, terraform, and aws operations.

## Smart Output

Responses are token-efficient summaries, not raw JSON. Trust them.

## Diagnosis Protocol

- Investigate before fixing: kubectl describe, get events, check logs
- Make one targeted fix, verify it worked, then stop
- Don't patch resources you didn't investigate

## Safety

- Never delete resources outside the problem scope
- Verify fixes with kubectl get or rollout status
- Check what exists before creating (avoid duplicates)

## Evidence Recording

Every infrastructure mutation is automatically recorded via `run_command`.
For explicit control, use `prescribe_smart` or `prescribe_full` before mutations.

### When to prescribe explicitly

- Mutations: kubectl apply/delete/patch, helm install/upgrade, terraform apply/destroy
- Skip for: kubectl get/describe/logs, helm list, terraform plan (read-only)
- When unsure: prescribe (safe default)

### prescribe_smart (recommended)

Call with: tool, operation, resource, namespace, actor.
Returns: prescription_id, effective_risk.

```json
{
  "tool": "kubectl", "operation": "apply",
  "resource": "deployment/web", "namespace": "default",
  "actor": {"type": "agent", "id": "your-id", "origin": "mcp-stdio", "skill_version": "1.1.0"}
}
```

### prescribe_full

Call with: tool, operation, raw_artifact, actor.
Use when you have the full YAML/manifest and want drift detection.

### report (after every mutation)

Call with: prescription_id, verdict (success/failure/declined), exit_code, actor.
On retry: new prescribe, execute, new report (each attempt is a pair).

### Critical Rules

- Every prescribe needs exactly one report
- Include actor.skill_version = "1.1.0"
- Report failures honestly (non-zero exit_code)
- Declined verdicts use decision_context, not exit_code

## Risk Tags

| Tag | Severity |
|-----|----------|
| `k8s.privileged_container` | critical |
| `k8s.cluster_admin_binding` | critical |
| `ops.mass_delete` | critical |
| `k8s.hostpath_mount`, `k8s.run_as_root`, `k8s.host_namespace_escape` | high |
| `k8s.docker_socket`, `k8s.dangerous_capabilities`, `k8s.writable_rootfs` | high |
| `ops.kube_system`, `ops.namespace_delete` | high |
| `aws_iam.wildcard_policy` | critical |
| `terraform.s3_public_access`, `aws.security_group_open` | high |

## Behavioral Signals

protocol_violation, artifact_drift, retry_loop, blast_radius, new_scope, repair_loop, thrashing, risk_escalation
