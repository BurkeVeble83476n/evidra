# EVIDRA Prompt Factory Spec (Single Source of Truth)

**Status:** Implemented baseline (`v1.0.1`)  
**Date:** 2026-03-06  
**Scope:** Prompt contract source, folder structure, generation targets, versioning, CI gates

---

## 1. Problem

Evidra currently has multiple prompt surfaces:
- MCP server prompts (`prompts/mcpserver/*`)
- Runtime experiment prompts (`prompts/experiments/runtime/*`)
- framework-specific integrations (future)

If these prompts evolve independently, protocol behavior drifts and experiment results become non-comparable.

We need one canonical prompt contract artifact and deterministic generation to all target surfaces.

---

## 2. Design Goals

1. One source of truth for prompt behavior and contract semantics.
2. Deterministic generation for different agents, runtimes, and protocols.
3. Shared contract version across generated outputs.
4. Explicit traceability: every generated prompt maps to one source contract artifact.
5. Backward compatibility with existing runtime file paths.

---

## 3. Non-Goals

1. This spec does not change wire protocol fields.
2. This spec does not require immediate migration of all existing prompts in one PR.
3. This spec does not force one "universal wording" for every target; only shared semantics are mandatory.

---

## 4. Canonical Prompt Source Model

Canonical source is a versioned contract artifact:

- Contract semantics (normative rules/invariants)
- Command classification (mutate vs read-only)
- Required failure-path rules
- Output contract requirements (when target requires strict JSON output)
- Target template bindings

Generated prompts are artifacts, not hand-authored source.

---

## 5. Folder Structure

```text
prompts/
  source/                                  # single source of truth
    contracts/
      v1.0.1/
        CONTRACT.yaml                      # canonical rules and invariants
        CLASSIFICATION.yaml                # mutate/read-only command taxonomy
        OUTPUT_CONTRACTS.yaml              # strict output schemas by target mode
        CHANGELOG.md
        templates/
          mcp/
            initialize.tmpl
            prescribe.tmpl
            report.tmpl
            get_event.tmpl
            agent_contract.tmpl
          runtime/
            system_instructions.tmpl
            agent_contract.tmpl

  generated/                               # generated artifacts (never manual source)
    v1.0.1/
      mcpserver/
        initialize/instructions.txt
        tools/prescribe_description.txt
        tools/report_description.txt
        tools/get_event_description.txt
        resources/content/agent_contract_v1.md
      experiments/
        runtime/
          system_instructions.txt
          agent_contract_v1.md

  manifests/
    v1.0.1.json                            # hash manifest for generated outputs

  mcpserver/                               # active runtime location (compat)
  experiments/runtime/                     # active runtime location (compat)
```

Compatibility rule:
- Runtime continues reading current active paths (`prompts/mcpserver/*`, `prompts/experiments/runtime/*`).
- Generation pipeline MUST materialize outputs into those active paths.

---

## 6. Generation Contract

Inputs:
1. `CONTRACT.yaml`
2. `CLASSIFICATION.yaml`
3. `OUTPUT_CONTRACTS.yaml`
4. target templates
5. profile (optional)

Outputs:
1. Generated prompt files for each target surface.
2. Manifest with file hashes.
3. Metadata with `contract_version`.

Determinism requirements:
1. Stable ordering of sections.
2. Stable newline formatting.
3. No timestamps embedded in generated prompt text.
4. Same input must produce byte-identical output.

---

## 7. Versioning and Semantics

1. Contract version uses semver (`vMAJOR.MINOR.PATCH`).
2. Every generated prompt starts with a contract header:
   - text: `# contract: vX.Y.Z`
   - markdown: `<!-- contract: vX.Y.Z -->`
3. `actor.skill_version` must map to this contract version.

Version bump rules:
1. Patch: wording clarifications, no new required behavior.
2. Minor: new guidance that may affect behavior but not wire schema.
3. Major: required behavior change or protocol contract change.

---

## 8. Target Mapping (Minimum)

### MCP

Generated files:
- `initialize/instructions.txt`
- `tools/{prescribe,report,get_event}_description.txt`
- `resources/content/agent_contract_v1.md`

Must preserve:
- mutate/read-only boundary
- prescribe/report invariants
- retry and failure-path rules
- `actor.skill_version` guidance

### Runtime Harness (Artifact Assessment + Execution Harness)

Generated files:
- `system_instructions.txt`
- `agent_contract_v1.md`

Must preserve:
- same core invariants as MCP
- strict output contract in assessment mode

### Future targets

Templates may be added for:
- Claude Code / IDE agent guidance
- OpenAI tool runtimes
- LangChain/LangGraph wrappers
- REST/hosted harness instruction formats

---

## 9. CI and Validation Gates

Required checks:
1. `generate` produces no diff on committed outputs.
2. All active prompt files have valid contract header.
3. Contract version in generated files is consistent across targets.
4. Hash manifest matches generated files.
5. Prompt text does not contradict `docs/system-design/EVIDRA_PROTOCOL.md`.

Recommended command contract:

```bash
make prompts-generate
make prompts-verify
```

---

## 10. Implementation State

Implemented:
1. Source contract tree under `prompts/source/contracts/v1.0.1/`.
2. Deterministic generator + verifier (`evidra prompts generate|verify`).
3. Make wrappers (`make prompts-generate`, `make prompts-verify`).
4. CI/release drift enforcement via `make prompts-verify`.

Planned next:
1. Additional target templates (LangChain/LangGraph/REST-hosted variants).
2. Optional contract profiles/overrides when needed.
3. Contributor guardrails for non-editable generated paths.

---

## 11. Ownership

Single source contract ownership:
- Evidra maintainers owning protocol + prompt behavior.

Review requirement for contract changes:
1. protocol correctness review
2. benchmark comparability review
3. experiment harness compatibility review
