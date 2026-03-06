# Prompt Factory Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a single-source-of-truth prompt factory so Evidra prompt semantics are authored once and generated deterministically for MCP, LiteLLM, and future targets.

**Architecture:** Introduce canonical prompt contract sources under `prompts/source/contracts/<version>/`, render target-specific prompt artifacts via a deterministic Go generator, and enforce drift checks in CI with `prompts-verify`. Runtime prompt file paths remain backward-compatible (`prompts/mcpserver/*`, `prompts/experiments/litellm/*`).

**Tech Stack:** Go 1.23, `go.yaml.in/yaml/v3`, `text/template`, bash, jq, GitHub Actions.

---

### Task 1: Scaffold Canonical Source Contract Tree

**Files:**
- Create: `prompts/source/contracts/v1.0.1/CONTRACT.yaml`
- Create: `prompts/source/contracts/v1.0.1/CLASSIFICATION.yaml`
- Create: `prompts/source/contracts/v1.0.1/OUTPUT_CONTRACTS.yaml`
- Create: `prompts/source/contracts/v1.0.1/CHANGELOG.md`
- Create: `prompts/source/contracts/v1.0.1/templates/mcp/initialize.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/mcp/prescribe.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/mcp/report.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/mcp/get_event.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/mcp/agent_contract.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/litellm/system_instructions.tmpl`
- Create: `prompts/source/contracts/v1.0.1/templates/litellm/agent_contract.tmpl`

**Step 1: Add source YAML contracts and template files with placeholders**

Put explicit semantic keys in YAML:

```yaml
contract_version: v1.0.1
invariants:
  - "Every prescribe must have exactly one report."
```

**Step 2: Add template placeholders for invariant sections**

Example template line:

```text
# contract: {{ .ContractVersion }}
{{ range .Invariants }}- {{ . }}
{{ end }}
```

**Step 3: Sanity check structure**

Run: `find prompts/source/contracts/v1.0.1 -type f | sort`
Expected: all source + template files listed.

**Step 4: Commit**

```bash
git add prompts/source/contracts/v1.0.1
git commit -m "feat(prompts): scaffold canonical prompt source contract v1.0.1"
```

---

### Task 2: Implement Prompt Factory Types and Loader

**Files:**
- Create: `internal/promptfactory/types.go`
- Create: `internal/promptfactory/load.go`
- Create: `internal/promptfactory/validate.go`
- Test: `internal/promptfactory/load_test.go`

**Step 1: Write failing loader test**

Add test for parsing source files:

```go
func TestLoadContractBundle(t *testing.T) {
    _, err := LoadBundle("prompts/source/contracts/v1.0.1")
    if err != nil {
        t.Fatalf("load bundle: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/promptfactory -run TestLoadContractBundle -count=1`
Expected: FAIL (`undefined: LoadBundle`).

**Step 3: Implement minimal loader + schema validation**

Implement `LoadBundle(dir string) (Bundle, error)`:
- read 3 YAML files
- unmarshal into typed structs
- validate required fields (`contract_version`, invariants, classifications)

**Step 4: Re-run test**

Run: `go test ./internal/promptfactory -run TestLoadContractBundle -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/promptfactory
git commit -m "feat(promptfactory): add canonical source loader and validation"
```

---

### Task 3: Implement Deterministic Renderer and Generation Plan

**Files:**
- Create: `internal/promptfactory/render.go`
- Create: `internal/promptfactory/generate.go`
- Test: `internal/promptfactory/render_test.go`

**Step 1: Write failing deterministic render test**

```go
func TestRenderDeterministic(t *testing.T) {
    a, _ := RenderTarget(bundle, "mcp.initialize")
    b, _ := RenderTarget(bundle, "mcp.initialize")
    if a != b { t.Fatal("non-deterministic render") }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/promptfactory -run TestRenderDeterministic -count=1`
Expected: FAIL (`undefined: RenderTarget`).

**Step 3: Implement renderer**

- parse templates with `text/template`
- stable map iteration (pre-sort slices before render)
- normalize output (`\n`, trim trailing spaces)

**Step 4: Add generation mapping**

Map source targets to runtime output paths:
- MCP files -> `prompts/mcpserver/...`
- LiteLLM files -> `prompts/experiments/litellm/...`

**Step 5: Re-run package tests**

Run: `go test ./internal/promptfactory -count=1`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/promptfactory
git commit -m "feat(promptfactory): add deterministic template renderer and target map"
```

---

### Task 4: Add Prompt Factory CLI Commands

**Files:**
- Create: `cmd/evidra/prompts.go`
- Modify: `cmd/evidra/main.go`
- Test: `cmd/evidra/prompts_test.go`

**Step 1: Write failing CLI test**

```go
func TestPromptsHelp(t *testing.T) {
    out := runCLI(t, "prompts", "--help")
    if !strings.Contains(out, "generate") { t.Fatal("missing generate") }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/evidra -run TestPromptsHelp -count=1`
Expected: FAIL (unknown command).

**Step 3: Implement command surface**

Commands:
- `evidra prompts generate --contract v1.0.1`
- `evidra prompts verify --contract v1.0.1`

**Step 4: Re-run tests**

Run: `go test ./cmd/evidra -run TestPromptsHelp -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add cmd/evidra
git commit -m "feat(cli): add evidra prompts generate/verify commands"
```

---

### Task 5: Add Manifest Writer and Drift Verification

**Files:**
- Create: `internal/promptfactory/manifest.go`
- Test: `internal/promptfactory/manifest_test.go`
- Create: `prompts/manifests/v1.0.1.json` (generated artifact)

**Step 1: Write failing manifest verification test**

```go
func TestVerifyDetectsDrift(t *testing.T) {
    err := VerifyOutputs(bundle, "prompts/manifests/v1.0.1.json")
    if err != nil { t.Fatalf("verify: %v", err) }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/promptfactory -run TestVerifyDetectsDrift -count=1`
Expected: FAIL until manifest logic is added.

**Step 3: Implement manifest hashing**

- hash every generated output with SHA-256
- persist manifest JSON with stable ordering
- compare current outputs vs manifest in verify mode

**Step 4: Generate initial manifest**

Run: `go run ./cmd/evidra prompts generate --contract v1.0.1`
Expected: creates/updates `prompts/manifests/v1.0.1.json`.

**Step 5: Re-run tests**

Run: `go test ./internal/promptfactory -count=1`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/promptfactory prompts/manifests/v1.0.1.json
git commit -m "feat(promptfactory): add manifest hashing and drift verification"
```

---

### Task 6: Add Make Targets and Script Wrappers

**Files:**
- Modify: `Makefile`
- Create: `scripts/prompts-generate.sh`
- Create: `scripts/prompts-verify.sh`

**Step 1: Add `prompts-generate` and `prompts-verify` targets**

```make
prompts-generate:
	go run ./cmd/evidra prompts generate --contract v1.0.1
```

**Step 2: Add wrappers with strict shell settings**

```bash
#!/usr/bin/env bash
set -euo pipefail
go run ./cmd/evidra prompts verify --contract v1.0.1
```

**Step 3: Run make targets**

Run: `make prompts-generate && make prompts-verify`
Expected: both succeed; verify reports no drift.

**Step 4: Commit**

```bash
git add Makefile scripts/prompts-generate.sh scripts/prompts-verify.sh
git commit -m "chore(prompts): add generation and verification make targets"
```

---

### Task 7: CI Enforcement

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

**Step 1: Add prompt verification step in CI**

Add after build/test:

```yaml
- name: Verify generated prompts
  run: make prompts-verify
```

**Step 2: Add same check in release pipeline**

Add to both `test` and/or `e2e` release jobs.

**Step 3: Validate workflows**

Run: `bash -n .github/workflows/ci.yml` (or static lint tool if available)
Expected: syntax valid.

**Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "ci(prompts): enforce generated prompt drift verification"
```

---

### Task 8: Documentation Cutover and Contributor Rules

**Files:**
- Modify: `docs/system-design/EVIDRA_PROMPT_FACTORY_SPEC.md`
- Modify: `docs/system-design/MCP_CONTRACT_PROMPTS.md`
- Modify: `docs/system-design/EVIDRA_MCP_PROMPT_TUNING_METHOD.md`
- Modify: `README.md`
- Modify: `experiments/README.md`

**Step 1: Document canonical workflow**

Add contributor rule: edit only `prompts/source/contracts/*`, then generate outputs.

**Step 2: Add command examples**

Document:
- `make prompts-generate`
- `make prompts-verify`

**Step 3: Add migration note**

State runtime paths remain backward-compatible and generated.

**Step 4: Run docs checks**

Run: `bash scripts/check-doc-commands.sh`
Expected: `doc checks passed`.

**Step 5: Commit**

```bash
git add README.md experiments/README.md docs/system-design/*.md
git commit -m "docs(prompts): document source-of-truth generation workflow"
```

---

### Task 9: Final Verification and Release-Readiness Check

**Files:**
- Validate all changed files from tasks 1-8.

**Step 1: Run full test + lint + prompt checks**

Run:

```bash
make fmt
make test
make lint
make prompts-verify
```

Expected:
- all pass
- no prompt drift

**Step 2: Validate generated outputs are stable**

Run twice:

```bash
make prompts-generate
git diff --exit-code
make prompts-generate
git diff --exit-code
```

Expected: no changes after second run.

**Step 3: Final commit (if needed)**

```bash
git add -A
git commit -m "feat(prompts): implement single-source prompt factory and generation pipeline"
```

