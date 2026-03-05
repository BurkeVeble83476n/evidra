# v0.3.0 Cleanup + CLI Wiring — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Execute all Immediate cleanup items and v0.3.x CLI wiring from the Post-Implementation Review. Put v0.5.0 items in backlog.

**Architecture:** Two phases — Phase 1 removes dead code/docs (safe, no logic changes). Phase 2 wires CLI prescribe/report/compare to the evidence store, adds actor to ReportInput, and integrates SARIF findings into CLI.

**Tech Stack:** Go 1.24, existing evidence/pipeline/signal/score packages.

**Source of truth:** `docs/system-design/CHIEF_ARCHITECT_POST_IMPLEMENTATION_REVIEW.md`

---

## Phase 1: Immediate Cleanup (before v0.3.0 tag)

### Task 1: Remove StoreManifest.PolicyRef vestigial field

**Files:**
- Modify: `pkg/evidence/types.go:19` — remove PolicyRef field
- Modify: `pkg/evidence/manifest.go:73` — remove PolicyRef initialization

**Step 1: Edit types.go**

Remove this line from StoreManifest:
```go
PolicyRef       string   `json:"policy_ref"`
```

**Step 2: Edit manifest.go**

Remove this line from the manifest initialization:
```go
PolicyRef:       "",
```

**Step 3: Run tests, gofmt, commit**

```bash
gofmt -w pkg/evidence/types.go pkg/evidence/manifest.go
go test ./pkg/evidence/ -v -count=1
git add pkg/evidence/types.go pkg/evidence/manifest.go
git commit -m "fix: remove vestigial PolicyRef from StoreManifest"
```

---

### Task 2: Delete stale docs and archive completed reviews

**Step 1: Delete missing_logic.md**

```bash
rm docs/system-design/missing_logic.md
```

**Step 2: Move completed reviews to done/**

```bash
mv docs/system-design/ARCHITECTURAL_REVIEW_AND_RECOMMENDATION.md docs/system-design/done/
mv docs/system-design/CHIEF_ARCHITECT_FINAL_REVIEW.md docs/system-design/done/
```

**Step 3: Commit**

```bash
git add docs/system-design/
git commit -m "docs: archive completed reviews and delete stale missing_logic.md"
```

---

### Task 3: Remove stub notes from CLI commands

**Files:**
- Modify: `cmd/evidra/main.go`

**Step 1: Remove note from cmdReport**

In `cmdReport()`, remove this line (line 302):
```go
"note":            "evidence chain recording requires --evidence-dir",
```

**Step 2: Remove note and stub from cmdCompare**

In `cmdCompare()`, remove this line (line 218):
```go
"note":    "load evidence chain for real comparison",
```

**Step 3: Run build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go
go build ./cmd/evidra/
git add cmd/evidra/main.go
git commit -m "fix: remove placeholder notes from CLI report and compare commands"
```

---

### Task 4: Commit .gitignore update

**Step 1: Commit**

```bash
git add .gitignore
git commit -m "chore: add .evidra.lock to .gitignore"
```

---

### Task 5: Create v0.5.0 implementation backlog

**Files:**
- Create: `docs/system-design/backlog/V050_IMPLEMENTATION_BACKLOG.md`

**Step 1: Write backlog document**

```markdown
# v0.5.0 Implementation Backlog

**Source:** CHIEF_ARCHITECT_POST_IMPLEMENTATION_REVIEW.md §5 (v0.5.0 tier)

## Items

### 1. Wire Ed25519 Signing into Evidence Pipeline

**Current state:** `internal/evidence/signer.go` has a complete Ed25519 signing
module (149 LOC, 14 tests). `EvidenceEntry.Signature` field exists but is always
empty string. No integration point in `BuildEntry()` or MCP server.

**Work required:**
- Add `Signer` parameter to `BuildEntry()` or wrap it
- Consume `EVIDRA_SIGNING_KEY` / `EVIDRA_SIGNING_KEY_PATH` env vars in MCP server
- Populate `Signature` field on every evidence entry
- Add verification in `ValidateChainAtPath`
- Update CLI scorecard to verify signatures if present

**Effort:** ~2 days

### 2. Forward Integrity + Server Receipts

**Current state:** `EntryTypeReceipt` exists as an enum value but no code
path creates receipt entries. `--forward-url` was removed in v0.3.0.

**Work required:**
- Add `EVIDRA_API_URL` config back to MCP server
- Implement HTTP forwarder (POST evidence entries to remote API)
- Remote API returns signed receipt → write as `receipt` entry
- Receipt entry links back to forwarded entry by entry_id

**Effort:** ~3 days

### 3. Actor auth_context / OIDC

**Current state:** `Actor` struct has `Type`, `ID`, `Provenance`. No
authentication or identity verification.

**Work required:**
- Add `AuthContext` field to Actor (optional JWT/OIDC token reference)
- MCP server validates token if present
- Evidence entries carry verified actor identity
- Confidence model considers actor verification level

**Effort:** ~3 days

### 4. Multi-Tenancy Enforcement

**Current state:** `EvidenceEntry.TenantID` field exists (omitempty).
No enforcement or isolation logic.

**Work required:**
- MCP server requires tenant_id in service mode
- Evidence store partitions by tenant_id
- Scorecard filters by tenant_id
- Cross-tenant access prevention

**Effort:** ~3 days
```

**Step 2: Commit**

```bash
git add docs/system-design/backlog/V050_IMPLEMENTATION_BACKLOG.md
git commit -m "docs: add v0.5.0 implementation backlog"
```

---

## Phase 2: v0.3.x CLI Wiring

### Task 6: Add Actor to MCP ReportInput

**Files:**
- Modify: `pkg/mcpserver/server.go` — add optional Actor to ReportInput, use if provided
- Modify: `pkg/mcpserver/schemas/report.schema.json` — add actor property
- Test: `pkg/mcpserver/integration_test.go` — add cross-actor test

**Step 1: Add Actor to ReportInput struct**

```go
type ReportInput struct {
	PrescriptionID string     `json:"prescription_id"`
	ExitCode       int        `json:"exit_code"`
	ArtifactDigest string     `json:"artifact_digest,omitempty"`
	Actor          InputActor `json:"actor"`
}
```

**Step 2: Update Report() to use input actor**

In `Report()`, replace `s.lastActor` with actor resolution:

```go
// Use actor from input if provided, fall back to lastActor from prescribe
actor := s.lastActor
if input.Actor.ID != "" {
    actor = evidence.Actor{
        Type:       input.Actor.Type,
        ID:         input.Actor.ID,
        Provenance: input.Actor.Origin,
    }
}
```

**Step 3: Add test for explicit report actor**

Add to `integration_test.go`:

```go
func TestReport_ExplicitActor(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := &BenchmarkService{
		evidencePath: dir,
		traceID:      "01TRACE_ACTOR",
	}

	prescOutput := svc.Prescribe(PrescribeInput{
		Actor:       InputActor{Type: "ai_agent", ID: "agent-1", Origin: "mcp"},
		Tool:        "kubectl",
		Operation:   "apply",
		RawArtifact: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n  namespace: default",
	})

	// Report with explicit different actor
	reportOutput := svc.Report(ReportInput{
		PrescriptionID: prescOutput.PrescriptionID,
		ExitCode:       0,
		Actor:          InputActor{Type: "ai_agent", ID: "agent-2", Origin: "mcp"},
	})

	if !reportOutput.OK {
		t.Fatalf("report failed: %v", reportOutput.Error)
	}

	entries, _ := evidence.ReadAllEntriesAtPath(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Actor.ID != "agent-2" {
		t.Errorf("report actor: got %q, want agent-2", entries[1].Actor.ID)
	}
}
```

**Step 4: Update report schema**

Add actor to `pkg/mcpserver/schemas/report.schema.json`.

**Step 5: Run tests, gofmt, commit**

```bash
gofmt -w pkg/mcpserver/server.go pkg/mcpserver/integration_test.go
go test ./pkg/mcpserver/ -v -count=1
git add pkg/mcpserver/
git commit -m "feat: add optional actor field to MCP ReportInput"
```

---

### Task 7: Wire CLI prescribe to evidence store

**Files:**
- Modify: `cmd/evidra/main.go` — rewrite `cmdPrescribe()` to write evidence

**Step 1: Rewrite cmdPrescribe**

Add `--evidence-dir` and `--actor` flags. After canonicalization and risk assessment, build and append an EvidenceEntry:

```go
func cmdPrescribe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prescribe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	artifactFlag := fs.String("artifact", "", "Path to artifact file (YAML or JSON)")
	toolFlag := fs.String("tool", "", "Tool name (kubectl, terraform)")
	operationFlag := fs.String("operation", "apply", "Operation (apply, delete, plan)")
	envFlag := fs.String("environment", "", "Environment (production, staging, development)")
	evidenceFlag := fs.String("evidence-dir", "", "Evidence directory")
	actorFlag := fs.String("actor", "", "Actor ID (e.g. ci-pipeline-123)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *artifactFlag == "" || *toolFlag == "" {
		fmt.Fprintln(stderr, "prescribe requires --artifact and --tool")
		return 2
	}

	data, err := os.ReadFile(*artifactFlag)
	if err != nil {
		fmt.Fprintf(stderr, "read artifact: %v\n", err)
		return 1
	}

	cr := canon.Canonicalize(*toolFlag, *operationFlag, *envFlag, data)
	riskTags := risk.RunAll(cr.CanonicalAction, data)
	riskLevel := risk.RiskLevel(cr.CanonicalAction.OperationClass, cr.CanonicalAction.ScopeClass)

	// Determine actor
	actorID := *actorFlag
	if actorID == "" {
		actorID = "cli"
	}
	actor := evidence.Actor{Type: "cli", ID: actorID, Provenance: "cli"}

	// Build prescription payload
	prescriptionID := evidence.GenerateTraceID() // ULID
	canonSource := "adapter"

	var canonActionJSON json.RawMessage
	if cr.RawAction != nil {
		canonActionJSON = cr.RawAction
	} else {
		canonActionJSON, _ = json.Marshal(cr.CanonicalAction)
	}

	prescPayload := evidence.PrescriptionPayload{
		PrescriptionID:  prescriptionID,
		CanonicalAction: canonActionJSON,
		RiskLevel:       riskLevel,
		RiskTags:        riskTags,
		TTLMs:           evidence.DefaultTTLMs,
		CanonSource:     canonSource,
	}

	// Handle parse error — write canon failure if evidence dir set
	evidencePath := resolveEvidencePath(*evidenceFlag)
	if cr.ParseError != nil {
		if evidencePath != "" {
			failPayload, _ := json.Marshal(evidence.CanonFailurePayload{
				ErrorCode:    "parse_error",
				ErrorMessage: cr.ParseError.Error(),
				Adapter:      cr.CanonVersion,
				RawDigest:    cr.ArtifactDigest,
			})
			lastHash, _ := evidence.LastHashAtPath(evidencePath)
			entry, buildErr := evidence.BuildEntry(evidence.EntryBuildParams{
				Type:           evidence.EntryTypeCanonFailure,
				TraceID:        prescriptionID,
				Actor:          actor,
				ArtifactDigest: cr.ArtifactDigest,
				Payload:        failPayload,
				PreviousHash:   lastHash,
				SpecVersion:    "0.3.0",
				AdapterVersion: version.Version,
			})
			if buildErr == nil {
				evidence.AppendEntryAtPath(evidencePath, entry)
			}
		}

		result := map[string]interface{}{
			"ok":          false,
			"parse_error": cr.ParseError.Error(),
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		return 1
	}

	payloadJSON, _ := json.Marshal(prescPayload)

	// Write evidence entry
	lastHash, _ := evidence.LastHashAtPath(evidencePath)
	entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
		Type:           evidence.EntryTypePrescribe,
		TraceID:        prescriptionID,
		Actor:          actor,
		IntentDigest:   cr.IntentDigest,
		ArtifactDigest: cr.ArtifactDigest,
		Payload:        payloadJSON,
		PreviousHash:   lastHash,
		SpecVersion:    "0.3.0",
		CanonVersion:   cr.CanonVersion,
		AdapterVersion: version.Version,
	})
	if err != nil {
		fmt.Fprintf(stderr, "build entry: %v\n", err)
		return 1
	}

	if err := evidence.AppendEntryAtPath(evidencePath, entry); err != nil {
		fmt.Fprintf(stderr, "write evidence: %v\n", err)
		return 1
	}

	result := map[string]interface{}{
		"ok":              true,
		"prescription_id": entry.EntryID,
		"risk_level":      riskLevel,
		"risk_tags":       riskTags,
		"artifact_digest": cr.ArtifactDigest,
		"intent_digest":   cr.IntentDigest,
		"operation_class": cr.CanonicalAction.OperationClass,
		"scope_class":     cr.CanonicalAction.ScopeClass,
		"canon_version":   cr.CanonVersion,
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode prescription: %v\n", err)
		return 1
	}
	return 0
}
```

**Step 2: Run build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go
go build ./cmd/evidra/
go test ./... -count=1
git add cmd/evidra/main.go
git commit -m "feat: wire CLI prescribe to evidence store"
```

---

### Task 8: Wire CLI report to evidence store

**Files:**
- Modify: `cmd/evidra/main.go` — rewrite `cmdReport()` to write evidence

**Step 1: Rewrite cmdReport**

```go
func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	prescriptionFlag := fs.String("prescription", "", "Prescription event ID")
	exitCodeFlag := fs.Int("exit-code", 0, "Exit code of the operation")
	evidenceFlag := fs.String("evidence-dir", "", "Evidence directory")
	actorFlag := fs.String("actor", "", "Actor ID")
	artifactDigestFlag := fs.String("artifact-digest", "", "Artifact digest for drift detection")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *prescriptionFlag == "" {
		fmt.Fprintln(stderr, "report requires --prescription")
		return 2
	}

	evidencePath := resolveEvidencePath(*evidenceFlag)

	// Look up prescription
	_, found, err := evidence.FindEntryByID(evidencePath, *prescriptionFlag)
	if err != nil {
		fmt.Fprintf(stderr, "read evidence: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "prescription %s not found in evidence\n", *prescriptionFlag)
		return 1
	}

	// Determine actor
	actorID := *actorFlag
	if actorID == "" {
		actorID = "cli"
	}
	actor := evidence.Actor{Type: "cli", ID: actorID, Provenance: "cli"}

	// Build report payload
	reportID := evidence.GenerateTraceID()
	reportPayload := evidence.ReportPayload{
		ReportID:       reportID,
		PrescriptionID: *prescriptionFlag,
		ExitCode:       *exitCodeFlag,
		Verdict:        evidence.VerdictFromExitCode(*exitCodeFlag),
	}
	payloadJSON, _ := json.Marshal(reportPayload)

	lastHash, _ := evidence.LastHashAtPath(evidencePath)
	entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
		Type:           evidence.EntryTypeReport,
		TraceID:        reportID,
		Actor:          actor,
		ArtifactDigest: *artifactDigestFlag,
		Payload:        payloadJSON,
		PreviousHash:   lastHash,
		SpecVersion:    "0.3.0",
		AdapterVersion: version.Version,
	})
	if err != nil {
		fmt.Fprintf(stderr, "build entry: %v\n", err)
		return 1
	}

	if err := evidence.AppendEntryAtPath(evidencePath, entry); err != nil {
		fmt.Fprintf(stderr, "write evidence: %v\n", err)
		return 1
	}

	result := map[string]interface{}{
		"ok":              true,
		"report_id":       entry.EntryID,
		"prescription_id": *prescriptionFlag,
		"exit_code":       *exitCodeFlag,
		"verdict":         reportPayload.Verdict,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode report: %v\n", err)
		return 1
	}
	return 0
}
```

**Step 2: Run build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go
go build ./cmd/evidra/
go test ./... -count=1
git add cmd/evidra/main.go
git commit -m "feat: wire CLI report to evidence store"
```

---

### Task 9: Add --scanner-report flag to CLI prescribe

**Files:**
- Modify: `cmd/evidra/main.go` — add SARIF finding ingestion after prescribe

**Step 1: Add flag and integration**

In `cmdPrescribe()`, add:

```go
scannerFlag := fs.String("scanner-report", "", "SARIF scanner report file")
```

After writing the prescription entry, if scanner report is provided:

```go
// Write scanner findings as evidence entries
if *scannerFlag != "" {
	sarifData, err := os.ReadFile(*scannerFlag)
	if err != nil {
		fmt.Fprintf(stderr, "read scanner report: %v\n", err)
		return 1
	}
	findings, err := sarif.Parse(sarifData)
	if err != nil {
		fmt.Fprintf(stderr, "parse scanner report: %v\n", err)
		return 1
	}
	for _, f := range findings {
		findingPayload, _ := json.Marshal(f)
		lastHash, _ = evidence.LastHashAtPath(evidencePath)
		findingEntry, err := evidence.BuildEntry(evidence.EntryBuildParams{
			Type:           evidence.EntryTypeFinding,
			TraceID:        prescriptionID,
			Actor:          actor,
			ArtifactDigest: cr.ArtifactDigest,
			Payload:        findingPayload,
			PreviousHash:   lastHash,
			SpecVersion:    "0.3.0",
			AdapterVersion: version.Version,
		})
		if err != nil {
			continue
		}
		evidence.AppendEntryAtPath(evidencePath, findingEntry)
	}
	result["findings_count"] = len(findings)
}
```

Add import for `"samebits.com/evidra-benchmark/internal/sarif"`.

**Step 2: Run build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go
go build ./cmd/evidra/
go test ./... -count=1
git add cmd/evidra/main.go
git commit -m "feat: add --scanner-report flag for SARIF finding ingestion"
```

---

### Task 10: Wire cmdCompare to real evidence

**Files:**
- Modify: `cmd/evidra/main.go` — rewrite `cmdCompare()`
- Modify: `internal/score/compare.go` — add `BuildProfile` function

**Step 1: Add BuildProfile to compare.go**

```go
// BuildProfile builds a WorkloadProfile from signal entries.
func BuildProfile(entries []signal.Entry) WorkloadProfile {
	p := WorkloadProfile{
		Tools:  make(map[string]bool),
		Scopes: make(map[string]bool),
	}
	for _, e := range entries {
		if !e.IsPrescription {
			continue
		}
		if e.Tool != "" {
			p.Tools[e.Tool] = true
		}
		if e.ScopeClass != "" {
			p.Scopes[e.ScopeClass] = true
		}
	}
	return p
}
```

**Step 2: Rewrite cmdCompare**

```go
func cmdCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	actorsFlag := fs.String("actors", "", "Comma-separated actor IDs to compare")
	periodFlag := fs.String("period", "30d", "Time period (e.g. 30d)")
	evidenceFlag := fs.String("evidence-dir", "", "Evidence directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	actors := strings.Split(*actorsFlag, ",")
	if len(actors) < 2 {
		fmt.Fprintln(stderr, "compare requires at least 2 actors (--actors A,B)")
		return 2
	}

	evidencePath := resolveEvidencePath(*evidenceFlag)
	entries, err := evidence.ReadAllEntriesAtPath(evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading evidence: %v\n", err)
		return 1
	}

	type actorScore struct {
		ActorID  string              `json:"actor_id"`
		Score    float64             `json:"score"`
		Band     string              `json:"band"`
		TotalOps int                 `json:"total_operations"`
		Profile  score.WorkloadProfile `json:"workload_profile"`
	}

	var scorecards []actorScore
	for _, actorID := range actors {
		filtered := filterEntries(entries, actorID, *periodFlag)
		signalEntries, err := pipeline.EvidenceToSignalEntries(filtered)
		if err != nil {
			fmt.Fprintf(stderr, "Error converting evidence for %s: %v\n", actorID, err)
			return 1
		}
		totalOps := countPrescriptions(signalEntries)
		results := signal.AllSignals(signalEntries)
		sc := score.Compute(results, totalOps)
		profile := score.BuildProfile(signalEntries)

		scorecards = append(scorecards, actorScore{
			ActorID:  actorID,
			Score:    sc.Score,
			Band:     sc.Band,
			TotalOps: sc.TotalOperations,
			Profile:  profile,
		})
	}

	// Compute pairwise overlap
	var overlap float64
	if len(scorecards) >= 2 {
		overlap = score.WorkloadOverlap(scorecards[0].Profile, scorecards[1].Profile)
	}

	result := map[string]interface{}{
		"actors":           scorecards,
		"workload_overlap": overlap,
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode comparison: %v\n", err)
		return 1
	}
	return 0
}
```

**Step 3: Run build, gofmt, commit**

```bash
gofmt -w cmd/evidra/main.go internal/score/compare.go
go build ./cmd/evidra/
go test ./... -count=1
git add cmd/evidra/main.go internal/score/compare.go
git commit -m "feat: wire cmdCompare to real evidence with workload profiles"
```

---

### Task 11: Record architecture decisions in overview doc

**Files:**
- Modify: `docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md`

**Step 1: Add architecture decisions**

Find the "Key Decisions" section and append decisions AD-1 through AD-6 from the post-implementation review:

```markdown
### Post-Implementation Decisions (v0.3.0)

8. **MCP server was the sole evidence writer in early v0.3.0.** CLI commands now also write evidence (v0.3.x). Both paths produce identical `EvidenceEntry` format.

9. **trace_id = process/invocation lifetime.** MCP: ULID generated at `NewServer()`, shared across session. CLI: ULID generated per command invocation.

10. **Report actor resolution.** `ReportInput` accepts optional `actor` field. Falls back to `lastActor` from preceding prescribe call in MCP server. CLI requires explicit `--actor` or defaults to "cli".

11. **Signature field is placeholder (v0.3.0).** `EvidenceEntry.Signature` is always empty. Ed25519 `Signer` module exists in `internal/evidence/signer.go` but is not integrated. Signing deferred to v0.5.0.

12. **Compare command requires evidence history.** `evidra compare` reads evidence, builds per-actor workload profiles, computes Jaccard similarity. Requires sufficient data per actor to be meaningful.
```

**Step 2: Commit**

```bash
git add docs/system-design/EVIDRA_ARCHITECTURE_OVERVIEW.md
git commit -m "docs: record post-implementation architecture decisions (AD-1 through AD-6)"
```

---

### Task 12: Final verification

**Step 1: Build**

```bash
go build ./cmd/evidra/ ./cmd/evidra-mcp/
```

**Step 2: All tests pass**

```bash
go test ./... -v -count=1
```

**Step 3: Race detector**

```bash
go test -race ./...
```

**Step 4: Lint**

```bash
gofmt -l .
```

**Step 5: Count tests**

```bash
go test ./... -v -count=1 2>&1 | grep -c 'PASS:'
```

Target: 200+ tests passing.
