package analyticsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"samebits.com/evidra/internal/analytics"
	"samebits.com/evidra/internal/store"
)

// fakeStore implements EntryFetcher with canned responses.
type fakeStore struct {
	// pages holds the entries to return per call (indexed by call order).
	pages [][]store.StoredEntry
	// totals holds the total count to return per call.
	totals []int
	// calls records all ListEntries invocations for assertion.
	calls []listCall
	// callIdx tracks the current page index.
	callIdx int
}

type listCall struct {
	TenantID string
	Opts     store.ListOptions
}

func (f *fakeStore) ListEntries(_ context.Context, tenantID string, opts store.ListOptions) ([]store.StoredEntry, int, error) {
	f.calls = append(f.calls, listCall{TenantID: tenantID, Opts: opts})
	if f.callIdx >= len(f.pages) {
		return nil, 0, nil
	}
	page := f.pages[f.callIdx]
	total := f.totals[f.callIdx]
	f.callIdx++
	return page, total, nil
}

// makePrescribeEntry builds a StoredEntry whose payload is a valid prescribe evidence entry.
func makePrescribeEntry(id string, ts time.Time) store.StoredEntry {
	payload := fmt.Sprintf(`{
		"entry_id": %q,
		"type": "prescribe",
		"timestamp": %q,
		"actor": {"id": "test-agent", "type": "ai_agent"},
		"trace_id": "trace-1",
		"spec_version": "1.0.0",
		"canonical_version": "1.0.0",
		"payload": {
			"prescription_id": %q,
			"canonical_action": {"tool": "kubectl", "verb": "apply", "resource": "deployment/nginx", "namespace": "default"},
			"effective_risk": "low",
			"ttl_ms": 30000
		}
	}`, id, ts.Format(time.RFC3339Nano), "rx-"+id)
	return store.StoredEntry{
		ID:      id,
		Payload: json.RawMessage(payload),
	}
}

// makeReportEntry builds a StoredEntry whose payload is a valid report evidence entry.
func makeReportEntry(id, prescriptionID string, ts time.Time) store.StoredEntry {
	exitCode := 0
	payload := fmt.Sprintf(`{
		"entry_id": %q,
		"type": "report",
		"timestamp": %q,
		"actor": {"id": "test-agent", "type": "ai_agent"},
		"trace_id": "trace-1",
		"spec_version": "1.0.0",
		"canonical_version": "1.0.0",
		"payload": {
			"report_id": %q,
			"prescription_id": %q,
			"exit_code": %d,
			"verdict": "applied"
		}
	}`, id, ts.Format(time.RFC3339Nano), "rpt-"+id, prescriptionID, exitCode)
	return store.StoredEntry{
		ID:      id,
		Payload: json.RawMessage(payload),
	}
}

func TestComputeScorecard_EmptyEntries(t *testing.T) {
	t.Parallel()

	fs := &fakeStore{
		pages:  [][]store.StoredEntry{nil},
		totals: []int{0},
	}
	svc := NewService(fs)

	result, err := svc.ComputeScorecard(context.Background(), "tenant-1", analytics.Filters{})
	if err != nil {
		t.Fatalf("ComputeScorecard returned error: %v", err)
	}

	resp, ok := result.(ScorecardAPIResponse)
	if !ok {
		t.Fatalf("expected ScorecardAPIResponse, got %T", result)
	}
	if resp.TotalEntries != 0 {
		t.Errorf("expected TotalEntries=0, got %d", resp.TotalEntries)
	}
	if resp.Basis != "insufficient" {
		t.Errorf("expected Basis=insufficient, got %q", resp.Basis)
	}
}

func TestComputeScorecard_WithEntries(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	entries := []store.StoredEntry{
		makePrescribeEntry("p1", now.Add(-2*time.Minute)),
		makeReportEntry("r1", "rx-p1", now.Add(-1*time.Minute)),
	}
	fs := &fakeStore{
		pages:  [][]store.StoredEntry{entries},
		totals: []int{len(entries)},
	}
	svc := NewService(fs)

	result, err := svc.ComputeScorecard(context.Background(), "tenant-1", analytics.Filters{})
	if err != nil {
		t.Fatalf("ComputeScorecard returned error: %v", err)
	}

	resp, ok := result.(ScorecardAPIResponse)
	if !ok {
		t.Fatalf("expected ScorecardAPIResponse, got %T", result)
	}
	// With a valid prescribe+report pair, the engine should count at least one operation.
	// The exact score depends on signal detection internals; we verify the service
	// orchestrated correctly and produced a non-empty response.
	if resp.Band == "" {
		t.Error("expected non-empty Band")
	}
	if len(fs.calls) != 1 {
		t.Errorf("expected 1 store call, got %d", len(fs.calls))
	}
}

func TestComputeScorecard_PaginatesMultiplePages(t *testing.T) {
	t.Parallel()

	// Test collectReplayEntries directly to exercise pagination without
	// needing analyticsReplayPageSize (1000) entries. We use pageSize=2
	// so that two entries per page triggers a second fetch.
	now := time.Now().UTC()
	page1 := []store.StoredEntry{
		makePrescribeEntry("p1", now.Add(-3*time.Minute)),
		makeReportEntry("r1", "rx-p1", now.Add(-2*time.Minute)),
	}
	page2 := []store.StoredEntry{
		makePrescribeEntry("p2", now.Add(-1*time.Minute)),
		makeReportEntry("r2", "rx-p2", now),
	}

	fs := &fakeStore{
		pages:  [][]store.StoredEntry{page1, page2},
		totals: []int{4, 4}, // total=4 across both pages
	}

	entries, err := collectReplayEntries(
		context.Background(),
		"tenant-1",
		store.ListOptions{},
		2, // pageSize=2 forces pagination
		fs.ListEntries,
	)
	if err != nil {
		t.Fatalf("collectReplayEntries returned error: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 total entries, got %d", len(entries))
	}
	if len(fs.calls) != 2 {
		t.Fatalf("expected 2 paginated store calls, got %d", len(fs.calls))
	}
	// First call should have offset 0.
	if fs.calls[0].Opts.Offset != 0 {
		t.Errorf("first call offset: expected 0, got %d", fs.calls[0].Opts.Offset)
	}
	// Second call should have offset = len(page1).
	if fs.calls[1].Opts.Offset != len(page1) {
		t.Errorf("second call offset: expected %d, got %d", len(page1), fs.calls[1].Opts.Offset)
	}
}

func TestComputeExplain_EmptyEntries(t *testing.T) {
	t.Parallel()

	fs := &fakeStore{
		pages:  [][]store.StoredEntry{nil},
		totals: []int{0},
	}
	svc := NewService(fs)

	result, err := svc.ComputeExplain(context.Background(), "tenant-1", analytics.Filters{})
	if err != nil {
		t.Fatalf("ComputeExplain returned error: %v", err)
	}

	resp, ok := result.(analytics.ExplainOutput)
	if !ok {
		t.Fatalf("expected ExplainOutput, got %T", result)
	}
	if resp.TotalOps != 0 {
		t.Errorf("expected TotalOps=0, got %d", resp.TotalOps)
	}
}

func TestComputeExplain_WithEntries(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	entries := []store.StoredEntry{
		makePrescribeEntry("p1", now.Add(-2*time.Minute)),
		makeReportEntry("r1", "rx-p1", now.Add(-1*time.Minute)),
	}
	fs := &fakeStore{
		pages:  [][]store.StoredEntry{entries},
		totals: []int{len(entries)},
	}
	svc := NewService(fs)

	result, err := svc.ComputeExplain(context.Background(), "tenant-1", analytics.Filters{})
	if err != nil {
		t.Fatalf("ComputeExplain returned error: %v", err)
	}

	resp, ok := result.(analytics.ExplainOutput)
	if !ok {
		t.Fatalf("expected ExplainOutput, got %T", result)
	}
	// With a valid prescribe+report pair, TotalOps should be at least 1.
	if resp.TotalOps < 1 {
		t.Errorf("expected TotalOps >= 1, got %d", resp.TotalOps)
	}
	if resp.Band == "" {
		t.Error("expected non-empty Band")
	}
	if len(resp.Signals) == 0 {
		t.Error("expected non-empty Signals slice")
	}
}

func TestComputeExplain_PassesFilters(t *testing.T) {
	t.Parallel()

	fs := &fakeStore{
		pages:  [][]store.StoredEntry{nil},
		totals: []int{0},
	}
	svc := NewService(fs)

	filters := analytics.Filters{
		Period:    "7d",
		SessionID: "sess-42",
	}
	_, err := svc.ComputeExplain(context.Background(), "tenant-abc", filters)
	if err != nil {
		t.Fatalf("ComputeExplain returned error: %v", err)
	}

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(fs.calls))
	}
	call := fs.calls[0]
	if call.TenantID != "tenant-abc" {
		t.Errorf("expected tenantID=tenant-abc, got %q", call.TenantID)
	}
	if call.Opts.Period != "7d" {
		t.Errorf("expected Period=7d, got %q", call.Opts.Period)
	}
	if call.Opts.SessionID != "sess-42" {
		t.Errorf("expected SessionID=sess-42, got %q", call.Opts.SessionID)
	}
}
