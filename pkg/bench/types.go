// Package bench provides structured result storage and queries for
// infrastructure agent benchmark runs (PostgreSQL / pgx).
package bench

import (
	"time"
)

// LeaderboardEntry represents one model's aggregate benchmark performance.
type LeaderboardEntry struct {
	Model       string  `json:"model"`
	Scenarios   int     `json:"scenarios"`
	Runs        int     `json:"runs"`
	PassRate    float64 `json:"pass_rate"`
	AvgDuration float64 `json:"avg_duration"`
	AvgCost     float64 `json:"avg_cost"`
	TotalCost   float64 `json:"total_cost"`
}

// RunRecord represents a single benchmark run stored in bench_runs.
type RunRecord struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	ScenarioID       string    `json:"scenario_id"`
	Model            string    `json:"model"`
	Provider         string    `json:"provider"`
	Adapter          string    `json:"adapter"`
	EvidenceMode     string    `json:"evidence_mode"` // direct, proxy, smart, or none
	Passed           bool      `json:"passed"`
	Duration         float64   `json:"duration_seconds"`
	ExitCode         int       `json:"exit_code"`
	Turns            int       `json:"turns"`
	MemoryWindow     int       `json:"memory_window"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	EstimatedCost    float64   `json:"estimated_cost_usd"`
	ChecksPassed     int       `json:"checks_passed"`
	ChecksTotal      int       `json:"checks_total"`
	ChecksJSON       string    `json:"checks_json,omitempty"`
	MetadataJSON     string    `json:"metadata_json,omitempty"`
	ArtifactDir      string    `json:"artifact_dir,omitempty"` // local filesystem path (bench runner only)
	CreatedAt        time.Time `json:"created_at"`
}

// RunFilters specifies filters for listing runs.
type RunFilters struct {
	ScenarioID   string
	Model        string
	Provider     string
	EvidenceMode string // proxy, direct, smart, none -- empty means all
	PassedOnly   bool
	FailedOnly   bool
	Since        *time.Time // cutoff time — handler parses, store just uses
	Limit        int
	Offset       int
	SortBy       string // column to sort by
	SortOrder    string // asc or desc (default: desc)
}

// RunCatalog holds distinct metadata values used for UI filters.
type RunCatalog struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
}

// StatsResult holds aggregate run statistics.
type StatsResult struct {
	TotalRuns  int            `json:"total_runs"`
	PassCount  int            `json:"pass_count"`
	FailCount  int            `json:"fail_count"`
	ByScenario []ScenarioStat `json:"by_scenario"`
}

// ScenarioStat holds per-scenario stats.
type ScenarioStat struct {
	ScenarioID string `json:"scenario_id"`
	Runs       int    `json:"runs"`
	Passed     int    `json:"passed"`
}

// ScenarioSummary holds metadata about a scenario for listing.
type ScenarioSummary struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Chaos    bool     `json:"chaos"`
	Evidra   bool     `json:"evidra"`
}
