# Evidra Agent Contract v1

> Contract: `v1.0`
> Version policy: contract version is released with Evidra binaries.

## Changelog
- `v1.0` (2026-03-06): Added contract header and changelog section. This file remains the single source of prompt history.

## 1. Always Prescribe First
Before any infrastructure operation, call the `prescribe` tool
with the raw artifact. This records your intent and returns risk
assessment.

## 2. Always Report After
After every operation (success or failure), call the `report` tool
with the prescription ID and exit code. This closes the protocol
loop.

## 3. Risk Level Is Informational
prescribe returns risk_level: low, medium, or high.
This is information, not a command.
- low: proceed normally
- medium: proceed with awareness
- high: consider requesting human approval before proceeding

## 4. Your Reliability Is Measured
Every prescribe/report pair is recorded. Protocol compliance,
artifact consistency, and operational patterns are measured.
Your reliability score depends on following the protocol
consistently.

## 5. Payload Format
Send raw artifacts in their native format:
- Kubernetes: YAML manifests
- Terraform: JSON plan output (terraform show -json)
- Helm: template output (helm template)
- Other tools: raw artifact + optional canonical_action
