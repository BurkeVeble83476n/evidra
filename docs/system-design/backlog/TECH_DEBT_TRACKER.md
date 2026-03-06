# Evidra Known Tech Debt Tracker
Status: Active  
Last updated: 2026-03-06

This document tracks known technical debt items that materially affect
reliability, maintainability, contributor onboarding, or production adoption.

Priority scale:
- `P0` critical architecture/runtime risk
- `P1` important maintainability/operability debt
- `P2` useful improvement, not immediately blocking

---

## Recently closed

| Item | Why it mattered | Status |
|---|---|---|
| `FindEntryByID` hot-path optimization | Repeated report lookups scanned all entries | Closed in `7f64a5d` (in-process lookup cache) |
| `RehashEntry` in prescribe flow | Double-sign/double-hash flow was fragile | Closed in `579628a` (single-pass prescribe build) |
| Optional non-blocking write mode | Strict-only behavior conflicted with inspector positioning | Closed in `512e4cc` (`best_effort` mode) |
| Bridge anonymous struct coupling | Fragile extraction from `canonical_action` payload | Closed in `bd049c1` (use `canon.CanonicalAction`) |
| Duplicate write-mode parsing logic | CLI and MCP parsed env locally with drift risk | Closed in `9fcda1d` (centralized config resolver) |
| CLI command decomposition for mutate/report/findings paths | Large handlers mixed flag parsing, domain execution, and render concerns | Closed on 2026-03-06 (`cmd/evidra/main.go` parse/prepare/execute/output split for `prescribe`, `report`, `ingest-findings`) |
| Missing complexity/function-size lint guardrails | Regressions toward oversized multi-responsibility handlers could reappear | Closed on 2026-03-06 (`.golangci.yml`: `funlen`, `gocyclo`, `nestif`; tests excluded for these rules) |

---

## Active known debt

| Priority | Debt | Impact | Code/docs anchors | Proposed direction | Status |
|---|---|---|---|---|---|
| `P0` | Local file-locking model for shared multi-writer stores | `evidence_store_busy` under concurrent writers on shared mounts | `pkg/evidence/lock.go`, `pkg/evlock/lock_unix.go`, `pkg/evidence/types.go` | Define supported local-write topology explicitly; for shared/multi-writer use API/backend ingestion path | Open |
| `P0` | No centralized API backend in current release | Limits reliable multi-node ingestion, tenancy, and receipt workflows | `CHANGELOG.md` ("No centralized API server"), `docs/system-design/backlog/V050_IMPLEMENTATION_BACKLOG.md` | Implement v0.5 API/outbox + receipts as canonical distributed write path | Open |
| `P1` | Mixed logging stack (`fmt`, `log`, `slog`) | Inconsistent observability and parsing in production | `cmd/evidra/main.go`, `cmd/evidra-mcp/main.go`, `internal/evidence/signer.go` | Standardize around `slog` with common logger wiring and structured fields | Open |
| `P1` | `MinOperations=100` is compile-time only | Hard to tune per org/workload | `internal/score/score.go` | Add configurable threshold (flag/env/config object), keep default at 100 | Open |
| `P1` | Threat-model depth for enterprise ops | Key rotation and tenancy expectations not fully actionable | `docs/system-design/backlog/EVIDRA_THREAT_MODEL.md` | Add concrete rotation, key-id lifecycle, and multi-tenant isolation guidance | Open |
| `P2` | OSS contributor hygiene missing | Slower onboarding and trust for external contributors | Missing: `CONTRIBUTING.md`, `SECURITY.md`, issue templates | Add contributor guide, vuln disclosure policy, issue/PR templates | Open |
| `P2` | No Go micro-benchmarks | Performance changes are harder to evaluate objectively | No `Benchmark*` functions in current test suite | Add `Benchmark*` for evidence append, lookup, and signal conversion hot paths | Open |

---

## Notes on behavior mode

`EVIDRA_EVIDENCE_WRITE_MODE` now supports:
- `strict` (default): return store read/write errors
- `best_effort`: warn and continue when local evidence store I/O fails

This mode is intended for inspector resilience in constrained local
environments. It is not a replacement for durable centralized ingestion.

---

## Next recommended execution order

1. `P1` logging unification (`slog` end-to-end)
2. `P1` configurable score sufficiency threshold
3. `P2` OSS hygiene docs/templates
