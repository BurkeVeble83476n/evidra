# Evidra Project Review — 2026-03-07

Full architect + product owner review of the evidra-benchmark codebase.

## Build Health

- Build: clean (3 binaries, 0 errors)
- Tests: all 26 packages pass, 0 failures
- Lint: 0 issues (golangci-lint with funlen, gocyclo, nestif)
- go vet: clean
- go mod tidy: clean
- Race detector: passes

---

## 1. evidra-exp — Internal

`evidra-exp` is internal tooling for experiment runners (artifact benchmarks, execution scenarios).

**Current state:**
- Built by GoReleaser and Homebrew but NOT tested in CI
- Barely documented in README (mentioned once in install section)
- Depends on `internal/experiments/` package with Claude/Bifrost/MCP-kubectl adapters

**Decision:** Internal. Do not ship in public release.

**Actions:**
- [ ] Remove `evidra-exp` from `.goreleaser.yaml` builds, archives, and homebrew formula
- [ ] Remove `bin/evidra-exp` from `make build` target
- [ ] Keep `cmd/evidra-exp/` and `internal/experiments/` in repo for internal use
- [ ] Remove the `bin/evidra-exp` mention from README install section

---

## 2. benchmark command — Minimum Implementation Required

`evidra benchmark` ships 5 non-functional subcommands (`run`, `list`, `validate`, `record`, `compare`) that return exit codes 3 or 4. Users hitting these get confusing output.

**Decision:** Minimum impl required — do not ship broken stubs.

**Actions:**
- [ ] Remove `benchmark` from the command dispatch in `cmd/evidra/main.go`
- [ ] Remove `cmd/evidra/benchmark.go` or gate it behind build tag
- [ ] Remove `benchmark` line from `printUsage()` help output
- [ ] Keep `docs/system-design/EVIDRA_BENCHMARK_CLI.md` as the design doc for future implementation
- [ ] If keeping the file: rename to `benchmark_experimental.go` with `//go:build experimental` tag

---

## 3. detectors command — Not Yet (for public promotion)

`evidra detectors list` works correctly but is not documented in README Core Commands table.

**Current state:**
- Functional: lists all registered detectors with metadata, severity, stability
- Has `--stable-only` filter
- Has tests (`detectors_test.go`)
- Not mentioned in README, CLI reference table is incomplete

**Decision:** Not yet promoted to user-facing. Keep as power-user/developer command.

**Actions:**
- [ ] Do NOT add to README Core Commands table yet
- [ ] Add to CLI Reference doc (`docs/integrations/CLI_REFERENCE.md`) under a "Developer Commands" section
- [ ] Consider promoting in v0.4 when detector coverage is broader (Azure, GCP stubs are empty)

---

## 4. MCP Schema Mismatches — Needs Investigation

Three schema-vs-code mismatches found in the MCP server. These are API contract bugs that affect any MCP client integration.

### 4a. `actor_meta` in prescribe schema but not captured in code

- **Schema:** `pkg/mcpserver/schemas/prescribe.schema.json` declares `actor_meta` field
- **Code:** `server.go` PrescribeInput struct does not read or store `actor_meta`
- **Impact:** Data silently dropped. MCP clients sending actor metadata lose it.
- **Investigation needed:** Was `actor_meta` intended to map to `Actor.InstanceID`/`Actor.Version`? Or was it removed from the code intentionally? Check git history for when it was added/removed.

### 4b. `external_refs` in report code but not in schema

- **Schema:** `pkg/mcpserver/schemas/report.schema.json` does NOT declare `external_refs`
- **Code:** `server.go` ReportInput struct has `ExternalRefs` field
- **Impact:** MCP clients sending `external_refs` get schema validation rejection.
- **Investigation needed:** Should the schema be updated to include `external_refs`, or was it intentionally excluded from the MCP surface? CLI `report` command does support `--external-refs`.

### 4c. `canonical_action` schema incomplete

- **Schema:** `prescribe.schema.json` defines `canonical_action` with 4 fields: `resource_identity`, `resource_count`, `operation_class`, `scope_class`
- **Code:** `canon.CanonicalAction` struct has 7 fields including `tool`, `operation`, `resource_shape_hash`
- **Impact:** MCP clients cannot pass pre-canonicalized actions with full fidelity.
- **Investigation needed:** Are the missing fields intentionally excluded from MCP (derived server-side), or is the schema just incomplete?

**Actions:**
- [ ] Check git history for `actor_meta` intent
- [ ] Decide: add `external_refs` to report schema or remove from code
- [ ] Decide: which `canonical_action` fields belong in MCP schema vs derived server-side
- [ ] Fix all three and add integration tests that verify schema ↔ struct parity
- [ ] Consider a contract test: unmarshal every schema field into the Go struct and verify no field is silently dropped

---

## Documentation Issues (actionable list)

### Ship blockers

- [ ] **CHANGELOG wrong:** Says "Five behavioral signals" — code has 7 (missing: repair_loop, thrashing). Fix before release.
- [ ] **CHANGELOG wrong:** Lists signed evidence as future — already shipped. Update.
- [ ] **Roadmap stale:** `docs/product/EVIDRA_ROADMAP.md` lists shipped features as future (signed evidence = v1.1, Helm adapter = v1.2). Rewrite to reflect actual state.
- [ ] **CONTRIBUTING.md missing:** Apache 2.0 OSS project needs contributor guidance.
- [ ] **SECURITY.md missing:** Project handling signing keys needs vulnerability disclosure policy.

### Should-fix

- [ ] **README Core Commands table incomplete:** Missing `keygen` (working command, only in Quick Start section).
- [ ] **CLAUDE.md lists `EVIDRA_API_URL`:** Not implemented anywhere. Remove.
- [ ] **CLAUDE.md `EVIDRA_ENVIRONMENT`:** Only used in MCP server, not CLI. Clarify scope.
- [ ] **README missing `EVIDRA_ENVIRONMENT`** in environment variable table.

---

## Dead Code (confirmed unused)

| Item | Location | Action |
|---|---|---|
| `MaxBaseSeverity()` | `internal/detectors/registry.go:87-104` | Remove — exported, never called |
| `RehashEntry()` | `pkg/evidence/entry_builder.go:112-128` | Remove — deprecated flow |
| `SegmentFiles()` | `pkg/evidence/segment.go:9-34` | Remove — never called |
| Azure detector stub | `internal/detectors/terraform/azure/doc.go` | Keep — roadmap placeholder |
| GCP detector stub | `internal/detectors/terraform/gcp/doc.go` | Keep — roadmap placeholder |
| Stale worktree | `.worktrees/mcp-inspector-e2e/` | Clean up — `git worktree remove` |

---

## Test Gaps (not blocking release, track for v0.4)

| Package | Gap |
|---|---|
| `internal/detectors/terraform/helpers.go` | 6 exported helpers tested only indirectly |
| `pkg/evlock` | File locking logic untested |
| `internal/experiments/artifact_baseline_runner.go` | `RunArtifactBaseline` untested |
| `pkg/evidence/types.go` | `ErrorCode`, `IsStoreBusyError` untested |
| `internal/promptfactory/validate.go` | `ValidateBundle` tested only indirectly |

---

## Code Quality Notes (non-blocking)

| Area | Note |
|---|---|
| `cmdScorecard` / `cmdExplain` duplication | ~80% identical flag parsing and filtering. Extract shared helper in v0.4. |
| Mixed logging (`fmt`/`log`/`slog`) | Already tracked in `TECH_DEBT_TRACKER.md` as P1. |
| `MinOperations=100` compile-time only | Already tracked in `TECH_DEBT_TRACKER.md` as P1. |
| CI missing `evidra-exp` build | Add to CI if keeping in repo, or remove from release. |

---

## Ship Readiness Summary

| Category | Status |
|---|---|
| Core pipeline | Ready |
| CLI commands | Ready (minus benchmark stubs) |
| MCP server | Blocked on schema investigation (#4) |
| Evidence chain | Ready |
| Signing | Ready |
| Metrics export | Ready |
| Documentation | Blocked on CHANGELOG/Roadmap fixes |
| OSS hygiene | Blocked on CONTRIBUTING.md + SECURITY.md |
| CI/Release | Ready (GoReleaser, Homebrew, Docker) |

**Estimated effort to unblock:** ~1 day focused work.
