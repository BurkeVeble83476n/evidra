# v0.3.0 Release Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all ship blockers and should-fix items from the 2026-03-07 project review so v0.3.0 can ship cleanly.

**Architecture:** No architectural changes. This is cleanup: remove dead code, fix docs, align MCP schemas with code, remove internal tooling from public release, gate unfinished features behind build tags.

**Tech Stack:** Go 1.23, JSON Schema, Markdown

---

### Task 1: Remove evidra-exp from public release

**Files:**
- Modify: `.goreleaser.yaml:46-63` (remove evidra-exp build)
- Modify: `.goreleaser.yaml:83-90` (remove evidra-exp archive)
- Modify: `.goreleaser.yaml:135-149` (remove evidra-exp homebrew formula)
- Modify: `Makefile:10` (remove `go build -o bin/evidra-exp ./cmd/evidra-exp`)
- Modify: `README.md:38` (remove `#   bin/evidra-exp`)
- Modify: `docs/integrations/CLI_REFERENCE.md:6` (remove `- evidra-exp (experiment runner)`)

**Step 1: Edit `.goreleaser.yaml`**

Remove the entire `evidra-exp` build block (lines 46-62), the `evidra-exp` archive block (lines 83-90), and the `evidra-exp` homebrew block (lines 135-149).

**Step 2: Edit `Makefile`**

Remove the `go build -o bin/evidra-exp ./cmd/evidra-exp` line from the `build` target.

**Step 3: Edit `README.md`**

Remove line 38 (`#   bin/evidra-exp`) from the install code block.

**Step 4: Edit `docs/integrations/CLI_REFERENCE.md`**

Remove line 6 (`- evidra-exp (experiment runner)`) from the binary list at the top.

**Step 5: Verify build**

Run: `make build`
Expected: Builds `bin/evidra` and `bin/evidra-mcp` only. No `bin/evidra-exp`.

**Step 6: Run tests**

Run: `make test`
Expected: All pass. `cmd/evidra-exp` tests still run (package is in repo, just not released).

**Step 7: Commit**

```bash
git add .goreleaser.yaml Makefile README.md docs/integrations/CLI_REFERENCE.md
git commit -m "chore(release): remove evidra-exp from public release artifacts

evidra-exp is internal experiment tooling. Keep cmd/evidra-exp/ and
internal/experiments/ in repo for development use."
```

---

### Task 2: Gate benchmark command behind build tag

**Files:**
- Modify: `cmd/evidra/main.go:61` (remove benchmark case from switch)
- Modify: `cmd/evidra/main.go:1046` (remove benchmark line from printUsage)
- Modify: `cmd/evidra/benchmark.go:1` (add build tag)
- Modify: `cmd/evidra/benchmark_test.go` (add build tag)
- Modify: `docs/integrations/CLI_REFERENCE.md:28` (remove benchmark row from command table)

**Step 1: Add build tag to `cmd/evidra/benchmark.go`**

Add `//go:build experimental` as line 1, blank line, then existing `package main`.

**Step 2: Add build tag to `cmd/evidra/benchmark_test.go`**

Add `//go:build experimental` as line 1, blank line, then existing content.

**Step 3: Create `cmd/evidra/benchmark_stub.go`**

This provides the `cmdBenchmark` function when the `experimental` tag is NOT set, so `main.go` compiles either way:

```go
//go:build !experimental

package main

import (
	"fmt"
	"io"
)

func cmdBenchmark(_ []string, _ io.Writer, stderr io.Writer) int {
	fmt.Fprintln(stderr, "benchmark is not available in this build (experimental feature)")
	return 2
}
```

**Step 4: Edit `docs/integrations/CLI_REFERENCE.md`**

Remove the `| benchmark | Benchmark command group (preview stubs) |` row from the command table (line 28).

**Step 5: Verify build and tests**

Run: `make build && make test`
Expected: Builds clean. Default build excludes benchmark. Tests pass (benchmark tests skipped unless `-tags experimental`).

**Step 6: Verify benchmark is gated**

Run: `./bin/evidra benchmark run`
Expected: `benchmark is not available in this build (experimental feature)` and exit code 2.

**Step 7: Commit**

```bash
git add cmd/evidra/benchmark.go cmd/evidra/benchmark_test.go cmd/evidra/benchmark_stub.go docs/integrations/CLI_REFERENCE.md
git commit -m "feat(cli): gate benchmark command behind experimental build tag

benchmark subcommands are not yet implemented. Hide from default builds
to avoid confusing users. Enable with: go build -tags experimental"
```

---

### Task 3: Add detectors to CLI Reference as developer command

**Files:**
- Modify: `docs/integrations/CLI_REFERENCE.md` (add Developer Commands section)

**Step 1: Add section to CLI Reference**

After the last `evidra` command section (before the `evidra-mcp` section), add:

```markdown
### Developer Commands

These commands are functional but not yet part of the stable public API.

#### `evidra detectors list`

| Flag | Description |
|---|---|
| `--stable-only` | Show only stable (non-experimental) detectors |

Output: JSON with `count` and `items` array of detector metadata (tag, description, severity, stability).
```

**Step 2: Commit**

```bash
git add docs/integrations/CLI_REFERENCE.md
git commit -m "docs(cli): add detectors command to CLI Reference as developer command"
```

---

### Task 4: Fix MCP schema — add `external_refs` to report schema

**Files:**
- Modify: `pkg/mcpserver/schemas/report.schema.json`
- Modify: `pkg/mcpserver/e2e_test.go` or create new contract test

**Step 1: Write failing test**

Add a test to `pkg/mcpserver/server_test.go` that verifies `external_refs` round-trips through report:

```go
func TestReport_ExternalRefsAccepted(t *testing.T) {
	t.Parallel()
	// Setup server with temp evidence dir, prescribe first, then report with external_refs.
	// Verify the report call succeeds (schema validation does not reject external_refs).
	// Verify the stored evidence entry contains the external refs.
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestReport_ExternalRefsAccepted ./pkg/mcpserver/ -v -count=1`
Expected: FAIL (schema validation rejects `external_refs`).

**Step 3: Add `external_refs` to report schema**

Edit `pkg/mcpserver/schemas/report.schema.json`. Add after `parent_span_id`:

```json
    "external_refs": {
      "type": "array",
      "description": "External reference links (e.g. GitHub run ID, Jira ticket)",
      "items": {
        "type": "object",
        "properties": {
          "type": { "type": "string" },
          "id": { "type": "string" }
        }
      }
    }
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestReport_ExternalRefsAccepted ./pkg/mcpserver/ -v -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/mcpserver/schemas/report.schema.json pkg/mcpserver/server_test.go
git commit -m "fix(mcp): add external_refs to report schema

The code accepted external_refs but the JSON schema did not declare it,
causing schema validation to reject the field from MCP clients."
```

---

### Task 5: Fix MCP schema — remove `actor_meta` from prescribe schema

Investigation result: `actor_meta` was added as a free-form metadata bag. The structured fields (`actor.instance_id`, `actor.version`, `actor.skill_version`) now cover the same use cases. The code never reads `actor_meta` — it is silently dropped. The correct fix is to remove it from the schema.

**Files:**
- Modify: `pkg/mcpserver/schemas/prescribe.schema.json:39-42` (remove `actor_meta`)

**Step 1: Write failing test**

Add a contract test to `pkg/mcpserver/server_test.go` that verifies no schema field is silently dropped:

```go
func TestPrescribeSchema_NoSilentlyDroppedFields(t *testing.T) {
	t.Parallel()
	// Parse prescribe.schema.json, extract all top-level property names.
	// For each property name, verify it has a corresponding json tag in PrescribeInput struct.
	// Fail if any schema property is not mapped to a struct field.
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestPrescribeSchema_NoSilentlyDroppedFields ./pkg/mcpserver/ -v -count=1`
Expected: FAIL — `actor_meta` is in schema but not in struct.

**Step 3: Remove `actor_meta` from prescribe schema**

Remove lines 39-42 from `pkg/mcpserver/schemas/prescribe.schema.json`:

```json
    "actor_meta": {
      "type": "object",
      "description": "Optional comparison dimensions (agent_version, model_id, prompt_id)"
    },
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestPrescribeSchema_NoSilentlyDroppedFields ./pkg/mcpserver/ -v -count=1`
Expected: PASS.

**Step 5: Run all MCP tests**

Run: `go test ./pkg/mcpserver/ -v -count=1`
Expected: All pass.

**Step 6: Commit**

```bash
git add pkg/mcpserver/schemas/prescribe.schema.json pkg/mcpserver/server_test.go
git commit -m "fix(mcp): remove actor_meta from prescribe schema

actor_meta was declared in the schema but never read by the server.
Structured actor fields (instance_id, version, skill_version) cover the
same use cases. Add contract test to prevent future schema/code drift."
```

---

### Task 6: Fix MCP schema — canonical_action intentionally partial

Investigation: `tool`, `operation`, and `resource_shape_hash` are **derived server-side** during canonicalization. MCP clients pass `canonical_action` only to override the classification fields (`operation_class`, `scope_class`, `resource_identity`, `resource_count`). The schema is intentionally partial — this is not a bug but needs a clarifying comment.

**Files:**
- Modify: `pkg/mcpserver/schemas/prescribe.schema.json:17-26` (add description clarification)

**Step 1: Update canonical_action description**

Change the description field in `prescribe.schema.json`:

```json
    "canonical_action": {
      "type": "object",
      "description": "Optional pre-canonicalized classification overrides. Fields like tool, operation, and resource_shape_hash are derived server-side and should not be provided here.",
      "properties": {
        "resource_identity": { "type": "array" },
        "resource_count": { "type": "integer" },
        "operation_class": { "type": "string" },
        "scope_class": { "type": "string" }
      }
    },
```

**Step 2: Commit**

```bash
git add pkg/mcpserver/schemas/prescribe.schema.json
git commit -m "docs(mcp): clarify canonical_action schema is intentionally partial

tool, operation, and resource_shape_hash are derived server-side during
canonicalization. The schema exposes only the classification overrides."
```

---

### Task 7: Remove dead code

**Files:**
- Modify: `internal/detectors/registry.go:87-104` (remove `MaxBaseSeverity`)
- Modify: `pkg/evidence/entry_builder.go:112-128` (remove `RehashEntry`)
- Modify: `pkg/evidence/segment.go:9-34` (remove `SegmentFiles`)

**Step 1: Remove `MaxBaseSeverity` from `internal/detectors/registry.go`**

Delete lines 87-104 (the `MaxBaseSeverity` function and its comment).

**Step 2: Remove `RehashEntry` from `pkg/evidence/entry_builder.go`**

Delete lines 112-128 (the `RehashEntry` function and its comment).

**Step 3: Remove `SegmentFiles` from `pkg/evidence/segment.go`**

Delete lines 9-34 (the `SegmentFiles` function and its comment).

**Step 4: Verify build and tests**

Run: `make build && make test && make lint`
Expected: All pass. No references to these functions exist.

**Step 5: Commit**

```bash
git add internal/detectors/registry.go pkg/evidence/entry_builder.go pkg/evidence/segment.go
git commit -m "chore: remove dead code (MaxBaseSeverity, RehashEntry, SegmentFiles)

All three exported functions were confirmed unused in the codebase."
```

---

### Task 8: Clean up stale worktree

**Step 1: Remove worktree**

Run: `git worktree remove .worktrees/mcp-inspector-e2e`
Expected: Worktree removed. If it has changes, use `--force`.

**Step 2: Delete branch if orphaned**

Run: `git branch -d mcp-inspector-e2e`
Expected: Branch deleted (or already merged).

**Step 3: Verify**

Run: `git worktree list`
Expected: Only the main worktree listed.

No commit needed — this is git housekeeping.

---

### Task 9: Fix CHANGELOG

**Files:**
- Modify: `CHANGELOG.md:8-11`

**Step 1: Fix signal count and add Docker adapter**

Change line 8 to include Docker adapter:
```
- Canonicalization adapters: Kubernetes (kubectl, oc, helm), Terraform, Docker (docker, nerdctl, podman), generic fallback
```

Change line 10 from:
```
- Five behavioral signals: protocol violation, artifact drift, retry loop, blast radius, new scope
```
To:
```
- Seven behavioral signals: protocol violation, artifact drift, retry loop, blast radius, new scope, repair loop, thrashing
```

**Step 2: Add signing to Core Pipeline section**

After line 11, add:
```
- Ed25519 evidence signing with strict/optional modes and key generation
```

**Step 3: Add missing CLI commands**

After line 21 (`- Tool and scope filtering on scorecard/explain/compare`), add:
```
- `run` — execute command live and record lifecycle outcome
- `record` — ingest completed operation from structured JSON input
- `validate` — verify evidence chain integrity and signatures
- `ingest-findings` — ingest SARIF scanner findings as evidence entries
- `keygen` — generate Ed25519 signing keypair
- `detectors list` — list registered risk detectors with metadata
```

**Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): fix signal count, add missing features and commands

CHANGELOG incorrectly listed 5 signals (there are 7), omitted Docker
adapter, and was missing several CLI commands shipped in v0.3.0."
```

---

### Task 10: Fix Roadmap

**Files:**
- Modify: `docs/product/EVIDRA_ROADMAP.md`

**Step 1: Rewrite to reflect actual state**

```markdown
# Evidra Roadmap

## Shipped

### v0.3.0 (current)
- Canonicalization adapters: K8s (kubectl, oc, helm), Terraform, Docker, generic
- Seven behavioral signal detectors
- Weighted reliability scoring with bands and confidence
- Evidence chain: append-only JSONL, hash-linked, Ed25519 signed
- CLI: run, record, prescribe, report, scorecard, explain, compare, validate, ingest-findings, keygen
- MCP server: prescribe, report, get_event (stdio transport)
- OTLP metrics export
- GoReleaser + Homebrew + Docker

## Next

### v0.4.0
- ArgoCD-specific canonicalization adapter
- Benchmark dataset engine (currently gated behind experimental build tag)
- Configurable MinOperations threshold
- Azure and GCP Terraform detectors

### v0.5.0
- Centralized API backend for multi-node ingestion
- Postgres evidence store
- Receipt/outbox pattern for distributed writes
```

**Step 2: Commit**

```bash
git add docs/product/EVIDRA_ROADMAP.md
git commit -m "docs(roadmap): rewrite to reflect shipped vs planned features

Previous roadmap listed already-shipped features (signing, Helm) as
future milestones."
```

---

### Task 11: Fix CLAUDE.md environment variables

**Files:**
- Modify: `CLAUDE.md:65-68`

**Step 1: Update environment variables section**

Replace lines 65-68 with:

```markdown
- `EVIDRA_EVIDENCE_DIR` — evidence storage directory (default: `~/.evidra/evidence`)
- `EVIDRA_ENVIRONMENT` — environment label (MCP server only)
- `EVIDRA_RETRY_TRACKER` — enable retry loop tracking (MCP server only)
```

Remove `EVIDRA_API_URL` (not implemented).

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): fix environment variables — remove unimplemented EVIDRA_API_URL

EVIDRA_API_URL was never implemented. Clarify which env vars are
MCP-server-only."
```

---

### Task 12: Fix README — add keygen to Core Commands and add EVIDRA_ENVIRONMENT

**Files:**
- Modify: `README.md:94-104` (add keygen row to commands table)
- Modify: `README.md:153-164` (add EVIDRA_ENVIRONMENT to env table)

**Step 1: Add keygen to Core Commands table**

After the `compare` row (line 104), add:
```
| `keygen` | Generate Ed25519 signing keypair |
```

**Step 2: Add EVIDRA_ENVIRONMENT to env table**

After `EVIDRA_EVIDENCE_DIR` row, add:
```
| `EVIDRA_ENVIRONMENT` | Environment label (MCP server) |
```

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add keygen command and EVIDRA_ENVIRONMENT to reference tables"
```

---

### Task 13: Add CONTRIBUTING.md

**Files:**
- Create: `CONTRIBUTING.md`

**Step 1: Create file**

```markdown
# Contributing to Evidra

Thank you for your interest in contributing to Evidra.

## Development Setup

```bash
git clone https://github.com/vitas/evidra-benchmark.git
cd evidra-benchmark
make build
make test
```

Requires Go 1.23+.

## Running Tests

```bash
make test          # unit tests
make lint          # golangci-lint
make fmt           # gofmt
make e2e           # end-to-end tests (requires build)
make test-signals  # signal validation scenarios
```

## Code Style

- Go stdlib conventions. No web frameworks.
- `gofmt -w .` before every commit.
- Error wrapping: `fmt.Errorf("context: %w", err)`.
- See `CLAUDE.md` for full conventions.

## Pull Requests

1. Fork the repo and create a feature branch.
2. Write tests for new functionality.
3. Ensure `make test && make lint` pass.
4. Open a PR with a clear description of what and why.

## Reporting Issues

Open a GitHub issue with:
- What you expected
- What happened
- Steps to reproduce
- Evidra version (`evidra version`)

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
```

**Step 2: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: add CONTRIBUTING.md for OSS contributor guidance"
```

---

### Task 14: Add SECURITY.md

**Files:**
- Create: `SECURITY.md`

**Step 1: Create file**

```markdown
# Security Policy

## Supported Versions

| Version | Supported |
|---|---|
| 0.3.x | Yes |
| < 0.3 | No |

## Reporting a Vulnerability

If you discover a security vulnerability in Evidra, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, email: security@samebits.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Scope

Security-relevant areas in Evidra include:
- Evidence chain integrity (hash-linking, signatures)
- Ed25519 signing key handling
- File-based locking and concurrent access
- SARIF parser input handling
```

**Step 2: Commit**

```bash
git add SECURITY.md
git commit -m "docs: add SECURITY.md with vulnerability disclosure policy"
```

---

## Task Dependency Order

Tasks are independent except:
- Tasks 4, 5, 6 all modify MCP schema files — execute sequentially
- Task 2 depends on Task 1 being committed (both touch CLI Reference)

Recommended execution order:
1. Task 8 (worktree cleanup — no commit, do first)
2. Task 7 (dead code removal)
3. Task 1 (evidra-exp removal from release)
4. Task 2 (benchmark build tag)
5. Task 3 (detectors in CLI Reference)
6. Tasks 4 → 5 → 6 (MCP schema fixes, sequential)
7. Task 9 (CHANGELOG)
8. Task 10 (Roadmap)
9. Task 11 (CLAUDE.md)
10. Task 12 (README)
11. Task 13 (CONTRIBUTING.md)
12. Task 14 (SECURITY.md)

**Total: 14 tasks, ~12 commits, estimated 2-3 hours.**
