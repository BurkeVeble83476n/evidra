package analyticsvc

import (
	"context"
	"fmt"

	"samebits.com/evidra/internal/analytics"
	"samebits.com/evidra/internal/analyticsdb"
	"samebits.com/evidra/internal/score"
	"samebits.com/evidra/internal/store"
)

// EntryFetcher provides paginated access to stored evidence entries.
type EntryFetcher interface {
	ListEntries(ctx context.Context, tenantID string, opts store.ListOptions) ([]store.StoredEntry, int, error)
}

// Service owns analytics computation over stored evidence entries.
// It reads raw entries via EntryFetcher and delegates to the shared
// analytics engine for scorecard and explain computation.
type Service struct {
	store EntryFetcher
}

// NewService creates an analytics service backed by the given store.
func NewService(store EntryFetcher) *Service {
	return &Service{store: store}
}

const analyticsReplayPageSize = 1000

// ScorecardAPIResponse is the documented API shape for scorecard responses.
type ScorecardAPIResponse struct {
	Score          float64                       `json:"score"`
	Band           string                        `json:"band"`
	Basis          string                        `json:"basis"`
	Confidence     string                        `json:"confidence"`
	TotalEntries   int                           `json:"total_entries"`
	SignalSummary  map[string]SignalSummaryEntry `json:"signal_summary"`
	Period         string                        `json:"period,omitempty"`
	ScoringVersion string                        `json:"scoring_version,omitempty"`
	GeneratedAt    string                        `json:"generated_at,omitempty"`
}

// SignalSummaryEntry describes a single signal in the scorecard API response.
type SignalSummaryEntry struct {
	Detected bool    `json:"detected"`
	Weight   float64 `json:"weight"`
	Count    int     `json:"count"`
}

// ComputeScorecard reads stored entries and produces a response-ready scorecard.
func (s *Service) ComputeScorecard(ctx context.Context, tenantID string, filters analytics.Filters) (interface{}, error) {
	entries, err := collectReplayEntries(ctx, tenantID, store.ListOptions{
		Period:    filters.Period,
		SessionID: filters.SessionID,
	}, analyticsReplayPageSize, s.store.ListEntries)
	if err != nil {
		return nil, fmt.Errorf("analyticsvc.ComputeScorecard: %w", err)
	}

	rows := storedToRows(entries)
	out, err := analyticsdb.ComputeScorecardFromStoredRows(rows, filters)
	if err != nil {
		return nil, fmt.Errorf("analyticsvc.ComputeScorecard: %w", err)
	}

	profile, err := score.ResolveProfile("")
	if err != nil {
		// Fall back to raw output if profile resolution fails.
		return out, nil
	}
	return toScorecardAPIResponse(out, profile), nil
}

// ComputeExplain reads stored entries and runs signal detection.
func (s *Service) ComputeExplain(ctx context.Context, tenantID string, filters analytics.Filters) (interface{}, error) {
	entries, err := collectReplayEntries(ctx, tenantID, store.ListOptions{
		Period:    filters.Period,
		SessionID: filters.SessionID,
	}, analyticsReplayPageSize, s.store.ListEntries)
	if err != nil {
		return nil, fmt.Errorf("analyticsvc.ComputeExplain: %w", err)
	}

	rows := storedToRows(entries)
	return analyticsdb.ComputeExplainFromStoredRows(rows, filters)
}

func toScorecardAPIResponse(out analytics.ScorecardOutput, profile score.Profile) ScorecardAPIResponse {
	basis := "insufficient"
	if out.Sufficient {
		basis = "sufficient"
	}

	summary := make(map[string]SignalSummaryEntry)
	for _, name := range analytics.PublicSignalNames(profile) {
		count := out.Signals[name]
		summary[name] = SignalSummaryEntry{
			Detected: count > 0,
			Weight:   profile.Weight(name),
			Count:    count,
		}
	}

	return ScorecardAPIResponse{
		Score:          out.Score,
		Band:           out.Band,
		Basis:          basis,
		Confidence:     out.Confidence.Level,
		TotalEntries:   out.TotalOperations,
		SignalSummary:  summary,
		Period:         out.Period,
		ScoringVersion: out.ScoringVersion,
		GeneratedAt:    out.GeneratedAt,
	}
}

func storedToRows(entries []store.StoredEntry) []analyticsdb.StoredRow {
	rows := make([]analyticsdb.StoredRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, analyticsdb.StoredRow{
			ID:      entry.ID,
			Payload: entry.Payload,
		})
	}
	return rows
}

func collectReplayEntries(
	ctx context.Context,
	tenantID string,
	baseOpts store.ListOptions,
	pageSize int,
	list func(context.Context, string, store.ListOptions) ([]store.StoredEntry, int, error),
) ([]store.StoredEntry, error) {
	if pageSize <= 0 {
		pageSize = analyticsReplayPageSize
	}

	opts := baseOpts
	opts.Limit = pageSize
	opts.Offset = 0

	all := make([]store.StoredEntry, 0, pageSize)
	for {
		page, total, err := list(ctx, tenantID, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(all) >= total || len(page) < opts.Limit {
			return all, nil
		}
		opts.Offset += len(page)
	}
}
