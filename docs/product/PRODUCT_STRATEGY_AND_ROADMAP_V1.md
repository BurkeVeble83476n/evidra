# Product Strategy: Evidra Benchmark v0.3.0+

**Date:** March 4, 2026  
**Author:** Product Manager  
**Status:** Strategic Roadmap  
**Subject:** Category Creation and Adoption Acceleration

---

## 1. Executive Summary: Category Creation

The pivot from **OPA Enforcement** to the **Reliability Benchmark** model is not just a technical refactor; it is a category-defining shift. We are moving from being a "Judge" (blocking operations) to being a **"Flight Recorder"** (Behavioral Telemetry).

**The Problem:** Enterprises are dealing with non-deterministic infrastructure automation, including AI agents (Claude, Cursor, Devin), CI pipelines, and scripted deployers mutating production infrastructure. Current tools (Gatekeeper, Sentinel) are too rigid for these workflows.
**The Solution:** Evidra provides the first **"Credit Score for Automation."** We enable teams to observe, measure, and compare automation actors before granting them production privileges.

---

## 2. Strategic Value Propositions ("The Moat")

### 2.1 Canonicalization as Intellectual Property
Our biggest asset is not the score, but the **Canonicalization Contract**. Our ability to turn messy, tool-specific artifacts (K8s YAML, TF Plan JSON) into a stable, comparable `intent_digest` allows for cross-tool behavioral analysis that no competitor currently offers.

### 2.2 Low-Friction Entry Point
By being an "Inspector" rather than an "Enforcer," we bypass the #1 barrier to DevOps tool adoption: **The fear of breaking the pipeline.** Evidra never denies an action; it only records and scores. This allows security teams to run us in "Passive Mode" for 90 days to build a trust baseline.

---

## 3. Killer Features

### A. Integrity Check: Artifact Drift Detection
Automation actors (especially AI agents) often regenerate or mutate artifacts between their "prescribe" (intent) and "report" (execution) phases.
*   **Feature:** Flagging when the `artifact_digest` changes mid-lifecycle.
*   **Value:** "You promised a label change, but you applied a Security Group update." This is the first integrity-check for automation intent drift (including AI hallucinations).

### B. Procurement Shield: Model-vs-Model Benchmarking
*   **Feature:** Side-by-side reliability scorecards for different LLMs/Prompts using `actor_meta`.
*   **Value:** "Claude 3.5 is 14% more reliable than GPT-4o on our Terraform workloads." This makes Evidra a procurement tool for the C-Suite.

### C. hallucination Breaker: Retry-Loop Detection
*   **Feature:** Identifying identical `intent_digest` + `resource_shape_hash` failures.
*   **Value:** Stop burning compute and API tokens when an agent gets stuck in a loop trying to apply an invalid manifest.

### D. Compliance Evidence: Signed Scorecards
*   **Feature:** Cryptographically signed, hash-linked PDF scorecards.
*   **Value:** Agent vendors can provide these to enterprise buyers as "Proof of Reliability" for SOC2/ISO audits.

---

## 4. Adoption Gaps (What is missing?)

To achieve "Prometheus-level" adoption, we must bridge these gaps:

1.  **Effectiveness Metrics (Goodhart's Law Mitigation):** 
    *   *Risk:* If we only score Safety, agents will do nothing to stay at 100.
    *   *Requirement:* We must deliver an **Effectiveness Score** (Completion Rate, P95 Duration) alongside the Safety Score.
2.  **The "OPA Purge":** 
    *   *Risk:* Legacy terminology (`PolicyDecision`, `BundleRevision`) from the `evidra-mcp` reuse is still present in the Go structs.
    *   *Requirement:* Total semantic alignment. If a user sees "Policy" in a benchmark tool, they think "Friction" and leave.
3.  **Centralized "Fleet" Visibility:** 
    *   *Risk:* v0.3.0 is local-only (JSONL). SRE Managers cannot see their "fleet" of agents.
    *   *Requirement:* Accelerate **evidra-api** (v0.5.0) to provide a centralized dashboard.
4.  **SARIF Integration:** 
    *   *Risk:* We don't want to build 1,000 security rules.
    *   *Requirement:* Ingest SARIF from Checkov/Trivy so we can provide "Security Context" without building a proprietary scanner.

---

## 5. Tactical Recommendation Plan

### Phase 1: Hardening the Standard (v0.3.0)
*   **Purge Legacy Types:** Align all Go structs with `EVIDRA_CORE_DATA_MODEL.md`.
*   **Real Scorecard:** Transition the CLI from a stub to a real evidence scanner.
*   **Golden Path:** Create a "5-minute Hello World" for Terraform users.

### Phase 2: Distribution (v0.4.0)
*   **setup-evidra GitHub Action:** Make integration a single line of YAML.
*   **Python/TS SDKs:** Target the LangChain/Vercel AI SDK developer communities.

### Phase 3: The Platform (v0.5.0)
*   **Fleet API:** Centralized aggregation of JSONL evidence.
*   **Safety Floors:** Cap scores at 50 if a "Catastrophic Detector" (e.g., world-open ingress) fires.

---

## 6. Conclusion

Evidra Benchmark is the **"Black Box Flight Recorder"** for the AI-DevOps era. Our strategic direction is sound, but our implementation is currently "polluted" by OPA remnants. We must synchronize the code with the design contract immediately to ensure the foundation is ready for the v0.3.0 launch.

**I approve the Chief Architect's plan for a "Surgical Refactor" of the evidence types.**
