# MCP Assessment Scaling Design

## Goal

Remove the `ReadAllEntriesAtPath` full-scan bottleneck from MCP `report` calls, clean up the misleading `BenchmarkService` name, eliminate the redundant post-write forward lookup, and add graceful shutdown for the retry tracker.

## Problem

Today every MCP `report` call records a report entry and then immediately runs:

- `assessment.BuildAtPathWithProfile`
- `evidence.ReadAllEntriesAtPath`

That makes assessment generation `O(n)` in total evidence entries per report call. With large evidence stores this adds visible latency and will scale poorly for long-lived MCP servers.

The current implementation also has three secondary issues:

1. `BenchmarkService` is the core MCP service but its name suggests a separate benchmark subsystem.
2. `tryForwardEntry` rereads the freshly written entry by ID even though the entry is already in memory when it is appended.
3. `RetryTracker` owns a cleanup goroutine but MCP server construction does not expose a cleanup path, so long-running processes have no explicit shutdown hook.

## Scope

In scope:

- `pkg/mcpserver`
- `internal/assessment`
- `cmd/evidra`
- `cmd/evidra-mcp`

Out of scope:

- distributed or multi-process cache coherence
- new scoring rules
- storage format changes for evidence logs

## Design Principles

1. Hot-path MCP reports should avoid full evidence-store scans.
2. Cache behavior must be correct first and fast second.
3. External writers to the same evidence path must not corrupt results; they should trigger safe rebuilds.
4. Existing public behavior should stay stable apart from lower latency and the service rename.
5. Shutdown must be explicit and best-effort.

## Chosen Approach

Implement a process-local incremental assessment tracker per evidence path and wire MCP report handling to update it from newly appended entries.

This is a middle path between two extremes:

- not enough: a read-through entry cache would avoid some disk and JSON work but still recompute all signals on every report
- too invasive right now: moving assessment ownership fully into `internal/lifecycle` would blur package boundaries and create a larger refactor surface

The selected design keeps current scoring and signal code intact while changing how session state is fed into it.

## Architecture

### `assessment.Tracker`

Add a new runtime component in `internal/assessment` that is scoped to one `evidencePath`.

It stores per-session state:

- the session's evidence entries
- derived signal entries
- latest signal results
- total operation count
- the latest computed snapshot per scoring profile
- enough metadata to detect whether the on-disk store has diverged from the in-process cache

Core methods:

- `Observe(entry evidence.EvidenceEntry)`
- `Snapshot(sessionID string, profile score.Profile) (Snapshot, error)`

Behavior:

- cold path: the first request for a session builds state from the evidence store
- hot path: subsequent writes call `Observe`, and `Snapshot` returns from memory
- invalid path: if tracker metadata suggests external writes or a cache gap, rebuild the requested session from disk

### Cache Invalidation

The tracker is process-local only. It does not attempt cross-process synchronization.

It uses lightweight evidence-store metadata to detect divergence:

- manifest fingerprint
- entry count
- missing-session state when a session suddenly appears without having been observed

If the tracker cannot prove its cache is current, it falls back to rebuilding session state from disk before returning a snapshot.

This preserves correctness while keeping the normal MCP-server case fast.

### MCP Service Rename

Rename `BenchmarkService` to `MCPService`.

The type is not a benchmark subsystem. It is the core MCP-facing service that implements prescribe/report behavior. The rename is intended to reduce confusion when reading and extending the package.

The rename should be mechanical and local:

- production code
- tests
- constructors and helper types

### Forwarding Without Reread

Change the MCP write path so the freshly appended `EvidenceEntry` and its serialized JSON are available immediately after write.

Instead of:

- append entry
- `FindEntryByID`
- `json.Marshal`
- forward

the new flow becomes:

- build entry
- serialize once
- append
- forward raw JSON directly
- `tracker.Observe(entry)`

This removes a redundant lookup and keeps the hot path linear in new work only.

### Graceful Shutdown

Add an explicit `Close()` path on the MCP service and a cleanup-capable server constructor.

Responsibilities:

- stop `RetryTracker`
- make repeated cleanup safe
- keep the existing `NewServer` API usable for current callers

The new constructor should return both the MCP server and a cleanup handle so `cmd/evidra-mcp` can stop background goroutines during shutdown.

## Data Flow

### `prescribe`

1. Lifecycle service builds and appends the prescription entry.
2. MCP service receives the entry and raw JSON.
3. MCP service forwards raw JSON best-effort.
4. MCP service updates the assessment tracker with the new entry.

### `report`

1. Lifecycle service builds and appends the report entry.
2. MCP service receives the entry and raw JSON.
3. MCP service forwards raw JSON best-effort.
4. MCP service updates the assessment tracker with the new entry.
5. MCP service asks the tracker for `Snapshot(sessionID, profile)`.
6. Tracker returns a hot cached snapshot or rebuilds the requested session if invalidated.

### CLI `report`

CLI report should use the same tracker-backed assessment path so the CLI and MCP server do not diverge in performance or semantics.

## Error Handling

- Forwarding remains best-effort.
- Tracker observe failures should not hide a successful evidence write; if the in-memory update fails, invalidate the affected session and rebuild on next snapshot request.
- Snapshot rebuild failures should surface as internal assessment errors, preserving current external error behavior.
- Shutdown should be idempotent and return best-effort errors only if cleanup truly fails.

## Testing Strategy

Add tests before implementation for:

1. repeated report snapshots reuse incremental state instead of forcing repeated full rebuilds
2. tracker invalidation rebuilds correctly after external evidence writes
3. forwarding uses already-available raw entry bytes instead of post-write rereads
4. service/server cleanup stops the retry tracker
5. renamed MCP service preserves current prescribe/report behavior

Also keep broad regression coverage with:

- `go test ./pkg/mcpserver ./internal/assessment ./cmd/evidra -count=1`
- `go test ./... -count=1`

## Rollout

1. Introduce tracker behind tests.
2. Rename the MCP service type and update references.
3. Refactor write/forward path to pass entry data directly.
4. Switch MCP report and CLI report to the tracker-backed snapshot path.
5. Add cleanup-capable constructor and wire `cmd/evidra-mcp` to use it.

## Expected Outcome

After this change:

- MCP `report` no longer performs a full evidence-store scan on every call in the steady state
- CLI `report` uses the same faster path
- MCP code reads more clearly with `MCPService`
- forwarding avoids unnecessary rereads
- retry tracker goroutines have an explicit shutdown path
