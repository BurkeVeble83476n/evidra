package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/pipeline"
	"samebits.com/evidra-benchmark/internal/risk"
	"samebits.com/evidra-benchmark/internal/score"
	"samebits.com/evidra-benchmark/internal/signal"
	"samebits.com/evidra-benchmark/pkg/evidence"
	"samebits.com/evidra-benchmark/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "evidra-benchmark %s (commit: %s, built: %s)\n",
			version.Version, version.Commit, version.Date)
		return 0
	case "scorecard":
		return cmdScorecard(args[1:], stdout, stderr)
	case "explain":
		return cmdExplain(args[1:], stdout, stderr)
	case "compare":
		return cmdCompare(args[1:], stdout, stderr)
	case "prescribe":
		return cmdPrescribe(args[1:], stdout, stderr)
	case "report":
		return cmdReport(args[1:], stdout, stderr)
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func cmdScorecard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scorecard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	actorFlag := fs.String("actor", "", "Actor ID to generate scorecard for")
	periodFlag := fs.String("period", "30d", "Time period (e.g. 30d)")
	evidenceFlag := fs.String("evidence-dir", "", "Evidence directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	evidencePath := resolveEvidencePath(*evidenceFlag)

	entries, err := evidence.ReadAllEntriesAtPath(evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading evidence: %v\n", err)
		return 1
	}

	filtered := filterEntries(entries, *actorFlag, *periodFlag)

	signalEntries, err := pipeline.EvidenceToSignalEntries(filtered)
	if err != nil {
		fmt.Fprintf(stderr, "Error converting evidence: %v\n", err)
		return 1
	}

	totalOps := countPrescriptions(signalEntries)
	results := signal.AllSignals(signalEntries)
	sc := score.Compute(results, totalOps)

	output := struct {
		score.Scorecard
		ActorID        string `json:"actor_id,omitempty"`
		Period         string `json:"period"`
		ScoringVersion string `json:"scoring_version"`
		SpecVersion    string `json:"spec_version"`
		EvidraVersion  string `json:"evidra_version"`
		GeneratedAt    string `json:"generated_at"`
	}{
		Scorecard:      sc,
		ActorID:        *actorFlag,
		Period:         *periodFlag,
		ScoringVersion: "0.3.0",
		SpecVersion:    "0.3.0",
		EvidraVersion:  version.Version,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(stderr, "encode scorecard: %v\n", err)
		return 1
	}
	return 0
}

func cmdExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	actorFlag := fs.String("actor", "", "Actor ID to explain")
	periodFlag := fs.String("period", "30d", "Time period (e.g. 30d)")
	evidenceFlag := fs.String("evidence-dir", "", "Evidence directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	evidencePath := resolveEvidencePath(*evidenceFlag)

	entries, err := evidence.ReadAllEntriesAtPath(evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading evidence: %v\n", err)
		return 1
	}

	filtered := filterEntries(entries, *actorFlag, *periodFlag)

	signalEntries, err := pipeline.EvidenceToSignalEntries(filtered)
	if err != nil {
		fmt.Fprintf(stderr, "Error converting evidence: %v\n", err)
		return 1
	}

	totalOps := countPrescriptions(signalEntries)
	results := signal.AllSignals(signalEntries)
	sc := score.Compute(results, totalOps)

	type SignalDetail struct {
		Signal   string   `json:"signal"`
		Count    int      `json:"count"`
		Weight   float64  `json:"weight"`
		Rate     float64  `json:"rate"`
		EntryIDs []string `json:"entry_ids,omitempty"`
	}

	var details []SignalDetail
	for _, r := range results {
		rate := 0.0
		if totalOps > 0 {
			rate = float64(r.Count) / float64(totalOps)
		}
		weight := score.DefaultWeights[r.Name]
		details = append(details, SignalDetail{
			Signal:   r.Name,
			Count:    r.Count,
			Weight:   weight,
			Rate:     rate,
			EntryIDs: r.EventIDs,
		})
	}

	output := struct {
		Score         float64        `json:"score"`
		Band          string         `json:"band"`
		TotalOps      int            `json:"total_operations"`
		Signals       []SignalDetail `json:"signals"`
		EvidraVersion string         `json:"evidra_version"`
		GeneratedAt   string         `json:"generated_at"`
	}{
		Score:         sc.Score,
		Band:          sc.Band,
		TotalOps:      totalOps,
		Signals:       details,
		EvidraVersion: version.Version,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(stderr, "encode explain: %v\n", err)
		return 1
	}
	return 0
}

func cmdCompare(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	actorsFlag := fs.String("actors", "", "Comma-separated actor IDs to compare")
	toolFlag := fs.String("tool", "", "Filter by tool")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	actors := strings.Split(*actorsFlag, ",")
	if len(actors) < 2 {
		fmt.Fprintln(stderr, "compare requires at least 2 actors (--actors A,B)")
		return 2
	}

	_ = *toolFlag

	// Demo: compute overlap between empty profiles
	a := score.WorkloadProfile{Tools: map[string]bool{}, Scopes: map[string]bool{}}
	b := score.WorkloadProfile{Tools: map[string]bool{}, Scopes: map[string]bool{}}
	overlap := score.WorkloadOverlap(a, b)

	result := map[string]interface{}{
		"actors":  actors,
		"overlap": overlap,
		"note":    "load evidence chain for real comparison",
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode comparison: %v\n", err)
		return 1
	}
	return 0
}

func cmdPrescribe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prescribe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	artifactFlag := fs.String("artifact", "", "Path to artifact file (YAML or JSON)")
	toolFlag := fs.String("tool", "", "Tool name (kubectl, terraform)")
	operationFlag := fs.String("operation", "apply", "Operation (apply, delete, plan)")
	envFlag := fs.String("environment", "", "Environment (production, staging, development)")
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

	result := map[string]interface{}{
		"artifact_digest":     cr.ArtifactDigest,
		"intent_digest":       cr.IntentDigest,
		"resource_shape_hash": cr.CanonicalAction.ResourceShapeHash,
		"canon_version":       cr.CanonVersion,
		"resource_count":      cr.CanonicalAction.ResourceCount,
		"operation_class":     cr.CanonicalAction.OperationClass,
		"scope_class":         cr.CanonicalAction.ScopeClass,
		"risk_level":          riskLevel,
		"risk_tags":           riskTags,
	}
	if cr.ParseError != nil {
		result["parse_error"] = cr.ParseError.Error()
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode prescription: %v\n", err)
		return 1
	}
	return 0
}

func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	prescriptionFlag := fs.String("prescription", "", "Prescription event ID")
	exitCodeFlag := fs.Int("exit-code", 0, "Exit code of the operation")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *prescriptionFlag == "" {
		fmt.Fprintln(stderr, "report requires --prescription")
		return 2
	}

	status := "completed"
	if *exitCodeFlag != 0 {
		status = "failed"
	}

	result := map[string]interface{}{
		"prescription_id": *prescriptionFlag,
		"exit_code":       *exitCodeFlag,
		"status":          status,
		"note":            "evidence chain recording requires --evidence-dir",
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "encode report: %v\n", err)
		return 1
	}
	return 0
}

func resolveEvidencePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".evidra", "evidence")
	}
	return filepath.Join(home, ".evidra", "evidence")
}

func filterEntries(entries []evidence.EvidenceEntry, actor, period string) []evidence.EvidenceEntry {
	cutoff := parsePeriodCutoff(period)
	var filtered []evidence.EvidenceEntry
	for _, e := range entries {
		if actor != "" && e.Actor.ID != actor {
			continue
		}
		if !cutoff.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func parsePeriodCutoff(period string) time.Time {
	if period == "" {
		return time.Time{}
	}
	now := time.Now().UTC()
	if len(period) < 2 {
		return time.Time{}
	}
	unit := period[len(period)-1]
	val := 0
	fmt.Sscanf(period[:len(period)-1], "%d", &val)
	if val <= 0 {
		return time.Time{}
	}
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -val)
	case 'h':
		return now.Add(-time.Duration(val) * time.Hour)
	default:
		return time.Time{}
	}
}

func countPrescriptions(entries []signal.Entry) int {
	count := 0
	for _, e := range entries {
		if e.IsPrescription {
			count++
		}
	}
	return count
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "evidra-benchmark — flight recorder for infrastructure automation")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "COMMANDS:")
	fmt.Fprintln(w, "  scorecard   Generate reliability scorecard for an actor")
	fmt.Fprintln(w, "  explain     Explain signals contributing to a score")
	fmt.Fprintln(w, "  compare     Compare reliability scores between actors")
	fmt.Fprintln(w, "  prescribe   Analyze artifact before execution")
	fmt.Fprintln(w, "  report      Record outcome after execution")
	fmt.Fprintln(w, "  version     Print version information")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'evidra <command> --help' for command-specific flags.")
}
