# MCP Assessment Scaling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace full evidence-store rescans on MCP `report` with an incremental assessment tracker, rename the core MCP service to `MCPService`, eliminate redundant post-write entry rereads for forwarding, and add graceful shutdown for retry-tracker goroutines.

**Architecture:** Add `internal/assessment.Tracker` as a process-local, per-evidence-path incremental cache keyed by `session_id`. `pkg/mcpserver` updates the tracker from freshly appended entries and reads snapshots from it on `report`, while falling back to session rebuilds when cache metadata indicates invalidation. Rename `BenchmarkService` to `MCPService`, thread raw serialized entry bytes through the forwarding path, and expose a cleanup-capable MCP server constructor so `cmd/evidra-mcp` can stop background goroutines cleanly.

**Tech Stack:** Go, `pkg/evidence`, `internal/assessment`, `internal/lifecycle`, `mark3labs/mcp-go`, `testing`

---

### Task 1: Save the approved design context in tests

**Files:**
- Modify: `internal/assessment/assessment_test.go`
- Test: `internal/assessment/assessment_test.go`

**Step 1: Write the failing test**

Add a test that creates a tracker for a temp evidence path, writes a prescription and report for one session, takes a snapshot, appends another report in the same session through the tracker, and asserts the second snapshot comes from incremental state instead of a forced full rebuild indicator.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/assessment -run TestTrackerIncrementalSnapshotReuse -count=1`
Expected: FAIL because `Tracker` does not exist yet.

**Step 3: Write minimal implementation**

Create only the minimal tracker surface needed to compile the test.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/assessment -run TestTrackerIncrementalSnapshotReuse -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/assessment/assessment_test.go internal/assessment/*.go
git commit -s -m "test: add tracker incremental snapshot coverage"
```

---

### Task 2: Add invalidation coverage for external writes

**Files:**
- Modify: `internal/assessment/assessment_test.go`
- Test: `internal/assessment/assessment_test.go`

**Step 1: Write the failing test**

Add a test that:

- builds a tracker snapshot for one session
- appends a new entry directly via `pkg/evidence` without calling `Observe`
- requests another snapshot
- asserts the tracker detects divergence and rebuilds correctly

**Step 2: Run test to verify it fails**

Run: `go test ./internal/assessment -run TestTrackerRebuildsAfterExternalWrite -count=1`
Expected: FAIL because invalidation/rebuild logic is incomplete.

**Step 3: Write minimal implementation**

Add tracker metadata and rebuild logic required to detect stale cache and rebuild the requested session.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/assessment -run TestTrackerRebuildsAfterExternalWrite -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/assessment/assessment_test.go internal/assessment/*.go
git commit -s -m "test: cover tracker invalidation on external writes"
```

---

### Task 3: Add forwarding-path regression coverage

**Files:**
- Modify: `pkg/mcpserver/server_test.go`
- Test: `pkg/mcpserver/server_test.go`

**Step 1: Write the failing test**

Add a test that performs `PrescribeCtx` or `ReportCtx` with forwarding enabled and asserts the forwarded JSON is produced without depending on a post-write lookup by injecting a service path that would fail if reread were required.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/mcpserver -run TestMCPServiceForwardUsesWrittenEntryBytes -count=1`
Expected: FAIL because the current implementation still rereads with `FindEntryByID`.

**Step 3: Write minimal implementation**

Refactor the MCP write/forward path to thread the already-built raw entry bytes directly to the forward callback.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/mcpserver -run TestMCPServiceForwardUsesWrittenEntryBytes -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/mcpserver/server_test.go pkg/mcpserver/*.go internal/lifecycle/*.go
git commit -s -m "fix: forward evidence entries without reread"
```

---

### Task 4: Add shutdown regression coverage

**Files:**
- Modify: `pkg/mcpserver/server_test.go`
- Modify: `cmd/evidra-mcp/main.go`
- Test: `pkg/mcpserver/server_test.go`

**Step 1: Write the failing test**

Add a test that constructs the cleanup-capable server/service path with retry tracking enabled, calls cleanup, and asserts the retry tracker stops cleanly and idempotently.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/mcpserver -run TestNewServerWithCleanup_StopsRetryTracker -count=1`
Expected: FAIL because no cleanup-capable constructor exists yet.

**Step 3: Write minimal implementation**

Add a cleanup-capable MCP server constructor and `MCPService.Close`, then wire `cmd/evidra-mcp` to call cleanup on shutdown.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/mcpserver -run TestNewServerWithCleanup_StopsRetryTracker -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/mcpserver/server_test.go pkg/mcpserver/*.go cmd/evidra-mcp/main.go
git commit -s -m "fix: add MCP server cleanup for retry tracker"
```

---

### Task 5: Rename `BenchmarkService` to `MCPService`

**Files:**
- Modify: `pkg/mcpserver/server.go`
- Modify: `pkg/mcpserver/server_test.go`
- Modify: `pkg/mcpserver/integration_test.go`
- Modify: `pkg/mcpserver/correlation_test.go`
- Modify: `pkg/mcpserver/scoring_profile_test.go`
- Modify: `pkg/mcpserver/e2e_test.go`

**Step 1: Write the failing test**

Rely on the existing package test suite after renaming test references to `MCPService`.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/mcpserver -count=1`
Expected: FAIL until the production rename is complete.

**Step 3: Write minimal implementation**

Rename the type and method receivers from `BenchmarkService` to `MCPService`, and update constructors/helpers without changing behavior.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/mcpserver -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/mcpserver/*.go
git commit -s -m "refactor: rename benchmark service to MCP service"
```

---

### Task 6: Implement the incremental tracker

**Files:**
- Create: `internal/assessment/tracker.go`
- Modify: `internal/assessment/assessment.go`
- Modify: `internal/assessment/assessment_test.go`
- Test: `internal/assessment/assessment_test.go`

**Step 1: Write the failing test**

Expand tracker coverage to assert:

- first snapshot cold-builds from disk
- `Observe` updates the same session incrementally
- snapshots are profile-aware
- operation counts and signal summaries stay correct after multiple reports

**Step 2: Run test to verify it fails**

Run: `go test ./internal/assessment -count=1`
Expected: FAIL until incremental tracker logic is complete.

**Step 3: Write minimal implementation**

Implement `Tracker`, per-session state, rebuild logic, and snapshot memoization by scoring-profile ID.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/assessment -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/assessment/*.go
git commit -s -m "feat: add incremental assessment tracker"
```

---

### Task 7: Wire MCP report handling to the tracker

**Files:**
- Modify: `pkg/mcpserver/server.go`
- Modify: `pkg/mcpserver/server_test.go`
- Modify: `pkg/mcpserver/integration_test.go`
- Test: `pkg/mcpserver/server_test.go`

**Step 1: Write the failing test**

Add a test that performs repeated `ReportCtx` calls for the same session and verifies snapshots stay correct while using the tracker-backed path.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/mcpserver -run TestMCPServiceReportUsesAssessmentTracker -count=1`
Expected: FAIL because `ReportCtx` still uses `BuildAtPathWithProfile`.

**Step 3: Write minimal implementation**

Inject/use the tracker in `MCPService`, update it on successful writes, and replace `BuildAtPathWithProfile` calls with tracker snapshots.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/mcpserver -run TestMCPServiceReportUsesAssessmentTracker -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/mcpserver/*.go internal/assessment/*.go
git commit -s -m "feat: use incremental assessment tracker in MCP reports"
```

---

### Task 8: Wire CLI report handling to the tracker

**Files:**
- Modify: `cmd/evidra/report.go`
- Modify: `cmd/evidra/assessment.go`
- Modify: `cmd/evidra/report_test.go`
- Test: `cmd/evidra/report_test.go`

**Step 1: Write the failing test**

Add a test that runs the CLI report assessment path twice against the same evidence path and asserts snapshot behavior remains correct using the shared tracker path.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/evidra -run TestReportAssessmentUsesTracker -count=1`
Expected: FAIL because CLI report still calls `BuildAtPathWithProfile` directly.

**Step 3: Write minimal implementation**

Reuse the shared tracker-backed assessment path in CLI reporting without changing user-visible output.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/evidra -run TestReportAssessmentUsesTracker -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/evidra/*.go internal/assessment/*.go
git commit -s -m "feat: share assessment tracker with CLI reporting"
```

---

### Task 9: Run targeted verification

**Files:**
- Test: `pkg/mcpserver`
- Test: `internal/assessment`
- Test: `cmd/evidra`
- Test: `cmd/evidra-mcp`

**Step 1: Run targeted package tests**

Run: `go test ./pkg/mcpserver ./internal/assessment ./cmd/evidra ./cmd/evidra-mcp -count=1`
Expected: PASS

**Step 2: Fix any failures minimally**

If a package fails, apply the smallest root-cause fix and rerun the same command until it passes.

**Step 3: Commit**

```bash
git add -A
git commit -s -m "test: verify MCP assessment scaling changes"
```

---

### Task 10: Run full regression verification

**Files:**
- Test: entire repository

**Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

**Step 2: Record the exact verification commands**

Capture the commands and note any intentionally skipped checks.

**Step 3: Final commit if needed**

```bash
git add -A
git commit -s -m "chore: finalize MCP assessment scaling rollout"
```
