# v0.3.0 Architecture Alignment — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Align the codebase with the system design contract so that evidence schema, canonicalization, signals, and scorecard all match the normative specs.

**Architecture:** Seven sequential PRs migrate the data path from the legacy OPA/enforcer model to the inspector/benchmark model. PR-1 (evidence schema) is the foundation — everything depends on it. PR-7 (tests) can run in parallel after PR-1.

**Tech Stack:** Go 1.24, `github.com/oklog/ulid/v2`, `github.com/modelcontextprotocol/go-sdk`, `github.com/hashicorp/terraform-json`, Ed25519 signing, JSONL append-only storage.

**Source of truth:** `docs/system-design/CHIEF_ARCHITECT_FINAL_REVIEW.md` (findings C1-C7, H1-H6, M1-M5) and `docs/system-design/EVIDRA_CORE_DATA_MODEL.md` (normative schema).

---

## PR-1: Evidence Schema Migration

**Findings addressed:** C1, C6, C7, H1, H3, H4, H6
**Goal:** Single `EvidenceEntry` struct matching `EVIDRA_CORE_DATA_MODEL.md §5`. Delete all OPA-era types. Wire MCP prescribe/report to write new format.

**Dependency:** None (foundation PR)

### Task 1: Define the new EvidenceEntry type

**Files:**
- Create: `pkg/evidence/entry.go`
- Reference: `docs/system-design/EVIDRA_CORE_DATA_MODEL.md` §5

**Step 1: Write the failing test**

Create `pkg/evidence/entry_test.go`:

```go
package evidence

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvidenceEntry_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	entry := EvidenceEntry{
		EntryID:      "01HXYZ",
		PreviousHash: "",
		Hash:         "sha256:abc123",
		Signature:    "sig_placeholder",
		Type:         EntryTypePrescribe,
		TenantID:     "",
		TraceID:      "01HXYZ_TRACE",
		Actor: Actor{
			Type:       "ai_agent",
			ID:         "claude-agent-1",
			Provenance: "mcp",
		},
		Timestamp:       time.Now().UTC(),
		IntentDigest:    "sha256:intent123",
		ArtifactDigest:  "sha256:artifact456",
		SpecVersion:     "0.3.0",
		CanonVersion:    "k8s/v1",
		AdapterVersion:  "0.3.0",
		ScoringVersion:  "",
		Payload:         json.RawMessage(`{"prescription_id":"01HXYZ"}`),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got EvidenceEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.EntryID != entry.EntryID {
		t.Errorf("entry_id: got %q, want %q", got.EntryID, entry.EntryID)
	}
	if got.Type != EntryTypePrescribe {
		t.Errorf("type: got %q, want %q", got.Type, EntryTypePrescribe)
	}
	if got.Actor.ID != "claude-agent-1" {
		t.Errorf("actor.id: got %q, want %q", got.Actor.ID, "claude-agent-1")
	}
}

func TestEntryType_Validate(t *testing.T) {
	t.Parallel()

	valid := []EntryType{
		EntryTypePrescribe, EntryTypeReport, EntryTypeFinding,
		EntryTypeSignal, EntryTypeReceipt, EntryTypeCanonFailure,
	}
	for _, et := range valid {
		if !et.Valid() {
			t.Errorf("%q should be valid", et)
		}
	}

	if EntryType("invalid").Valid() {
		t.Error("'invalid' should not be valid")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/evidence/ -run TestEvidenceEntry -v -count=1`
Expected: FAIL — `EvidenceEntry` type not defined

**Step 3: Write minimal implementation**

Create `pkg/evidence/entry.go`:

```go
package evidence

import (
	"encoding/json"
	"time"
)

// EntryType is a closed enum. Adding a value requires spec version bump.
type EntryType string

const (
	EntryTypePrescribe   EntryType = "prescribe"
	EntryTypeReport      EntryType = "report"
	EntryTypeFinding     EntryType = "finding"
	EntryTypeSignal      EntryType = "signal"
	EntryTypeReceipt     EntryType = "receipt"
	EntryTypeCanonFailure EntryType = "canonicalization_failure"
)

var validEntryTypes = map[EntryType]bool{
	EntryTypePrescribe:    true,
	EntryTypeReport:       true,
	EntryTypeFinding:      true,
	EntryTypeSignal:       true,
	EntryTypeReceipt:      true,
	EntryTypeCanonFailure: true,
}

// Valid returns true if et is a known entry type.
func (et EntryType) Valid() bool {
	return validEntryTypes[et]
}

// Actor identifies who performed an action.
type Actor struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Provenance string `json:"provenance"`
}

// EvidenceEntry is the append-only event log entry.
// Every JSONL line is one EvidenceEntry.
// Schema: EVIDRA_CORE_DATA_MODEL.md §5.
type EvidenceEntry struct {
	EntryID      string          `json:"entry_id"`
	PreviousHash string          `json:"previous_hash"`
	Hash         string          `json:"hash"`
	Signature    string          `json:"signature"`
	Type         EntryType       `json:"type"`
	TenantID     string          `json:"tenant_id,omitempty"`
	TraceID      string          `json:"trace_id"`
	Actor        Actor           `json:"actor"`
	Timestamp    time.Time       `json:"timestamp"`
	IntentDigest    string       `json:"intent_digest,omitempty"`
	ArtifactDigest  string       `json:"artifact_digest,omitempty"`
	Payload      json.RawMessage `json:"payload"`

	SpecVersion    string `json:"spec_version"`
	CanonVersion   string `json:"canonical_version"`
	AdapterVersion string `json:"adapter_version"`
	ScoringVersion string `json:"scoring_version,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/evidence/ -run TestEvidenceEntry -v -count=1`
Expected: PASS

**Step 5: Run gofmt**

Run: `gofmt -w pkg/evidence/entry.go pkg/evidence/entry_test.go`

**Step 6: Commit**

```bash
git add pkg/evidence/entry.go pkg/evidence/entry_test.go
git commit -m "$(cat <<'EOF'
feat: add EvidenceEntry type matching CORE_DATA_MODEL §5

New append-only evidence entry envelope with typed payloads,
hash chain, and version fields. Closed EntryType enum.
EOF
)"
```

---

### Task 2: Define typed payload structs

**Files:**
- Create: `pkg/evidence/payloads.go`
- Test: `pkg/evidence/payloads_test.go`

**Step 1: Write the failing test**

Create `pkg/evidence/payloads_test.go`:

```go
package evidence

import (
	"encoding/json"
	"testing"
)

func TestPrescriptionPayload_Marshal(t *testing.T) {
	t.Parallel()

	p := PrescriptionPayload{
		PrescriptionID: "01HXYZ",
		CanonicalAction: json.RawMessage(`{"tool":"kubectl","operation_class":"mutate"}`),
		RiskLevel:       "medium",
		RiskTags:        []string{"k8s.privileged_container"},
		TTLMs:           300000,
		CanonSource:     "adapter",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got PrescriptionPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.PrescriptionID != "01HXYZ" {
		t.Errorf("prescription_id: got %q", got.PrescriptionID)
	}
	if got.TTLMs != 300000 {
		t.Errorf("ttl_ms: got %d, want 300000", got.TTLMs)
	}
	if got.CanonSource != "adapter" {
		t.Errorf("canon_source: got %q", got.CanonSource)
	}
}

func TestReportPayload_Marshal(t *testing.T) {
	t.Parallel()

	p := ReportPayload{
		ReportID:       "01HXYZ_RPT",
		PrescriptionID: "01HXYZ",
		ExitCode:       0,
		Verdict:        VerdictSuccess,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ReportPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Verdict != VerdictSuccess {
		t.Errorf("verdict: got %q", got.Verdict)
	}
}

func TestVerdictFromExitCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code int
		want Verdict
	}{
		{0, VerdictSuccess},
		{1, VerdictFailure},
		{-1, VerdictError},
		{137, VerdictFailure},
	}
	for _, tc := range cases {
		got := VerdictFromExitCode(tc.code)
		if got != tc.want {
			t.Errorf("VerdictFromExitCode(%d): got %q, want %q", tc.code, got, tc.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/evidence/ -run TestPrescription -v -count=1`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `pkg/evidence/payloads.go`:

```go
package evidence

import "encoding/json"

// Verdict is a closed enum for report outcomes.
type Verdict string

const (
	VerdictSuccess Verdict = "success"
	VerdictFailure Verdict = "failure"
	VerdictError   Verdict = "error"
)

// VerdictFromExitCode derives verdict from tool exit code.
func VerdictFromExitCode(code int) Verdict {
	switch {
	case code == 0:
		return VerdictSuccess
	case code < 0:
		return VerdictError
	default:
		return VerdictFailure
	}
}

// PrescriptionPayload is the type=prescribe entry payload.
type PrescriptionPayload struct {
	PrescriptionID string          `json:"prescription_id"`
	CanonicalAction json.RawMessage `json:"canonical_action"`
	RiskLevel      string          `json:"risk_level"`
	RiskTags       []string        `json:"risk_tags,omitempty"`
	RiskDetails    []string        `json:"risk_details,omitempty"`
	TTLMs          int64           `json:"ttl_ms"`
	CanonSource    string          `json:"canon_source"`
}

// ReportPayload is the type=report entry payload.
type ReportPayload struct {
	ReportID       string  `json:"report_id"`
	PrescriptionID string  `json:"prescription_id"`
	ExitCode       int     `json:"exit_code"`
	Verdict        Verdict `json:"verdict"`
}

// FindingPayload is the type=finding entry payload.
type FindingPayload struct {
	Tool     string `json:"tool"`
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Resource string `json:"resource"`
	Message  string `json:"message"`
}

// SignalPayload is the type=signal entry payload.
type SignalPayload struct {
	SignalName string   `json:"signal_name"`
	SubSignal  string   `json:"sub_signal,omitempty"`
	EntryRefs  []string `json:"entry_refs"`
	Details    string   `json:"details,omitempty"`
}

// CanonFailurePayload is the type=canonicalization_failure entry payload.
type CanonFailurePayload struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Adapter      string `json:"adapter"`
	RawDigest    string `json:"raw_digest"`
}
```

**Step 4: Run tests**

Run: `go test ./pkg/evidence/ -run "TestPrescription|TestReport|TestVerdict" -v -count=1`
Expected: PASS

**Step 5: gofmt + commit**

```bash
gofmt -w pkg/evidence/payloads.go pkg/evidence/payloads_test.go
git add pkg/evidence/payloads.go pkg/evidence/payloads_test.go
git commit -m "$(cat <<'EOF'
feat: add typed payload structs for all evidence entry types

PrescriptionPayload, ReportPayload, FindingPayload, SignalPayload,
CanonFailurePayload. Verdict enum with VerdictFromExitCode.
EOF
)"
```

---

### Task 3: Add entry builder with hash chain and digest formatting

**Files:**
- Create: `pkg/evidence/entry_builder.go`
- Test: `pkg/evidence/entry_builder_test.go`

**Step 1: Write the failing test**

Create `pkg/evidence/entry_builder_test.go`:

```go
package evidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildEntry_Prescribe(t *testing.T) {
	t.Parallel()

	payload := PrescriptionPayload{
		PrescriptionID: "01HXYZ",
		RiskLevel:      "low",
		TTLMs:          300000,
		CanonSource:    "adapter",
	}
	payloadJSON, _ := json.Marshal(payload)

	entry, err := BuildEntry(EntryBuildParams{
		Type:           EntryTypePrescribe,
		TraceID:        "01HTRACE",
		Actor:          Actor{Type: "ai_agent", ID: "test-agent", Provenance: "mcp"},
		IntentDigest:   "sha256:abc",
		ArtifactDigest: "sha256:def",
		Payload:        payloadJSON,
		PreviousHash:   "",
		SpecVersion:    "0.3.0",
		CanonVersion:   "k8s/v1",
		AdapterVersion: "0.3.0",
	})
	if err != nil {
		t.Fatalf("BuildEntry: %v", err)
	}

	if entry.EntryID == "" {
		t.Error("entry_id must not be empty")
	}
	if entry.Type != EntryTypePrescribe {
		t.Errorf("type: got %q", entry.Type)
	}
	if !strings.HasPrefix(entry.Hash, "sha256:") {
		t.Errorf("hash must have sha256: prefix, got %q", entry.Hash)
	}
	if !strings.HasPrefix(entry.IntentDigest, "sha256:") {
		t.Errorf("intent_digest must have sha256: prefix, got %q", entry.IntentDigest)
	}
	if entry.TraceID != "01HTRACE" {
		t.Errorf("trace_id: got %q", entry.TraceID)
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp must not be zero")
	}
}

func TestBuildEntry_HashChain(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{"report_id":"01RPT"}`)

	entry1, _ := BuildEntry(EntryBuildParams{
		Type:           EntryTypePrescribe,
		TraceID:        "01HTRACE",
		Actor:          Actor{Type: "ci", ID: "gh-actions", Provenance: "cli"},
		Payload:        payload,
		PreviousHash:   "",
		SpecVersion:    "0.3.0",
		CanonVersion:   "k8s/v1",
		AdapterVersion: "0.3.0",
	})

	entry2, _ := BuildEntry(EntryBuildParams{
		Type:           EntryTypeReport,
		TraceID:        "01HTRACE",
		Actor:          Actor{Type: "ci", ID: "gh-actions", Provenance: "cli"},
		Payload:        payload,
		PreviousHash:   entry1.Hash,
		SpecVersion:    "0.3.0",
		CanonVersion:   "k8s/v1",
		AdapterVersion: "0.3.0",
	})

	if entry2.PreviousHash != entry1.Hash {
		t.Errorf("previous_hash: got %q, want %q", entry2.PreviousHash, entry1.Hash)
	}
	if entry2.Hash == entry1.Hash {
		t.Error("entry hashes must differ")
	}
}

func TestFormatDigest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"abc123", "sha256:abc123"},
		{"sha256:abc123", "sha256:abc123"},
		{"", ""},
	}
	for _, tc := range cases {
		got := FormatDigest(tc.input)
		if got != tc.want {
			t.Errorf("FormatDigest(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/evidence/ -run "TestBuildEntry|TestFormatDigest" -v -count=1`
Expected: FAIL

**Step 3: Write implementation**

Create `pkg/evidence/entry_builder.go`:

```go
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// DefaultTTLMs is the default prescription TTL (5 minutes).
const DefaultTTLMs = 300000

// EntryBuildParams contains the inputs for building an evidence entry.
type EntryBuildParams struct {
	Type           EntryType
	TenantID       string
	TraceID        string
	Actor          Actor
	IntentDigest   string
	ArtifactDigest string
	Payload        json.RawMessage
	PreviousHash   string
	SpecVersion    string
	CanonVersion   string
	AdapterVersion string
	ScoringVersion string
}

// BuildEntry creates a new EvidenceEntry with generated ID, timestamp, and hash.
func BuildEntry(p EntryBuildParams) (EvidenceEntry, error) {
	if !p.Type.Valid() {
		return EvidenceEntry{}, fmt.Errorf("evidence.BuildEntry: invalid entry type %q", p.Type)
	}

	entry := EvidenceEntry{
		EntryID:        generateEntryID(),
		PreviousHash:   p.PreviousHash,
		Type:           p.Type,
		TenantID:       p.TenantID,
		TraceID:        p.TraceID,
		Actor:          p.Actor,
		Timestamp:      time.Now().UTC(),
		IntentDigest:   FormatDigest(p.IntentDigest),
		ArtifactDigest: FormatDigest(p.ArtifactDigest),
		Payload:        p.Payload,
		SpecVersion:    p.SpecVersion,
		CanonVersion:   p.CanonVersion,
		AdapterVersion: p.AdapterVersion,
		ScoringVersion: p.ScoringVersion,
	}

	hash, err := computeEntryHash(entry)
	if err != nil {
		return EvidenceEntry{}, fmt.Errorf("evidence.BuildEntry: %w", err)
	}
	entry.Hash = hash

	return entry, nil
}

// FormatDigest ensures a digest has the sha256: prefix.
func FormatDigest(d string) string {
	if d == "" {
		return ""
	}
	if strings.HasPrefix(d, "sha256:") {
		return d
	}
	return "sha256:" + d
}

func generateEntryID() string {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

func computeEntryHash(e EvidenceEntry) (string, error) {
	// Hash all fields except hash and signature.
	hashInput := struct {
		EntryID        string          `json:"entry_id"`
		PreviousHash   string          `json:"previous_hash"`
		Type           EntryType       `json:"type"`
		TenantID       string          `json:"tenant_id"`
		TraceID        string          `json:"trace_id"`
		Actor          Actor           `json:"actor"`
		Timestamp      time.Time       `json:"timestamp"`
		IntentDigest   string          `json:"intent_digest"`
		ArtifactDigest string          `json:"artifact_digest"`
		Payload        json.RawMessage `json:"payload"`
		SpecVersion    string          `json:"spec_version"`
		CanonVersion   string          `json:"canonical_version"`
		AdapterVersion string          `json:"adapter_version"`
		ScoringVersion string          `json:"scoring_version"`
	}{
		EntryID:        e.EntryID,
		PreviousHash:   e.PreviousHash,
		Type:           e.Type,
		TenantID:       e.TenantID,
		TraceID:        e.TraceID,
		Actor:          e.Actor,
		Timestamp:      e.Timestamp,
		IntentDigest:   e.IntentDigest,
		ArtifactDigest: e.ArtifactDigest,
		Payload:        e.Payload,
		SpecVersion:    e.SpecVersion,
		CanonVersion:   e.CanonVersion,
		AdapterVersion: e.AdapterVersion,
		ScoringVersion: e.ScoringVersion,
	}

	data, err := json.Marshal(hashInput)
	if err != nil {
		return "", fmt.Errorf("computeEntryHash: %w", err)
	}

	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/evidence/ -run "TestBuildEntry|TestFormatDigest" -v -count=1`
Expected: PASS

**Step 5: gofmt + commit**

```bash
gofmt -w pkg/evidence/entry_builder.go pkg/evidence/entry_builder_test.go
git add pkg/evidence/entry_builder.go pkg/evidence/entry_builder_test.go
git commit -m "$(cat <<'EOF'
feat: add evidence entry builder with hash chain and sha256: prefix

BuildEntry generates ULID entry_id, computes sha256-prefixed hash
from all fields except hash/signature, chains via previous_hash.
FormatDigest ensures consistent sha256: prefix on all digests.
EOF
)"
```

---

### Task 4: Add trace_id generation

**Files:**
- Create: `pkg/evidence/trace.go`
- Test: `pkg/evidence/trace_test.go`

**Step 1: Write the failing test**

```go
package evidence

import "testing"

func TestGenerateTraceID(t *testing.T) {
	t.Parallel()

	id := GenerateTraceID()
	if id == "" {
		t.Fatal("trace_id must not be empty")
	}
	if len(id) != 26 {
		t.Errorf("trace_id should be ULID (26 chars), got %d: %q", len(id), id)
	}

	id2 := GenerateTraceID()
	if id == id2 {
		t.Error("two trace_ids must differ")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/evidence/ -run TestGenerateTraceID -v -count=1`

**Step 3: Write implementation**

Create `pkg/evidence/trace.go`:

```go
package evidence

import (
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// GenerateTraceID creates a new trace_id as a ULID.
// MCP: call once at server startup. CLI: call once per command.
func GenerateTraceID() string {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
```

**Step 4: Run test, gofmt, commit**

```bash
go test ./pkg/evidence/ -run TestGenerateTraceID -v -count=1
gofmt -w pkg/evidence/trace.go pkg/evidence/trace_test.go
git add pkg/evidence/trace.go pkg/evidence/trace_test.go
git commit -m "feat: add trace_id generation (ULID per session/invocation)"
```

---

### Task 5: Wire MCP server to new evidence format

**Files:**
- Modify: `pkg/mcpserver/server.go` (rewrite Prescribe/Report to use new types)
- Modify: `pkg/mcpserver/schemas/report.schema.json` (add actor, trace_id)

**Step 1: Update ReportInput to include actor**

In `pkg/mcpserver/server.go`, find `ReportInput` struct and add `Actor` field:

```go
type ReportInput struct {
	PrescriptionID string           `json:"prescription_id"`
	ExitCode       int              `json:"exit_code"`
	ArtifactDigest string           `json:"artifact_digest"`
	Actor          invocation.Actor `json:"actor"`
}
```

**Step 2: Add trace_id to Server options and BenchmarkService**

Add `TraceID string` field to `BenchmarkService`. Generate in `NewServer`:

```go
type BenchmarkService struct {
	evidencePath string
	retryTracker *RetryTracker
	traceID      string
	environment  string
}
```

In `NewServer`, add: `svc.traceID = evidence.GenerateTraceID()`

**Step 3: Rewrite Prescribe to build EvidenceEntry**

Replace the legacy `evidence.Record` construction with:

```go
prescPayload := evidence.PrescriptionPayload{
	PrescriptionID: prescriptionID,
	CanonicalAction: cr.RawAction,
	RiskLevel:      riskLevel,
	RiskTags:       riskTags,
	TTLMs:          evidence.DefaultTTLMs,
	CanonSource:    canonSource,
}
payloadJSON, _ := json.Marshal(prescPayload)

entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
	Type:           evidence.EntryTypePrescribe,
	TraceID:        s.traceID,
	Actor:          evidence.Actor{Type: input.Actor.Type, ID: input.Actor.ID, Provenance: input.Actor.Origin},
	IntentDigest:   cr.IntentDigest,
	ArtifactDigest: cr.ArtifactDigest,
	Payload:        payloadJSON,
	PreviousHash:   lastHash,
	SpecVersion:    "0.3.0",
	CanonVersion:   cr.CanonVersion,
	AdapterVersion: version.Version,
})
```

**Step 4: Rewrite Report similarly**

```go
reportPayload := evidence.ReportPayload{
	ReportID:       reportID,
	PrescriptionID: input.PrescriptionID,
	ExitCode:       input.ExitCode,
	Verdict:        evidence.VerdictFromExitCode(input.ExitCode),
}
payloadJSON, _ := json.Marshal(reportPayload)

entry, err := evidence.BuildEntry(evidence.EntryBuildParams{
	Type:           evidence.EntryTypeReport,
	TraceID:        s.traceID,
	Actor:          evidence.Actor{Type: input.Actor.Type, ID: input.Actor.ID, Provenance: input.Actor.Origin},
	ArtifactDigest: evidence.FormatDigest(input.ArtifactDigest),
	Payload:        payloadJSON,
	PreviousHash:   lastHash,
	SpecVersion:    "0.3.0",
	CanonVersion:   "",
	AdapterVersion: version.Version,
})
```

**Step 5: Update evidence storage to write EvidenceEntry**

The existing `evidence.AppendAtPath` writes `EvidenceRecord`. This must be updated to accept `EvidenceEntry` — see Task 6.

**Step 6: Remove ForwardURL from Options**

Delete `ForwardURL string` from `Options`. Remove all references in `cmd/evidra-mcp/main.go` (`--forward-url` flag and `EVIDRA_API_URL` env var).

**Step 7: Run existing tests**

Run: `go test ./pkg/mcpserver/ -v -count=1`
Expected: Tests need updating — existing tests use old record format.

**Step 8: Update server tests**

Update `pkg/mcpserver/server_test.go` to assert on `EvidenceEntry` fields instead of `PolicyDecision`.

**Step 9: gofmt + commit**

```bash
gofmt -w pkg/mcpserver/server.go cmd/evidra-mcp/main.go
git add pkg/mcpserver/server.go pkg/mcpserver/server_test.go cmd/evidra-mcp/main.go \
       pkg/mcpserver/schemas/report.schema.json
git commit -m "$(cat <<'EOF'
feat: wire MCP server to new EvidenceEntry format

- Prescribe/Report now build EvidenceEntry with typed payloads
- Add trace_id (ULID, generated at server startup)
- Add actor to ReportInput for cross-actor validation
- Add ttl_ms and canon_source to prescription payload
- Remove dead ForwardURL config (v0.5.0 feature)
EOF
)"
```

---

### Task 6: Update evidence store to read/write EvidenceEntry

**Files:**
- Modify: `pkg/evidence/io.go` (JSONL read/write for EvidenceEntry)
- Modify: `pkg/evidence/evidence.go` (AppendAtPath, ReadAllAtPath, etc.)
- Modify: `pkg/evidence/hash.go` (chain validation for new format)

This is the biggest sub-task. The existing store mechanics (segments, manifest, locking) are reusable. Only the record type changes.

**Step 1: Add EvidenceEntry JSONL writer**

In `pkg/evidence/io.go`, add functions that write/read `EvidenceEntry` as JSONL:

```go
func WriteEntry(w io.Writer, entry EvidenceEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("evidence.WriteEntry: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func ReadEntry(line []byte) (EvidenceEntry, error) {
	var entry EvidenceEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return EvidenceEntry{}, fmt.Errorf("evidence.ReadEntry: %w", err)
	}
	return entry, nil
}
```

**Step 2: Update AppendAtPath to accept EvidenceEntry**

Change signature from `AppendAtPath(path string, rec Record)` to `AppendEntryAtPath(path string, entry EvidenceEntry)`. Keep old function as deprecated until all callers migrate.

**Step 3: Add ReadAllEntries**

```go
func ReadAllEntriesAtPath(path string) ([]EvidenceEntry, error)
func ForEachEntryAtPath(path string, fn func(EvidenceEntry) error) error
func FindEntryByID(path string, entryID string) (EvidenceEntry, bool, error)
```

**Step 4: Run all tests**

Run: `go test ./pkg/evidence/ -v -count=1`

**Step 5: Commit**

```bash
git add pkg/evidence/
git commit -m "$(cat <<'EOF'
feat: evidence store reads/writes EvidenceEntry format

New AppendEntryAtPath, ReadAllEntriesAtPath, ForEachEntryAtPath,
FindEntryByID functions. JSONL serialization with hash chain.
EOF
)"
```

---

### Task 7: Delete legacy types and dead code

**Files:**
- Delete: `internal/evidence/types.go`
- Delete: `internal/evidence/builder.go`
- Delete: `internal/evidence/decision.go`
- Delete: `internal/evidence/payload.go`
- Delete: `internal/evidence/builder_test.go`
- Delete: `internal/evidence/payload_test.go`
- Keep: `internal/evidence/signer.go` + `signer_test.go` (Ed25519 signing is reusable)
- Delete: `pkg/invocation/invocation.go` (replaced by `evidence.Actor`)
- Modify: `pkg/evidence/types.go` — remove `PolicyDecision`, `EvidenceRecord` (old format). Keep `StoreManifest`, `StoreError`, error vars.

**Step 1: Remove OPA-era fields from pkg/evidence/types.go**

Delete `PolicyDecision`, `ExecutionResult`, and old `EvidenceRecord` struct.
Keep `StoreManifest`, `StoreError`, `ErrChainInvalid`, `ErrCursorSegmentNotFound`.

**Step 2: Delete internal/evidence files except signer**

```bash
rm internal/evidence/types.go internal/evidence/builder.go \
   internal/evidence/decision.go internal/evidence/payload.go \
   internal/evidence/builder_test.go internal/evidence/payload_test.go
```

**Step 3: Check for compile errors**

Run: `go build ./...`
Fix any remaining references to deleted types.

**Step 4: Delete pkg/invocation/**

```bash
rm -r pkg/invocation/
```

Update `pkg/mcpserver/server.go` to use `evidence.Actor` directly instead of `invocation.Actor`.

**Step 5: Run all tests**

Run: `go test ./... -v -count=1`

**Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
refactor: remove OPA-era types and dead code

Delete PolicyDecision, legacy EvidenceRecord, internal/evidence
builder/payload/decision, pkg/invocation. Keep Ed25519 signer.
Zero OPA fields remain in runtime code paths.
EOF
)"
```

---

### Task 8: Add sha256: prefix to canon digests

**Files:**
- Modify: `internal/canon/k8s.go`
- Modify: `internal/canon/terraform.go`
- Modify: `internal/canon/generic.go`

**Step 1: Update sha256Hex to include prefix**

In each adapter, change digest output from raw hex to `sha256:` prefixed:

```go
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
```

Or centralize in a shared function in `internal/canon/types.go`:

```go
// SHA256Digest computes SHA256 and returns with sha256: prefix.
func SHA256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
```

**Step 2: Update golden test digests**

Run: `EVIDRA_UPDATE_GOLDEN=1 go test ./internal/canon/ -run TestGolden -v -count=1`
Verify digests now have `sha256:` prefix.

**Step 3: Run all tests**

Run: `go test ./... -v -count=1`

**Step 4: Commit**

```bash
git add internal/canon/ tests/golden/
git commit -m "$(cat <<'EOF'
fix: all canon digests use sha256: prefix per contract

artifact_digest, intent_digest, and resource_shape_hash now
all use sha256:<hex> format. Golden corpus updated.
EOF
)"
```

---

## PR-2: Canonicalization Contract Sync

**Findings addressed:** C3, C4, H2, A3
**Goal:** ScopeClass uses environment, intent_digest excludes shape_hash, risk matrix uses environment scope.

**Dependency:** PR-1

### Task 9: Fix ScopeClass — environment-based resolution

**Files:**
- Modify: `internal/canon/types.go` — rewrite `ScopeClass()`
- Modify: `internal/canon/types.go` — update `CanonResult` with environment support
- Test: `internal/canon/canon_test.go`

**Step 1: Write the failing test**

Add to `internal/canon/canon_test.go`:

```go
func TestScopeClass_EnvironmentBased(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		env        string
		namespaces []string
		want       string
	}{
		{"explicit production", "production", nil, "production"},
		{"explicit staging", "staging", nil, "staging"},
		{"namespace prod-api", "", []string{"prod-api"}, "production"},
		{"namespace staging-v2", "", []string{"staging-v2"}, "staging"},
		{"namespace dev-test", "", []string{"dev-test"}, "development"},
		{"namespace default", "", []string{"default"}, "unknown"},
		{"no namespace no env", "", nil, "unknown"},
		{"env overrides namespace", "production", []string{"dev-ns"}, "production"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resources := make([]ResourceID, len(tc.namespaces))
			for i, ns := range tc.namespaces {
				resources[i] = ResourceID{Namespace: ns, Kind: "Deployment", Name: "test"}
			}
			got := ResolveScopeClass(tc.env, resources)
			if got != tc.want {
				t.Errorf("ResolveScopeClass(%q, %v): got %q, want %q", tc.env, tc.namespaces, got, tc.want)
			}
		})
	}
}
```

**Step 2: Run test — fails**

Run: `go test ./internal/canon/ -run TestScopeClass_EnvironmentBased -v -count=1`

**Step 3: Implement ResolveScopeClass**

```go
// ResolveScopeClass returns environment-based scope.
// Priority: explicit env flag > namespace substring > "unknown".
func ResolveScopeClass(env string, resources []ResourceID) string {
	// Explicit environment wins.
	switch env {
	case "production", "staging", "development":
		return env
	}

	// Derive from namespace substrings.
	for _, r := range resources {
		ns := strings.ToLower(r.Namespace)
		if strings.Contains(ns, "prod") {
			return "production"
		}
	}
	for _, r := range resources {
		ns := strings.ToLower(r.Namespace)
		if strings.Contains(ns, "stag") {
			return "staging"
		}
	}
	for _, r := range resources {
		ns := strings.ToLower(r.Namespace)
		if strings.Contains(ns, "dev") {
			return "development"
		}
	}

	return "unknown"
}
```

**Step 4: Update adapters to call ResolveScopeClass instead of ScopeClass**

In k8s.go, terraform.go: replace `ScopeClass(identities)` with `ResolveScopeClass(environment, identities)`.

This requires passing `environment` through the adapter. Update `Adapter` interface:

```go
type Adapter interface {
	Name() string
	CanHandle(tool string) bool
	Canonicalize(tool, operation, environment string, rawArtifact []byte) (CanonResult, error)
}
```

**Step 5: Update golden corpus**

Run: `EVIDRA_UPDATE_GOLDEN=1 go test ./internal/canon/ -run TestGolden -v -count=1`
Review changes — scope_class values will change.

**Step 6: Run all tests, gofmt, commit**

```bash
go test ./... -v -count=1
gofmt -w internal/canon/
git add internal/canon/ tests/golden/
git commit -m "$(cat <<'EOF'
fix: ScopeClass uses environment-based resolution

Replace topology-based scope (single/namespace/cluster) with
environment-based scope (production/staging/development/unknown).
Resolution: explicit --env > namespace substring > unknown.
EOF
)"
```

---

### Task 10: Fix intent_digest — exclude resource_shape_hash

**Files:**
- Modify: `internal/canon/k8s.go`
- Modify: `internal/canon/terraform.go`
- Modify: `internal/canon/generic.go`
- Test: `internal/canon/canon_test.go`

**Step 1: Write the failing test**

```go
func TestIntentDigest_ExcludesShapeHash(t *testing.T) {
	t.Parallel()

	artifact1 := loadGolden(t, "k8s_deployment.yaml")

	// Canonicalize same artifact — should get same intent_digest
	adapter := &K8sAdapter{}
	r1, err := adapter.Canonicalize("kubectl", "apply", "", artifact1)
	if err != nil {
		t.Fatal(err)
	}

	// Manually change shape_hash on the result — intent_digest should NOT change
	r1Copy := r1
	r1Copy.CanonicalAction.ResourceShapeHash = "sha256:different"

	// Recompute intent digest from the modified action
	intentFields := IntentFields(r1Copy.CanonicalAction)
	intentJSON, _ := json.Marshal(intentFields)
	recomputedDigest := SHA256Digest(intentJSON)

	if recomputedDigest != r1.IntentDigest {
		t.Errorf("intent_digest changed when shape_hash changed — shape_hash is leaking into intent")
	}
}
```

**Step 2: Run test — fails**

Current implementation marshals full CanonicalAction including ResourceShapeHash.

**Step 3: Add IntentFields and fix digest computation**

```go
// IntentFields returns only the fields that contribute to intent_digest.
// resource_shape_hash is explicitly excluded.
type intentFieldsStruct struct {
	Tool             string       `json:"tool"`
	Operation        string       `json:"operation"`
	OperationClass   string       `json:"operation_class"`
	ResourceIdentity []ResourceID `json:"resource_identity"`
	ScopeClass       string       `json:"scope_class"`
	ResourceCount    int          `json:"resource_count"`
}

func IntentFields(a CanonicalAction) intentFieldsStruct {
	return intentFieldsStruct{
		Tool:             a.Tool,
		Operation:        a.Operation,
		OperationClass:   a.OperationClass,
		ResourceIdentity: a.ResourceIdentity,
		ScopeClass:       a.ScopeClass,
		ResourceCount:    a.ResourceCount,
	}
}
```

Update all adapters' `Canonicalize` to:

```go
intentJSON, _ := json.Marshal(IntentFields(action))
intentDigest := SHA256Digest(intentJSON)
```

**Step 4: Update golden digests**

Run: `EVIDRA_UPDATE_GOLDEN=1 go test ./internal/canon/ -run TestGolden -v -count=1`

**Step 5: Bump canon version if identity changes**

If intent_digest values changed (they will), bump canon version to indicate breaking change. Update `CanonVersion` in each adapter.

**Step 6: gofmt + commit**

```bash
gofmt -w internal/canon/
git add internal/canon/ tests/golden/
git commit -m "$(cat <<'EOF'
fix: intent_digest excludes resource_shape_hash

Extract IntentFields() that returns only identity fields
(tool, operation, operation_class, resource_identity, scope_class,
resource_count). resource_shape_hash excluded per contract §Digest Rules.
Golden corpus updated with new intent_digest values.
EOF
)"
```

---

### Task 11: Fix risk matrix — environment-based scope

**Files:**
- Modify: `internal/risk/matrix.go`
- Test: `internal/risk/risk_test.go`

**Step 1: Write the failing test**

```go
func TestRiskLevel_EnvironmentScope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		opClass string
		scope   string
		want    string
	}{
		{"read", "production", "low"},
		{"mutate", "production", "high"},
		{"mutate", "staging", "medium"},
		{"mutate", "development", "low"},
		{"mutate", "unknown", "medium"},
		{"destroy", "production", "critical"},
		{"destroy", "staging", "high"},
		{"destroy", "development", "medium"},
		{"destroy", "unknown", "high"},
		{"plan", "production", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.opClass+"_"+tc.scope, func(t *testing.T) {
			t.Parallel()
			got := RiskLevel(tc.opClass, tc.scope)
			if got != tc.want {
				t.Errorf("RiskLevel(%q, %q): got %q, want %q", tc.opClass, tc.scope, got, tc.want)
			}
		})
	}
}
```

**Step 2: Run test — fails** (old matrix uses single/namespace/cluster)

**Step 3: Update risk matrix**

```go
var riskMatrix = map[string]map[string]string{
	"read":    {"production": "low", "staging": "low", "development": "low", "unknown": "low"},
	"mutate":  {"production": "high", "staging": "medium", "development": "low", "unknown": "medium"},
	"destroy": {"production": "critical", "staging": "high", "development": "medium", "unknown": "high"},
	"plan":    {"production": "low", "staging": "low", "development": "low", "unknown": "low"},
}
```

**Step 4: Run tests, gofmt, commit**

```bash
go test ./internal/risk/ -v -count=1
gofmt -w internal/risk/matrix.go
git add internal/risk/
git commit -m "$(cat <<'EOF'
fix: risk matrix uses environment-based scope

Replace topology scope (single/namespace/cluster) with
environment scope (production/staging/development/unknown).
Destroy+production → critical. Mutate+production → high.
EOF
)"
```

---

## PR-3: Signal Detector Alignment

**Findings addressed:** C5
**Goal:** All five detectors match `EVIDRA_SIGNAL_SPEC.md` exactly.

**Dependency:** PR-2

### Task 12: Fix retry_loop detector

**Files:**
- Modify: `internal/signal/retry_loop.go`
- Test: `internal/signal/signal_test.go`

**Step 1: Write the failing test**

```go
func TestRetryLoop_RequiresPriorFailure(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		// 3 prescriptions with same intent, but no failures → NOT a retry loop
		{EventID: "p1", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ActorID: "agent-1", ExitCode: intPtr(0), Timestamp: now.Add(1 * time.Minute)},
		{EventID: "p2", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now.Add(5 * time.Minute)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ActorID: "agent-1", ExitCode: intPtr(0), Timestamp: now.Add(6 * time.Minute)},
		{EventID: "p3", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now.Add(10 * time.Minute)},
		{EventID: "r3", IsReport: true, PrescriptionID: "p3", ActorID: "agent-1", ExitCode: intPtr(0), Timestamp: now.Add(11 * time.Minute)},
	}

	result := DetectRetryLoops(entries)
	if result.Count != 0 {
		t.Errorf("should not detect retry loop without prior failure, got %d events", result.Count)
	}
}

func TestRetryLoop_DetectsAfterFailure(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		// First attempt: fails
		{EventID: "p1", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ActorID: "agent-1", ExitCode: intPtr(1), Timestamp: now.Add(1 * time.Minute)},
		// Second attempt: same intent (retry)
		{EventID: "p2", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now.Add(5 * time.Minute)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ActorID: "agent-1", ExitCode: intPtr(1), Timestamp: now.Add(6 * time.Minute)},
		// Third attempt: same intent (retry loop detected at 3)
		{EventID: "p3", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now.Add(10 * time.Minute)},
	}

	result := DetectRetryLoops(entries)
	if result.Count == 0 {
		t.Error("should detect retry loop after failures")
	}
}

func TestRetryLoop_ScopesByActor(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		// Different actors doing same intent — NOT a retry loop
		{EventID: "p1", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ActorID: "agent-1", ExitCode: intPtr(1), Timestamp: now.Add(1 * time.Minute)},
		{EventID: "p2", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-2", Timestamp: now.Add(5 * time.Minute)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ActorID: "agent-2", ExitCode: intPtr(1), Timestamp: now.Add(6 * time.Minute)},
		{EventID: "p3", IsPrescription: true, IntentDigest: "sha256:same", ShapeHash: "sha256:shape", ActorID: "agent-1", Timestamp: now.Add(10 * time.Minute)},
	}

	result := DetectRetryLoops(entries)
	if result.Count != 0 {
		t.Errorf("different actors should not trigger retry loop, got %d", result.Count)
	}
}

func intPtr(i int) *int { return &i }
```

**Step 2: Run test — fails** (current impl doesn't require failure or scope by actor)

**Step 3: Rewrite retry_loop.go**

```go
const (
	DefaultRetryThreshold = 3
	DefaultRetryWindow    = 30 * time.Minute // spec: 30min, was 10min
)

func DetectRetryLoops(entries []Entry) SignalResult {
	return DetectRetryLoopsWithConfig(entries, DefaultRetryThreshold, DefaultRetryWindow)
}

func DetectRetryLoopsWithConfig(entries []Entry, threshold int, window time.Duration) SignalResult {
	// Build report lookup: prescription_id → exit_code
	reportExitCodes := make(map[string]int)
	for _, e := range entries {
		if e.IsReport && e.PrescriptionID != "" && e.ExitCode != nil {
			reportExitCodes[e.PrescriptionID] = *e.ExitCode
		}
	}

	// Group prescriptions by (actor, intent_digest, shape_hash) — scoped by actor
	type key struct{ actor, intent, shape string }
	groups := make(map[key][]Entry)
	for _, e := range entries {
		if !e.IsPrescription || e.IntentDigest == "" || e.ActorID == "" {
			continue
		}
		k := key{e.ActorID, e.IntentDigest, e.ShapeHash}
		groups[k] = append(groups[k], e)
	}

	var eventIDs []string
	for _, group := range groups {
		if len(group) < threshold {
			continue
		}

		sort.Slice(group, func(i, j int) bool {
			return group[i].Timestamp.Before(group[j].Timestamp)
		})

		// Only count retries AFTER a prior failed execution
		var retryChain []Entry
		for _, e := range group {
			exitCode, reported := reportExitCodes[e.EventID]
			if len(retryChain) == 0 {
				// First in chain: must have failed
				if reported && exitCode != 0 {
					retryChain = append(retryChain, e)
				}
				continue
			}

			// Subsequent: check window from first failure
			if e.Timestamp.Sub(retryChain[0].Timestamp) <= window {
				retryChain = append(retryChain, e)
			} else {
				// Window expired, reset
				retryChain = nil
				if reported && exitCode != 0 {
					retryChain = append(retryChain, e)
				}
			}
		}

		if len(retryChain) >= threshold {
			for _, e := range retryChain {
				eventIDs = append(eventIDs, e.EventID)
			}
		}
	}

	return SignalResult{Name: "retry_loop", Count: len(eventIDs), EventIDs: eventIDs}
}
```

**Step 4: Run tests, gofmt, commit**

```bash
go test ./internal/signal/ -run TestRetryLoop -v -count=1
gofmt -w internal/signal/retry_loop.go
git add internal/signal/
git commit -m "$(cat <<'EOF'
fix: retry_loop detector matches signal spec

- Window: 30min (was 10min)
- Requires prior failed execution before counting retries
- Scoped by actor (different actors = not a retry loop)
EOF
)"
```

---

### Task 13: Fix blast_radius detector

**Files:**
- Modify: `internal/signal/blast_radius.go`
- Test: `internal/signal/signal_test.go`

**Step 1: Write the failing test**

```go
func TestBlastRadius_DestroyOnlyThreshold5(t *testing.T) {
	t.Parallel()

	entries := []Entry{
		// destroy with 6 resources — should fire (threshold 5)
		{EventID: "p1", IsPrescription: true, OperationClass: "destroy", ResourceCount: 6},
		// mutate with 60 resources — should NOT fire (spec: destroy-only)
		{EventID: "p2", IsPrescription: true, OperationClass: "mutate", ResourceCount: 60},
		// destroy with 4 resources — should NOT fire (below threshold)
		{EventID: "p3", IsPrescription: true, OperationClass: "destroy", ResourceCount: 4},
	}

	result := DetectBlastRadius(entries)
	if result.Count != 1 {
		t.Errorf("expected 1 blast_radius event, got %d", result.Count)
	}
	if len(result.EventIDs) != 1 || result.EventIDs[0] != "p1" {
		t.Errorf("expected event p1, got %v", result.EventIDs)
	}
}
```

**Step 2: Run test — fails** (current: mutate=50, destroy=10)

**Step 3: Fix blast_radius.go**

```go
const BlastRadiusThreshold = 5 // Spec: destructive-only, threshold 5

func DetectBlastRadius(entries []Entry) SignalResult {
	var eventIDs []string
	for _, e := range entries {
		if !e.IsPrescription || e.ResourceCount == 0 {
			continue
		}
		if e.OperationClass == "destroy" && e.ResourceCount > BlastRadiusThreshold {
			eventIDs = append(eventIDs, e.EventID)
		}
	}
	return SignalResult{Name: "blast_radius", Count: len(eventIDs), EventIDs: eventIDs}
}
```

**Step 4: Run tests, gofmt, commit**

```bash
go test ./internal/signal/ -run TestBlastRadius -v -count=1
gofmt -w internal/signal/blast_radius.go
git add internal/signal/blast_radius.go
git commit -m "$(cat <<'EOF'
fix: blast_radius is destroy-only with threshold 5

Per signal spec: only destructive operations trigger blast_radius.
Threshold: 5 resources (was: destroy=10, mutate=50).
EOF
)"
```

---

### Task 14: Fix new_scope detector key

**Files:**
- Modify: `internal/signal/new_scope.go`
- Test: `internal/signal/signal_test.go`

**Step 1: Write the failing test**

```go
func TestNewScope_FullKey(t *testing.T) {
	t.Parallel()

	entries := []Entry{
		// Same tool+opClass but different actor+scopeClass → both new
		{EventID: "p1", IsPrescription: true, ActorID: "agent-1", Tool: "kubectl", OperationClass: "mutate", ScopeClass: "production"},
		{EventID: "p2", IsPrescription: true, ActorID: "agent-1", Tool: "kubectl", OperationClass: "mutate", ScopeClass: "staging"},
		{EventID: "p3", IsPrescription: true, ActorID: "agent-2", Tool: "kubectl", OperationClass: "mutate", ScopeClass: "production"},
		// Same combo as p1 — not new
		{EventID: "p4", IsPrescription: true, ActorID: "agent-1", Tool: "kubectl", OperationClass: "mutate", ScopeClass: "production"},
	}

	result := DetectNewScope(entries)
	if result.Count != 3 {
		t.Errorf("expected 3 new_scope events (p1,p2,p3), got %d: %v", result.Count, result.EventIDs)
	}
}
```

**Step 2: Run test — fails** (current key: tool+opClass only, missing actor+scopeClass)

**Step 3: Fix new_scope.go**

```go
func DetectNewScope(entries []Entry) SignalResult {
	type scopeKey struct {
		actor    string
		tool     string
		opClass  string
		scope    string
	}

	seen := make(map[scopeKey]bool)
	var eventIDs []string

	for _, e := range entries {
		if !e.IsPrescription {
			continue
		}
		k := scopeKey{e.ActorID, e.Tool, e.OperationClass, e.ScopeClass}
		if !seen[k] {
			seen[k] = true
			eventIDs = append(eventIDs, e.EventID)
		}
	}

	return SignalResult{Name: "new_scope", Count: len(eventIDs), EventIDs: eventIDs}
}
```

**Step 4: Run tests, gofmt, commit**

```bash
go test ./internal/signal/ -run TestNewScope -v -count=1
gofmt -w internal/signal/new_scope.go
git add internal/signal/new_scope.go
git commit -m "$(cat <<'EOF'
fix: new_scope key includes actor and scope_class

Key changed from (tool, opClass) to (actor, tool, opClass, scopeClass)
per signal spec §Signal 5.
EOF
)"
```

---

### Task 15: Add ScopeClass field to signal Entry and fix protocol_violation stalled/crash

**Files:**
- Modify: `internal/signal/types.go` — add `ScopeClass` field to `Entry`
- Modify: `internal/signal/protocol_violation.go` — verify stalled/crash classification
- Test: `internal/signal/signal_test.go`

**Step 1: Add ScopeClass to Entry**

```go
type Entry struct {
	// ... existing fields ...
	ScopeClass     string // production, staging, development, unknown
}
```

**Step 2: Verify protocol_violation classification**

Read `docs/system-design/EVIDRA_SIGNAL_SPEC.md` to confirm stalled_operation vs crash_before_report semantics. Review finding C5 in `CHIEF_ARCHITECT_FINAL_REVIEW.md`:

> stalled_operation vs crash_before_report classification is reversed relative to spec table

Fix the classification in `classifyUnreported()` to match spec.

**Step 3: Run all signal tests**

Run: `go test ./internal/signal/ -v -count=1`

**Step 4: Commit**

```bash
git add internal/signal/
git commit -m "$(cat <<'EOF'
fix: add ScopeClass to signal Entry, fix stalled/crash classification

ScopeClass field enables new_scope detector with full key.
Protocol violation stalled/crash classification aligned with
signal spec §Signal 1 table.
EOF
)"
```

---

## PR-4: Real Scorecard Pipeline

**Findings addressed:** C2
**Goal:** `evidra scorecard` reads evidence chain, computes signals, outputs real score.

**Dependency:** PR-3

### Task 16: Evidence-to-signal bridge

**Files:**
- Create: `internal/pipeline/bridge.go`
- Test: `internal/pipeline/bridge_test.go`

**Step 1: Write the failing test**

```go
package pipeline

import (
	"encoding/json"
	"testing"
	"time"

	"samebits.com/evidra-benchmark/internal/signal"
	"samebits.com/evidra-benchmark/pkg/evidence"
)

func TestEntriesToSignal(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	canonAction := json.RawMessage(`{"tool":"kubectl","operation_class":"mutate"}`)

	prescPayload, _ := json.Marshal(evidence.PrescriptionPayload{
		PrescriptionID:  "01PRESC",
		CanonicalAction: canonAction,
		RiskLevel:       "low",
		TTLMs:           300000,
		CanonSource:     "adapter",
	})

	entry := evidence.EvidenceEntry{
		EntryID:        "01PRESC",
		Type:           evidence.EntryTypePrescribe,
		TraceID:        "01TRACE",
		Actor:          evidence.Actor{Type: "ai_agent", ID: "agent-1", Provenance: "mcp"},
		Timestamp:      now,
		IntentDigest:   "sha256:abc",
		ArtifactDigest: "sha256:def",
		Payload:        prescPayload,
	}

	signalEntries, err := EvidenceToSignalEntries([]evidence.EvidenceEntry{entry})
	if err != nil {
		t.Fatalf("EvidenceToSignalEntries: %v", err)
	}

	if len(signalEntries) != 1 {
		t.Fatalf("expected 1 signal entry, got %d", len(signalEntries))
	}

	se := signalEntries[0]
	if se.EventID != "01PRESC" {
		t.Errorf("EventID: got %q", se.EventID)
	}
	if !se.IsPrescription {
		t.Error("should be prescription")
	}
	if se.ActorID != "agent-1" {
		t.Errorf("ActorID: got %q", se.ActorID)
	}
}
```

**Step 2: Implement bridge**

```go
package pipeline

import (
	"encoding/json"
	"fmt"

	"samebits.com/evidra-benchmark/internal/signal"
	"samebits.com/evidra-benchmark/pkg/evidence"
)

// EvidenceToSignalEntries converts evidence entries to signal detector input.
func EvidenceToSignalEntries(entries []evidence.EvidenceEntry) ([]signal.Entry, error) {
	var result []signal.Entry

	for _, e := range entries {
		se := signal.Entry{
			EventID:        e.EntryID,
			Timestamp:      e.Timestamp,
			ActorID:        e.Actor.ID,
			ArtifactDigest: e.ArtifactDigest,
			IntentDigest:   e.IntentDigest,
		}

		switch e.Type {
		case evidence.EntryTypePrescribe:
			se.IsPrescription = true
			var p evidence.PrescriptionPayload
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				return nil, fmt.Errorf("pipeline: unmarshal prescription %s: %w", e.EntryID, err)
			}
			// Extract fields from canonical_action
			var ca struct {
				Tool           string `json:"tool"`
				Operation      string `json:"operation"`
				OperationClass string `json:"operation_class"`
				ScopeClass     string `json:"scope_class"`
				ResourceCount  int    `json:"resource_count"`
				ShapeHash      string `json:"resource_shape_hash"`
			}
			if err := json.Unmarshal(p.CanonicalAction, &ca); err == nil {
				se.Tool = ca.Tool
				se.Operation = ca.Operation
				se.OperationClass = ca.OperationClass
				se.ScopeClass = ca.ScopeClass
				se.ResourceCount = ca.ResourceCount
				se.ShapeHash = ca.ShapeHash
			}
			se.RiskTags = p.RiskTags

		case evidence.EntryTypeReport:
			se.IsReport = true
			var r evidence.ReportPayload
			if err := json.Unmarshal(e.Payload, &r); err != nil {
				return nil, fmt.Errorf("pipeline: unmarshal report %s: %w", e.EntryID, err)
			}
			se.PrescriptionID = r.PrescriptionID
			exitCode := r.ExitCode
			se.ExitCode = &exitCode
		}

		result = append(result, se)
	}

	return result, nil
}
```

**Step 3: Run tests, gofmt, commit**

```bash
go test ./internal/pipeline/ -v -count=1
gofmt -w internal/pipeline/
git add internal/pipeline/
git commit -m "feat: add evidence-to-signal bridge for scorecard pipeline"
```

---

### Task 17: Wire CLI scorecard command

**Files:**
- Modify: `cmd/evidra/main.go` — rewrite `cmdScorecard()`

**Step 1: Rewrite cmdScorecard**

Replace the stub (`signal.AllSignals(nil)`) with:

```go
func cmdScorecard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scorecard", flag.ContinueOnError)
	actor := fs.String("actor", "", "Actor ID to score")
	period := fs.String("period", "30d", "Time period")
	evidencePath := fs.String("evidence-dir", resolveEvidencePath(""), "Evidence directory")
	// ...

	// Read evidence
	entries, err := evidence.ReadAllEntriesAtPath(*evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading evidence: %v\n", err)
		return 1
	}

	// Filter by actor and period
	filtered := filterEntries(entries, *actor, *period)

	// Convert to signal entries
	signalEntries, err := pipeline.EvidenceToSignalEntries(filtered)
	if err != nil {
		fmt.Fprintf(stderr, "Error converting evidence: %v\n", err)
		return 1
	}

	// Count prescriptions as total ops
	totalOps := countPrescriptions(signalEntries)

	// Run signals and compute score
	results := signal.AllSignals(signalEntries)
	sc := score.Compute(results, totalOps)

	// Output JSON with version metadata
	output := struct {
		score.Scorecard
		ActorID       string `json:"actor_id"`
		Period        string `json:"period"`
		ScoringVersion string `json:"scoring_version"`
		SpecVersion   string `json:"spec_version"`
		EvidraVersion string `json:"evidra_version"`
		GeneratedAt   string `json:"generated_at"`
	}{
		Scorecard:      sc,
		ActorID:        *actor,
		Period:         *period,
		ScoringVersion: "0.3.0",
		SpecVersion:    "0.3.0",
		EvidraVersion:  version.Version,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
	return 0
}
```

**Step 2: Add `evidra explain` command**

```go
func cmdExplain(args []string, stdout, stderr io.Writer) int {
	// Same as scorecard but outputs top signals with entry references
	// For each signal with Count > 0, list the entry_ids
}
```

**Step 3: Run build, test**

```bash
go build ./cmd/evidra/
go test ./cmd/evidra/ -v -count=1
```

**Step 4: Commit**

```bash
git add cmd/evidra/ internal/pipeline/
git commit -m "$(cat <<'EOF'
feat: wire real scorecard pipeline through evidence chain

evidra scorecard reads JSONL, converts to signal entries, runs
all 5 detectors, computes score with version metadata.
Add evidra explain command for signal-level detail.
EOF
)"
```

---

## PR-5: Confidence Model + Score Ceilings

**Findings addressed:** M1
**Goal:** Scorecard includes confidence and safety floors.

**Dependency:** PR-4

### Task 18: Add confidence computation

**Files:**
- Modify: `internal/score/score.go`
- Test: `internal/score/score_test.go`

**Step 1: Write the failing test**

```go
func TestScorecard_Confidence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		externalPct    float64
		violationRate  float64
		wantConfidence string
		wantCeiling    float64
	}{
		{"high confidence", 0.0, 0.0, "high", 100},
		{"medium - external heavy", 0.6, 0.0, "medium", 95},
		{"low - high violations", 0.0, 0.15, "low", 85},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conf := ComputeConfidence(tc.externalPct, tc.violationRate)
			if conf.Level != tc.wantConfidence {
				t.Errorf("level: got %q, want %q", conf.Level, tc.wantConfidence)
			}
			if conf.ScoreCeiling != tc.wantCeiling {
				t.Errorf("ceiling: got %.0f, want %.0f", conf.ScoreCeiling, tc.wantCeiling)
			}
		})
	}
}

func TestScorecard_SafetyFloors(t *testing.T) {
	t.Parallel()

	// Protocol violation rate > 10% → score capped at 90
	results := []signal.SignalResult{
		{Name: "protocol_violation", Count: 15},
		{Name: "artifact_drift", Count: 0},
		{Name: "retry_loop", Count: 0},
		{Name: "blast_radius", Count: 0},
		{Name: "new_scope", Count: 0},
	}
	sc := Compute(results, 100)
	if sc.Score > 90 {
		t.Errorf("protocol_violation > 10%% should cap score at 90, got %.1f", sc.Score)
	}
}
```

**Step 2: Implement confidence + safety floors**

```go
type Confidence struct {
	Level        string  // "high", "medium", "low"
	ScoreCeiling float64
}

func ComputeConfidence(externalPct, violationRate float64) Confidence {
	if violationRate > 0.10 {
		return Confidence{Level: "low", ScoreCeiling: 85}
	}
	if externalPct > 0.50 {
		return Confidence{Level: "medium", ScoreCeiling: 95}
	}
	return Confidence{Level: "high", ScoreCeiling: 100}
}
```

Add safety floors to `Compute()`:

```go
// Safety floors
if sc.Rates["protocol_violation"] > 0.10 && sc.Score > 90 {
	sc.Score = 90
}
if sc.Rates["artifact_drift"] > 0.05 && sc.Score > 85 {
	sc.Score = 85
}
sc.Band = scoreBand(sc.Score)
```

**Step 3: Run tests, gofmt, commit**

```bash
go test ./internal/score/ -v -count=1
gofmt -w internal/score/score.go
git add internal/score/
git commit -m "$(cat <<'EOF'
feat: add confidence model and safety floors to scoring

Confidence levels: high (no cap), medium (cap 95), low (cap 85).
Safety floors: violation_rate>10% caps at 90, drift_rate>5% caps at 85.
EOF
)"
```

---

## PR-6: Entry Types + SARIF

**Findings addressed:** M2, M3, M4
**Goal:** canonicalization_failure and finding entry types. SARIF parser.

**Dependency:** PR-4

### Task 19: Write canonicalization_failure entries on parse error

**Files:**
- Modify: `pkg/mcpserver/server.go` — on parse error, write evidence instead of just returning error

**Step 1: In Prescribe(), when adapter returns ParseError**

```go
if cr.ParseError != nil {
	// Write canonicalization_failure evidence entry
	failPayload, _ := json.Marshal(evidence.CanonFailurePayload{
		ErrorCode:    "parse_error",
		ErrorMessage: cr.ParseError.Error(),
		Adapter:      cr.CanonVersion,
		RawDigest:    evidence.FormatDigest(canon.SHA256Digest(rawArtifact)),
	})
	entry, _ := evidence.BuildEntry(evidence.EntryBuildParams{
		Type:           evidence.EntryTypeCanonFailure,
		TraceID:        s.traceID,
		Actor:          evidence.Actor{Type: input.Actor.Type, ID: input.Actor.ID, Provenance: input.Actor.Origin},
		ArtifactDigest: evidence.FormatDigest(canon.SHA256Digest(rawArtifact)),
		Payload:        failPayload,
		PreviousHash:   lastHash,
		SpecVersion:    "0.3.0",
		AdapterVersion: version.Version,
	})
	// Write entry to evidence
	evidence.AppendEntryAtPath(s.evidencePath, entry)

	return PrescribeOutput{OK: false, Error: &ErrInfo{Code: "parse_error", Message: cr.ParseError.Error()}}
}
```

**Step 2: Test, commit**

```bash
go test ./pkg/mcpserver/ -v -count=1
git add pkg/mcpserver/
git commit -m "feat: write canonicalization_failure evidence on parse error"
```

---

### Task 20: Add canon_source field

**Files:**
- Modify: `pkg/mcpserver/server.go` — set canon_source based on path

**Step 1: Determine canon_source**

```go
canonSource := "adapter"
if input.CanonicalAction != nil {
	canonSource = "external"
}
```

Already used in PrescriptionPayload from Task 5. Verify it's being set correctly.

**Step 2: Commit if changes needed**

```bash
git add pkg/mcpserver/
git commit -m "fix: ensure canon_source is set correctly (adapter/external)"
```

---

### Task 21: SARIF parser for scanner findings

**Files:**
- Create: `internal/sarif/parser.go`
- Create: `internal/sarif/parser_test.go`
- Create: `tests/testdata/sarif_checkov.json` (test fixture)

**Step 1: Write the failing test**

```go
package sarif

import (
	"os"
	"testing"
)

func TestParseSARIF_Checkov(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../tests/testdata/sarif_checkov.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	findings, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(findings) == 0 {
		t.Fatal("expected findings from SARIF")
	}

	f := findings[0]
	if f.Tool == "" {
		t.Error("tool should not be empty")
	}
	if f.RuleID == "" {
		t.Error("rule_id should not be empty")
	}
	if f.Severity == "" {
		t.Error("severity should not be empty")
	}
}
```

**Step 2: Implement SARIF parser**

```go
package sarif

import (
	"encoding/json"
	"fmt"
	"strings"

	"samebits.com/evidra-benchmark/pkg/evidence"
)

type sarifReport struct {
	Runs []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool struct {
		Driver struct {
			Name  string      `json:"name"`
			Rules []sarifRule `json:"rules"`
		} `json:"driver"`
	} `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifRule struct {
	ID string `json:"id"`
	DefaultConfiguration struct {
		Level string `json:"level"`
	} `json:"defaultConfiguration"`
}

type sarifResult struct {
	RuleID  string `json:"ruleId"`
	Level   string `json:"level"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	Locations []struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
		} `json:"physicalLocation"`
	} `json:"locations"`
}

// Parse extracts findings from SARIF JSON.
func Parse(data []byte) ([]evidence.FindingPayload, error) {
	var report sarifReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("sarif.Parse: %w", err)
	}

	var findings []evidence.FindingPayload
	for _, run := range report.Runs {
		toolName := run.Tool.Driver.Name
		for _, result := range run.Results {
			resource := ""
			if len(result.Locations) > 0 {
				resource = result.Locations[0].PhysicalLocation.ArtifactLocation.URI
			}
			findings = append(findings, evidence.FindingPayload{
				Tool:     strings.ToLower(toolName),
				RuleID:   result.RuleID,
				Severity: mapSeverity(result.Level),
				Resource: resource,
				Message:  result.Message.Text,
			})
		}
	}

	return findings, nil
}

func mapSeverity(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "note":
		return "low"
	default:
		return "info"
	}
}
```

**Step 3: Create test fixture**

Create `tests/testdata/sarif_checkov.json` with a minimal SARIF report.

**Step 4: Add `--scanner-report` flag to CLI prescribe**

In `cmd/evidra/main.go`, add flag to `cmdPrescribe`:

```go
scannerReport := fs.String("scanner-report", "", "SARIF scanner report file")
```

After prescribing, if scanner report provided, parse it and write finding entries.

**Step 5: Run tests, gofmt, commit**

```bash
go test ./internal/sarif/ -v -count=1
gofmt -w internal/sarif/
git add internal/sarif/ tests/testdata/ cmd/evidra/main.go
git commit -m "$(cat <<'EOF'
feat: SARIF parser for scanner findings

Parse Checkov/Trivy/tfsec SARIF output into FindingPayload.
Findings written as independent evidence entries (type=finding)
linked by artifact_digest. Add --scanner-report flag to CLI.
EOF
)"
```

---

## PR-7: Test Porting

**Findings addressed:** H5
**Goal:** Evidence store and MCP integration tests.

**Dependency:** PR-1 (can run in parallel with PR-2/3/4)

### Task 22: Port evidence chain integrity tests

**Files:**
- Create: `pkg/evidence/chain_test.go`

**Step 1: Write chain integrity tests**

```go
package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestChainIntegrity_AppendAndValidate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write 3 entries
	var lastHash string
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]string{"index": fmt.Sprintf("%d", i)})
		entry, err := BuildEntry(EntryBuildParams{
			Type:           EntryTypePrescribe,
			TraceID:        "01TRACE",
			Actor:          Actor{Type: "ci", ID: "test", Provenance: "cli"},
			Payload:        payload,
			PreviousHash:   lastHash,
			SpecVersion:    "0.3.0",
			CanonVersion:   "test/v1",
			AdapterVersion: "0.3.0",
		})
		if err != nil {
			t.Fatalf("BuildEntry %d: %v", i, err)
		}

		if err := AppendEntryAtPath(dir, entry); err != nil {
			t.Fatalf("AppendEntryAtPath %d: %v", i, err)
		}
		lastHash = entry.Hash
	}

	// Read back
	entries, err := ReadAllEntriesAtPath(dir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Validate chain
	if err := ValidateChainAtPath(dir); err != nil {
		t.Fatalf("ValidateChainAtPath: %v", err)
	}
}

func TestChainIntegrity_TamperDetection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write 2 entries
	payload, _ := json.Marshal(map[string]string{"data": "first"})
	entry1, _ := BuildEntry(EntryBuildParams{
		Type: EntryTypePrescribe, TraceID: "01T",
		Actor: Actor{Type: "ci", ID: "t", Provenance: "cli"},
		Payload: payload, SpecVersion: "0.3.0",
		CanonVersion: "test/v1", AdapterVersion: "0.3.0",
	})
	AppendEntryAtPath(dir, entry1)

	entry2, _ := BuildEntry(EntryBuildParams{
		Type: EntryTypeReport, TraceID: "01T",
		Actor: Actor{Type: "ci", ID: "t", Provenance: "cli"},
		Payload: payload, PreviousHash: entry1.Hash,
		SpecVersion: "0.3.0", CanonVersion: "test/v1", AdapterVersion: "0.3.0",
	})
	AppendEntryAtPath(dir, entry2)

	// Tamper: modify first entry's payload in the file
	// This should cause chain validation to fail
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) == 0 {
		t.Fatal("no JSONL files found")
	}

	data, _ := os.ReadFile(files[0])
	tampered := bytes.Replace(data, []byte(`"first"`), []byte(`"TAMPERED"`), 1)
	os.WriteFile(files[0], tampered, 0644)

	err := ValidateChainAtPath(dir)
	if err == nil {
		t.Fatal("expected chain validation error after tampering")
	}
}
```

**Step 2: Run tests**

Run: `go test ./pkg/evidence/ -run TestChainIntegrity -v -count=1`

**Step 3: Commit**

```bash
git add pkg/evidence/chain_test.go
git commit -m "test: add evidence chain integrity and tamper detection tests"
```

---

### Task 23: Port MCP integration tests

**Files:**
- Create: `pkg/mcpserver/integration_test.go`

**Step 1: Write prescribe/report lifecycle test**

```go
package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestPrescribeReport_Lifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := &BenchmarkService{
		evidencePath: dir,
		traceID:      "01TRACE_TEST",
	}

	// Prescribe
	prescOutput := svc.Prescribe(PrescribeInput{
		Actor:       Actor{Type: "ai_agent", ID: "test-agent", Provenance: "mcp"},
		Tool:        "kubectl",
		Operation:   "apply",
		RawArtifact: `apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test\n  namespace: staging`,
	})

	if !prescOutput.OK {
		t.Fatalf("prescribe failed: %v", prescOutput.Error)
	}
	if prescOutput.PrescriptionID == "" {
		t.Error("prescription_id must not be empty")
	}
	if prescOutput.RiskLevel == "" {
		t.Error("risk_level must not be empty")
	}

	// Report
	reportOutput := svc.Report(ReportInput{
		PrescriptionID: prescOutput.PrescriptionID,
		ExitCode:       0,
		ArtifactDigest: prescOutput.ArtifactDigest,
		Actor:          Actor{Type: "ai_agent", ID: "test-agent", Provenance: "mcp"},
	})

	if !reportOutput.OK {
		t.Fatalf("report failed: %v", reportOutput.Error)
	}
	if reportOutput.ReportID == "" {
		t.Error("report_id must not be empty")
	}

	// Read evidence and verify both entries exist
	entries, err := evidence.ReadAllEntriesAtPath(dir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 evidence entries, got %d", len(entries))
	}
	if entries[0].Type != evidence.EntryTypePrescribe {
		t.Errorf("first entry type: got %q, want prescribe", entries[0].Type)
	}
	if entries[1].Type != evidence.EntryTypeReport {
		t.Errorf("second entry type: got %q, want report", entries[1].Type)
	}
}

func TestReport_CrossActorViolation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := &BenchmarkService{evidencePath: dir, traceID: "01TRACE"}

	// Prescribe as agent-1
	prescOutput := svc.Prescribe(PrescribeInput{
		Actor:       Actor{Type: "ai_agent", ID: "agent-1", Provenance: "mcp"},
		Tool:        "kubectl",
		Operation:   "apply",
		RawArtifact: `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n  namespace: default`,
	})

	// Report as agent-2 (cross-actor)
	reportOutput := svc.Report(ReportInput{
		PrescriptionID: prescOutput.PrescriptionID,
		ExitCode:       0,
		Actor:          Actor{Type: "ai_agent", ID: "agent-2", Provenance: "mcp"},
	})

	// Should still succeed (recording, not blocking) but the signal
	// detector will catch this as cross_actor_report
	if !reportOutput.OK {
		t.Fatalf("report should succeed (inspector records, not blocks)")
	}
}
```

**Step 2: Run tests, commit**

```bash
go test ./pkg/mcpserver/ -run TestPrescribeReport -v -count=1
git add pkg/mcpserver/integration_test.go
git commit -m "test: add MCP prescribe/report lifecycle and cross-actor tests"
```

---

### Task 24: Add signal detector tests with evidence fixtures

**Files:**
- Create: `internal/signal/signal_integration_test.go`

**Step 1: Write end-to-end signal detection test**

```go
func TestAllSignals_EndToEnd(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		// Normal prescribe/report pair
		{EventID: "p1", IsPrescription: true, Tool: "kubectl", OperationClass: "mutate",
		 ScopeClass: "staging", ActorID: "agent-1", IntentDigest: "sha256:a",
		 ShapeHash: "sha256:s1", ArtifactDigest: "sha256:art1", ResourceCount: 1, Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ActorID: "agent-1",
		 ArtifactDigest: "sha256:art1", ExitCode: intPtr(0), Timestamp: now.Add(1 * time.Minute)},

		// Unreported prescription (protocol violation)
		{EventID: "p2", IsPrescription: true, Tool: "kubectl", OperationClass: "mutate",
		 ScopeClass: "staging", ActorID: "agent-1", IntentDigest: "sha256:b",
		 ArtifactDigest: "sha256:art2", ResourceCount: 1, Timestamp: now.Add(2 * time.Minute)},

		// Drift: report has different artifact_digest
		{EventID: "p3", IsPrescription: true, Tool: "terraform", OperationClass: "mutate",
		 ScopeClass: "production", ActorID: "agent-1", IntentDigest: "sha256:c",
		 ArtifactDigest: "sha256:art3", ResourceCount: 1, Timestamp: now.Add(3 * time.Minute)},
		{EventID: "r3", IsReport: true, PrescriptionID: "p3", ActorID: "agent-1",
		 ArtifactDigest: "sha256:art3_DIFFERENT", ExitCode: intPtr(0), Timestamp: now.Add(4 * time.Minute)},
	}

	results := AllSignals(entries)

	resultMap := make(map[string]SignalResult)
	for _, r := range results {
		resultMap[r.Name] = r
	}

	if resultMap["protocol_violation"].Count == 0 {
		t.Error("expected protocol_violation for unreported p2")
	}
	if resultMap["artifact_drift"].Count == 0 {
		t.Error("expected artifact_drift for p3/r3 digest mismatch")
	}
}
```

**Step 2: Run, commit**

```bash
go test ./internal/signal/ -run TestAllSignals_EndToEnd -v -count=1
git add internal/signal/
git commit -m "test: add end-to-end signal detection integration test"
```

---

## Final Verification

After all PRs are merged:

### Task 25: Full verification

**Step 1: Build**

```bash
make build
```

**Step 2: All tests pass**

```bash
go test ./... -v -count=1
```

**Step 3: Race detector**

```bash
go test -race ./...
```

**Step 4: Golden corpus**

```bash
go test ./internal/canon/ -run TestGolden -v -count=1
```

**Step 5: Lint**

```bash
make lint
```

**Step 6: Verify success criteria from CHIEF_ARCHITECT_FINAL_REVIEW.md §11**

1. `EvidenceEntry` matches CORE_DATA_MODEL.md — check
2. Zero OPA-era fields — check (PolicyDecision, PolicyRef, BundleRevision deleted)
3. Golden corpus passes with correct intent_digest — check (shape_hash excluded)
4. All five signal detectors match SIGNAL_SPEC.md — check
5. `evidra scorecard` produces real scores — check
6. `evidra explain` shows top signals — check
7. Confidence model caps scores — check
8. SARIF parser accepts Checkov/Trivy output — check
9. Evidence entries include all version fields — check
10. At least 65 tests pass — check

---

## Dependency Graph Summary

```
PR-1 (evidence schema) — Tasks 1-8
  │
  ├──► PR-2 (canon sync) — Tasks 9-11
  │       │
  │       └──► PR-3 (signal alignment) — Tasks 12-15
  │               │
  │               └──► PR-4 (real scorecard) — Tasks 16-17
  │                       │
  │                       ├──► PR-5 (confidence) — Task 18
  │                       └──► PR-6 (entry types + SARIF) — Tasks 19-21
  │
  └──► PR-7 (test porting) — Tasks 22-24 (parallel after PR-1)
```

**Total: 25 tasks, 7 PRs**
