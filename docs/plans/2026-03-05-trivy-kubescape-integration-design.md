# Trivy + Kubescape Integration Design

Date: 2026-03-05  
Status: Approved (updated)

## 1. Context and Goal

The milestone goal is buyer confidence in pilots while preserving fast delivery and low maintenance.

Current architecture already defines:
- Evidra is not a scanner/policy engine.
- Evidra ingests SARIF from external scanners.
- One scanner ingestion contract (`--scanner-report`) should support multiple tools.

## 2. Recommendation

Adopt a **dual-default scanner strategy**:
- **Trivy** for Terraform/general IaC workflows.
- **Kubescape** for Kubernetes-focused workflows.

Decision:
- Official path: Trivy and Kubescape in quickstart and CI examples.
- Keep one scanner-agnostic SARIF ingestion path in core (`--scanner-report`).
- Avoid multi-scanner-by-default baselines beyond these two.

Why:
- Fast shipping and low maintenance.
- Strong enterprise story: one general scanner + one K8s-native scanner.
- Clear positioning without extra false-positive noise from additional defaults.

## 3. Evaluated Approaches

### A. Trivy-only default
Pros:
- Lowest complexity.
- Fewest moving parts in docs and CI.

Cons:
- Weaker Kubernetes-native story in security-heavy pilots.

### B. Trivy + Kubescape defaults (selected)
Pros:
- Covers broad IaC + Kubernetes concerns with two mature tools.
- Strong buyer confidence narrative.
- Still manageable operational complexity.

Cons:
- Slightly more docs/testing work than single-scanner default.

### C. Multi-scanner default set (>2 scanners)
Pros:
- Maximum coverage claims.

Cons:
- Higher CI cost/noise.
- More overlap and triage burden.
- Poor fit for current speed-to-value objective.

## 4. Architecture and Data Flow

### 4.1 Ingestion contract
- Keep one CLI interface: `--scanner-report <sarif-file>`.
- No per-scanner ingestion flags.
- No scanner-specific branching in core ingestion flow.

### 4.2 Canonical finding shape
For each SARIF result, normalize into:
- `tool`
- `rule_id`
- `severity` (`critical|high|medium|low|info`)
- `resource`
- `message`

### 4.3 Evidence linkage
- Findings are written as `finding` entries.
- Findings are linked by `artifact_digest` to the same operation context.
- Prescriptions/reports stay behavioral-first; scanner findings remain external risk context.

## 5. Risk and Scoring Behavior

- Scanner findings are context, not policy enforcement.
- High/critical findings may elevate prescription risk level.
- Findings remain available for longitudinal analysis (e.g., future risk-ignorance style signals).

## 6. Error Handling

- Missing/unreadable SARIF file: fail `prescribe` with explicit error.
- Invalid SARIF payload: fail fast; no partial parse success.
- Per-finding write failure: continue best-effort ingestion, report successful write count and warnings.

## 7. Testing Strategy

- Parser fixtures for Trivy and Kubescape SARIF samples.
- CLI integration tests for `--scanner-report` success/failure paths.
- Regression tests proving scanner-agnostic ingestion behavior across both defaults.
- Demo artifact for buyer confidence: both scanners feed the same contract.

## 8. Rollout Plan

1. Set Trivy and Kubescape as documented defaults in quickstart and CI examples.
2. Maintain scanner-agnostic SARIF parser behavior in core.
3. Publish buyer-facing matrix: “default scanners + single ingestion contract.”

## 9. Success Criteria (4-6 weeks)

Primary success metric: buyer confidence.

Measured by:
- Pilot materials show Trivy + Kubescape default workflows.
- Reproducible demo proving shared SARIF contract across both scanners.
- No scanner-specific integration code required in core ingestion path.

## 10. Non-goals

- Building proprietary scanner rules.
- Running scanners from inside Evidra core.
- Maintaining separate code paths for each scanner.
