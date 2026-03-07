# CTO Strategic Review: Evidra Benchmark

**Date:** March 4, 2026  
**Author:** CTO  
**Status:** BINDING TECHNICAL DIRECTION  
**Subject:** Technical Moat, Adoption Barriers, and Killer Features

---

## 1. The Strategic Bet: Telemetry over Enforcement

As CTO, I am officially retiring the "Policy Engine" model. Competing with Open Policy Agent (OPA) or Kyverno is a commodity race. Our strategic bet is **Behavioral Telemetry for Infrastructure Automation (including AI Agents).**

**The Core Thesis:** Infrastructure automation is high-impact and increasingly non-deterministic. You cannot "rule-base" every automation path into safety. You must "benchmark" automation behavior into trust. Evidra is the system that provides that benchmark.

---

## 2. Technical Moat: The Canonicalization ABI

Our **Canonicalization Contract** is our most defensible asset. 
*   **The Problem:** Terraform plans and K8s manifests change their byte-level structure on every tool update.
*   **Our Solution:** We provide a stable `intent_digest`. This is the **ABI (Application Binary Interface) for Infrastructure Operations.** 
*   **Action:** We must fix the current bug where `resource_shape_hash` is included in the `intent_digest`. This violates the contract and prevents us from tracking an agent's progress across iterative updates.

---

## 3. Killer Features (The "Why We Win")

### 3.1 Hallucination Detection (Artifact Drift)
We are the only tool that records what an automation actor *intended* to do versus what it *actually* did. If an AI agent hallucinates a change between the "Prescribe" and "Report" phase, we flag it. This is the "Seatbelt" for infrastructure automation.

### 3.2 Infinite Loop Suppression (Retry Signal)
Automation actors (especially AI agents) often get stuck trying the same failing command. By correlating `intent_digest` + `resource_shape_hash` over time, Evidra can trigger a system-level kill signal to stop compute-waste and potential infra-instability.

### 3.3 Zero-Privilege Security Model
Unlike Spacelift or Terraform Cloud, Evidra requires **zero infrastructure credentials.** It reads the artifact the agent provides. This makes us the easiest tool to "vendor-approve" in highly regulated enterprises (Banking, Healthcare).

---

## 4. Adoption Gaps (DX & CX)

To achieve industry standard status, we must address these gaps:

1.  **Semantic Alignment (P0):** The reused code from `evidra-mcp` is "polluted" with OPA terminology. We must purge `PolicyDecision` and `BundleRevision` from our Go structs. Developers will not adopt a "Benchmark" tool that looks like a "Policy" tool.
2.  **CLI Pipeline Maturity (P0):** The CLI scorecard is currently a stub. We must implement a high-performance JSONL scanner that can process 100,000 entries in < 2 seconds.
3.  **SARIF over Detectors (P1):** We will not build 500 risk detectors. We will build **one** SARIF parser. We consume the output of the industry's best scanners (Checkov, Trivy) and correlate their findings with our behavioral signals.

---

## 5. Technical Roadmap: v0.3.0 Hardening

*   **Foundation:** Unify `EvidenceEntry` structs. Remove all OPA dependencies.
*   **Integrity:** Implement `sha256:` prefixing across all digests.
*   **DX:** Create a `setup-evidra` GitHub Action.
*   **Standardization:** Freeze the 5 Core Signals and their default parameters (30m window, threshold 5).

---

## 6. Conclusion

Evidra Benchmark is the **Standard Telemetry Layer** for the next decade of infrastructure. We are building the "Prometheus for Behavior." I authorize the engineering team to prioritize **Data Model Integrity** above all else for the v0.3.0 release.

**Approved by:**  
*CTO, Evidra Benchmark Project*
