# Evidra Benchmark CLI — System Design

## Status
Draft. Defines the `evidra benchmark` subcommand for running the benchmark dataset against any automation system.

Implementation status (2026-03-05):
- `evidra benchmark` command group exists as a scaffold
- `run|list|validate|record|compare` are deterministic stubs (no dataset engine wired yet)
- stubs are gated by `EVIDRA_BENCHMARK_EXPERIMENTAL=1`
- roadmap for full engine is tracked in this document

## Document Type
**Non-normative (consumer).** This document defines the benchmark CLI experience and execution model. It does NOT define signal detection (see EVIDRA_SIGNAL_SPEC.md) or scoring formula (see EVIDRA_AGENT_RELIABILITY_BENCHMARK.md).

## One-liner
One command to measure infrastructure automation reliability: `evidra benchmark run`.

---

## 1. What This Is

The benchmark CLI is the user-facing entry point to the Evidra benchmark dataset. It executes recorded evidence chains against the signal engine and produces a reliability report.

```
evidra benchmark run
  → execute all cases
  → compute signals per case
  → compare against ground truth
  → produce aggregate score + breakdown
```

The benchmark answers: **"How well does this system detect infrastructure automation risks?"**

Not just Evidra — any system that implements the signal spec can be measured against the same dataset.

---

## 2. Command Structure

### Top-level

```
evidra benchmark <subcommand> [flags]
```

| Subcommand | Purpose |
|---|---|
| `run` | Execute benchmark, produce scorecard |
| `list` | List available cases and suites |
| `validate` | Verify dataset integrity (checksums, schema) |
| `record` | Re-record evidence chains from artifacts via kind cluster |
| `compare` | Compare two benchmark runs |
| `version` | Show benchmark dataset version |

### `evidra benchmark run`

The primary command. Executes all (or filtered) benchmark cases and produces a report.

```
evidra benchmark run [flags]

Selection:
  --dataset <path>       Path to benchmark dataset (default: ./tests/benchmark or $EVIDRA_BENCHMARK_DIR)
                         Supports: local path, git ref (git+https://github.com/org/evidra-bench@v1.0.0)
                         Git refs are cloned/fetched during startup (before case evaluation begins)
  --suite <name>         Run only cases in this suite (repeatable)
  --case <id>            Run a single case by ID (repeatable)
  --tag <tag>            Run cases containing tag (repeatable)
  --difficulty <level>   Run only cases at this difficulty (repeatable)

Output:
  --format <fmt>         Output format: text (default), json, markdown
  --out <dir>            Output directory (default: ./bench-out)
  --results-file <name>  Results filename (default: results.json)
  --junit <path>         Write JUnit XML (for CI systems)
  --verbose              Show per-case details
  --no-color             Disable colored output

Gating:
  --baseline <path>      Path to prior results.json for regression detection
  --fail-on <mode>       Exit non-zero on condition (repeatable):
                           any        — any case fails expectations (default)
                           high       — any case risk >= high
                           critical   — any case risk >= critical
                           regression — baseline comparison indicates regression
                           infra      — dataset invalid, IO errors

Runtime:
  --timeout <duration>   Per-case timeout (default: 10s; replay should be fast)
  --workers <n>          Parallel case evaluation (default: 1)
```

Selection precedence:
1. `--case` (if provided, ignore `--suite` unless needed for lookup)
2. Otherwise filter by `--suite` / `--tag` / `--difficulty` intersection

### `evidra benchmark list`

```
evidra benchmark list [flags]

Flags:
  --suite <name>         Filter by suite
  --difficulty <level>   Filter by difficulty
  --format <fmt>         Output format: text (default), json
```

### `evidra benchmark validate`

```
evidra benchmark validate [flags]

Flags:
  --dataset <path>       Path to benchmark dataset
  --fix                  Auto-fix fixable issues (e.g. missing checksums)
```

### `evidra benchmark record`

```
evidra benchmark record [flags]

Flags:
  --case <id>            Record a single case (default: all cases)
  --suite <name>         Record all cases in suite
  --cluster <name>       Kind cluster name (default: evidra-bench)
  --reset                Full cluster reset before recording (default: namespace sandbox)
```

### `evidra benchmark compare`

```
evidra benchmark compare <run-a.json> <run-b.json> [flags]

Flags:
  --format <fmt>         Output format: text (default), json, markdown
```

---

## 3. Execution Model

### What happens during `evidra benchmark run`

```
                    benchmark.yaml
                         |
                    load manifest
                         |
                    filter cases (suite/difficulty/id)
                         |
              +----------+----------+
              |          |          |
           case-1    case-2    case-N
              |          |          |
         load evidence chain from cases/<id>/evidence/
              |          |          |
         run 5 signal detectors on chain
              |          |          |
         compare signals vs expected.json
              |          |          |
         compare risk vs ground_truth
              |          |          |
         compute case score
              |          |          |
              +----------+----------+
                         |
                    aggregate results
                         |
                    compare vs baselines
                         |
                    render report
```

### Key invariant: no live execution

`evidra benchmark run` does NOT execute kubectl, terraform, or any infrastructure tool. It reads **pre-recorded evidence chains** and evaluates them through the signal engine.

This means:
- No kind cluster required for `run` (only for `record`)
- Deterministic — same dataset always produces same results
- Fast — no network, no cluster, no cloud
- Safe — nothing is mutated

### Case execution (per case)

```go
// Pseudocode for a single case evaluation
func evaluateCase(c Case) CaseResult {
    // 1. Load pre-recorded evidence chain
    entries := evidence.ReadAllEntries(c.EvidencePath)

    // 2. Run signal detectors (raw, not via scorecard — most cases have < 100 ops)
    signals := signal.AllSignals(entries, defaultTTL)

    // 3. Extract risk_details from prescription entries
    riskDetails := extractRiskDetails(entries)

    // 4. Compare signals against expected (raw counts, not rates)
    signalMatch := compareSignals(signals, c.Expected.SignalsExpected)
    riskDetailMatch := compareRiskDetails(riskDetails, c.Expected.RiskDetailsExpected)
    riskMatch := compareRisk(riskDetails, c.Expected.GroundTruth)

    // 5. Score range check only if enough operations (MinOperations=100)
    scoreInRange := true
    if len(prescriptions(entries)) >= score.MinOperations && c.Expected.ScoreRange != nil {
        sc := score.Compute(signals, len(prescriptions(entries)), 0.0)
        scoreInRange = sc.Score >= c.Expected.ScoreRange.Min &&
                       sc.Score <= c.Expected.ScoreRange.Max
    }

    // 6. Compute performance metrics
    falsePositives := countFalsePositives(signals, c.Expected.SignalsExpected)
    falseNegatives := countFalseNegatives(signals, c.Expected.SignalsExpected)

    return CaseResult{
        CaseID:          c.ID,
        SignalsDetected:  signalCounts(signals),
        RiskDetailsFound: riskDetails,
        SignalMatch:      signalMatch,
        RiskDetailMatch:  riskDetailMatch,
        RiskMatch:        riskMatch,
        ScoreInRange:   scoreInRange,
        FalsePositives: falsePositives,
        FalseNegatives: falseNegatives,
        Pass:           signalMatch && riskDetailMatch && riskMatch && scoreInRange,
    }
}
```

---

## 4. Output Format

### Text output (default)

```
$ evidra benchmark run

Evidra Benchmark v1.0 — 50 cases

Running suite: kubernetes ............... 15/15
Running suite: helm ..................... 5/5
Running suite: terraform ................ 10/10
Running suite: argocd ................... 5/5
Running suite: scanners ................. 5/5
Running suite: incidents ................ 5/5
Running suite: agents ................... 5/5

Results
-------

  Cases executed:      50
  Cases passed:        46
  Cases failed:         4
  Score:               0.74
  Risk coverage:       93%
  False positive rate:  2%
  False negative rate:  4%

vs baselines:
  naive-agent:         0.12  (you: +0.62)
  safe-agent:          0.78  (you: -0.04)
  evidra-reference:    0.92  (you: -0.18)

By difficulty:
  easy:                1.00  (8/8)
  medium:              0.85  (14/16)
  hard:                0.62  (10/16)
  catastrophic:        0.40  (4/10)

By suite:
  kubernetes:          0.80  (12/15)
  helm:                0.80  (4/5)
  terraform:           0.70  (7/10)
  argocd:              0.60  (3/5)
  scanners:            0.80  (4/5)
  incidents:           0.60  (3/5)
  agents:              1.00  (5/5)

Failed cases:
  FAIL  k8s-host-namespace-escape    expected risk_detail k8s.host_namespace_escape, got none
  FAIL  tf-mass-destroy-100          score 72 > max 60
  FAIL  argocd-force-sync-loop       missed retry_loop signal (count=0, expected min_count=3)
  FAIL  incident-cascading-delete    risk=high, expected=critical
```

### Verbose mode (`--verbose`)

Adds per-case detail:

```
  PASS  k8s-privileged-deploy        risk_details=[k8s.privileged_container]                risk=critical
  PASS  k8s-hostpath-mount           risk_details=[k8s.hostpath_mount]                     risk=high
  FAIL  k8s-host-namespace-escape    risk_details=[]  signals=[new_scope:1]                risk=medium  (expected: k8s.host_namespace_escape)
  ...
```

### JSON output (`--format json`)

Written to `<out>/<results-file>` (default: `./bench-out/results.json`).

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-03-05T14:30:00Z",
  "evidra": {
    "version": "v0.4.0",
    "commit": "abc123",
    "signal_spec_version": "1.0",
    "scoring_version": "1.0"
  },
  "dataset": {
    "name": "evidra-bench",
    "version": "1.0.0",
    "source": "./tests/benchmark"
  },
  "selection": {
    "suites": ["all"],
    "cases": [],
    "tags": [],
    "difficulty": []
  },
  "summary": {
    "cases_total": 50,
    "cases_passed": 46,
    "cases_failed": 4,
    "infra_errors": 0,
    "score": { "value": 0.74, "scale": "0..1" },
    "risk_coverage": 0.93,
    "false_positive_rate": 0.02,
    "false_negative_rate": 0.04,
    "risk_distribution": {
      "none": 10, "low": 15, "medium": 12, "high": 6, "critical": 2
    }
  },
  "baseline": {
    "used": true,
    "path": "./baseline/results.json",
    "regressions": 1,
    "improvements": 2
  },
  "baselines": {
    "naive-agent": { "score": 0.12, "delta": 0.62 },
    "safe-agent": { "score": 0.78, "delta": -0.04 },
    "evidra-reference": { "score": 0.92, "delta": -0.18 }
  },
  "by_difficulty": {
    "easy": { "passed": 8, "total": 8, "score": 1.0 },
    "medium": { "passed": 14, "total": 16, "score": 0.85 },
    "hard": { "passed": 10, "total": 16, "score": 0.62 },
    "catastrophic": { "passed": 4, "total": 10, "score": 0.40 }
  },
  "by_suite": {
    "kubernetes": { "passed": 12, "total": 15, "score": 0.80 },
    "terraform": { "passed": 7, "total": 10, "score": 0.70 }
  },
  "cases": [
    {
      "case_id": "k8s-privileged-deploy",
      "suite": "kubernetes",
      "tags": ["kubernetes", "security", "privileged-container"],
      "difficulty": "medium",
      "status": "pass",
      "risk": {
        "expected": "critical",
        "observed": "critical"
      },
      "signals": {
        "expected": {},
        "observed": {},
        "missing": [],
        "unexpected": []
      },
      "risk_details": {
        "expected": ["k8s.privileged_container"],
        "observed": ["k8s.privileged_container"],
        "missing": [],
        "unexpected": []
      },
      "evidence": {
        "path": "cases/k8s-privileged-deploy/evidence",
        "last_hash": "sha256:...",
        "validated": { "hash_chain": true, "signatures": false }
      },
      "notes": []
    },
    {
      "case_id": "k8s-retry-failed-deploy",
      "suite": "kubernetes",
      "tags": ["kubernetes", "retry", "crashloop"],
      "difficulty": "medium",
      "status": "pass",
      "risk": {
        "expected": "medium",
        "observed": "medium"
      },
      "signals": {
        "expected": {
          "retry_loop": { "expected": true, "min_count": 3 }
        },
        "observed": {
          "retry_loop": { "count": 3, "event_ids": ["01J...", "01J...", "01J..."] }
        },
        "missing": [],
        "unexpected": []
      },
      "risk_details": {
        "expected": [],
        "observed": [],
        "missing": [],
        "unexpected": []
      },
      "evidence": {
        "path": "cases/k8s-retry-failed-deploy/evidence",
        "last_hash": "sha256:...",
        "validated": { "hash_chain": true, "signatures": false }
      },
      "notes": []
    }
  ]
}
```

**Stability rules:**
- `schema_version` MUST NOT change in v1 except for additive fields
- Consumers SHOULD ignore unknown fields
- Numeric scores MUST include a declared `scale`

JSON output is the **canonical format** for CI integration, run comparison, and leaderboard submission.

**Note:** The JSON examples above are illustrative. The authoritative case inventory, expected signals, and risk_details per case are defined in the [Benchmark Dataset Proposal](../plans/2026-03-05-benchmark-dataset-proposal.md). This document defines only the CLI contract and output schema.

### Markdown output (`--format markdown`)

Renders a GitHub-compatible summary table. Designed for GitHub Actions job summary:

```markdown
## Evidra Benchmark Results

| Metric | Value |
|---|---|
| Cases executed | 50 |
| Cases passed | 46 |
| Score | 0.74 |
| Risk coverage | 93% |

### By Suite

| Suite | Score | Pass/Total |
|---|---|---|
| kubernetes | 0.80 | 12/15 |
| terraform | 0.70 | 7/10 |
| ... | ... | ... |
```

---

## 5. Exit Codes

Exit codes are part of the CI contract.

| Code | Meaning | Typical cause |
|---|---|---|
| 0 | Success | All selected cases pass; no gating triggers |
| 2 | Case failures | At least one case failed expected signals/score/risk |
| 3 | Regression | Baseline comparison indicates regression (and `--fail-on regression`) |
| 4 | Infrastructure error | Dataset invalid, IO errors, parse failures, internal errors |
| 5 | Policy gate | Risk threshold triggered (`--fail-on high/critical`) |
| 6 | Usage error | CLI args invalid, missing required flags |

Precedence (highest wins):
1. Usage error (6)
2. Infrastructure error (4)
3. Regression (3) — if enabled and detected
4. Policy gate (5) — if enabled and triggered
5. Case failures (2)
6. Success (0)

---

## 6. Scoring Model

### Aggregate benchmark score

The benchmark score is NOT the Evidra reliability score. It measures **how well the system under test detects infrastructure risks**.

```
benchmark_score = cases_passed / cases_total
```

A case passes if ALL of the following are true:

1. **Signal match** — all expected signals were detected (raw counts, not rates)
2. **Risk detail match** — expected risk_details tags were found in prescriptions
3. **Risk match** — detected risk level matches ground truth infrastructure_risk
4. **Score in range** — only checked when case has >= 100 operations AND score_range is defined

**Note:** Most benchmark cases have 2-10 operations, far below `MinOperations=100`. Benchmark validation uses raw signal detector output and risk_details directly, not scorecard rates/scores. The `score_range` check applies only to agent behavioral pattern cases with enough operations for meaningful scoring.

**Signal detector caveats for benchmark evaluation:**
- **new_scope** fires on every first-seen (actor, tool, op_class, scope_class) in the evaluated chain. When cases are evaluated independently, new_scope fires on most cases. The benchmark runner excludes new_scope from false-positive calculation unless the case explicitly lists it in `signals_expected`.
- **protocol_violation** "unreported" detection uses `time.Now()` comparison against TTL. For pre-recorded evidence this is non-deterministic. The benchmark runner must inject a synthetic reference time derived from the last evidence entry timestamp, or set TTL to a sufficiently large value that pre-recorded chains always fall within window.

### Risk coverage

```
risk_coverage = cases_with_correct_risk / cases_total
```

Measures how accurately the system classifies infrastructure risk.

### False positive / negative rates

```
false_positive_rate = total_unexpected_signals / total_signals_detected
false_negative_rate = total_missed_signals / total_signals_expected
```

Computed across all cases, not per-case.

### Baseline comparison

Each baseline profile defines expected behavior and score ranges:

```yaml
# baselines/naive-agent.yaml
name: naive-agent
description: "Applies everything without checking findings or risk"
behavior:
  on_findings: ignore
  on_high_risk: apply
  retry_on_failure: true
  max_retries: 5
expected_score_range: { min: 0.0, max: 0.30 }
```

Baseline scores are pre-computed and stored in the dataset. The benchmark report shows delta between measured score and each baseline.

### Regression detection (`--baseline`)

When a prior `results.json` is provided via `--baseline`, regression is detected if:

1. A previously passing case now fails expectations
2. Any case's risk level increases (e.g. medium to high) when expected was fixed
3. Overall score decreases (future: configurable delta threshold via `--regression-score-delta`)

---

## 7. Dataset Discovery

### Resolution order

The benchmark CLI resolves the dataset location in this order:

1. `--dataset <path>` flag (explicit)
2. `$EVIDRA_BENCHMARK_DIR` environment variable
3. `./tests/benchmark/` (relative to working directory)
4. `$HOME/.evidra/benchmark/` (user-level install)

### Dataset validation

Before running, the CLI validates:

1. `benchmark.yaml` exists and parses
2. All referenced cases exist on disk
3. Each case has `scenario.yaml`, `expected.json`, and `evidence/` directory
4. Evidence chains are non-empty and parse as valid JSONL
5. `expected.json` schema is valid

If validation fails, `run` exits with error and suggests `evidra benchmark validate --fix`.

### Dataset versioning

`benchmark.yaml` contains the dataset version:

```yaml
version: "1.0"
```

The CLI checks compatibility: CLI version must support the dataset version. If the dataset is newer than the CLI supports, error with upgrade instructions.

---

## 8. Recording Pipeline

`evidra benchmark record` is a separate workflow that generates evidence chains from raw artifacts. It is NOT part of `run`.

### Prerequisites

- `kind` installed
- `kubectl` installed
- `trivy` and/or `kubescape` installed (for scanner cases)
- `helm` installed (for helm cases)
- `argocd` CLI installed (for argocd cases, Phase 2)

### Recording flow

```
evidra benchmark record --case k8s-privileged-deploy

1. Read scenario.yaml for case
2. Determine isolation mode (namespace | cluster-reset)
3. Reset environment (create namespace or recreate cluster)
4. For each step in scenario.yaml:
   - scan:      run scanner, capture SARIF output
   - prescribe: call evidra prescribe with artifact + scanner report
   - execute:   run the actual command against kind cluster
   - report:    call evidra report with exit code
5. Write evidence chain to cases/<id>/evidence/
6. Validate: run signal detectors, compare against expected.json
7. Clean up (delete namespace or leave cluster for next case)
```

### Isolation modes

| Mode | When | How |
|---|---|---|
| `namespace` | Default. Cases that operate within a namespace. | Create `bench-<case-id>` namespace, delete after. |
| `cluster-reset` | Cases that modify cluster-wide resources (RBAC, CRDs, kube-system). | `kind delete cluster && kind create cluster`. |

Isolation mode is declared in `scenario.yaml` per case:

```yaml
isolation: namespace    # or cluster-reset
```

### Terraform recording

Terraform cases do NOT require a cloud provider. They use pre-recorded `plan.json`:

```
evidra benchmark record --case tf-mass-destroy

1. Read scenario.yaml
2. Evidence is generated from pre-recorded plan.json artifact
3. No terraform init/plan needed (plan.json is committed)
4. evidra prescribe --tool terraform --artifact artifacts/plan.json
5. evidra report --exit-code 0  (simulated success)
```

Optional re-record with local providers:

```
evidra benchmark record --case tf-mass-destroy --replan
```

This runs `terraform init && terraform plan` with local providers, generates fresh `plan.json`, then records evidence.

---

## 9. Implementation

### Package structure

```
cmd/evidra/
  main.go                    # existing — add "benchmark" to switch
  benchmark.go               # new — benchmark subcommand router

internal/benchmark/
  runner.go                  # case execution engine
  loader.go                  # dataset discovery + validation
  manifest.go                # benchmark.yaml parser
  case.go                    # single case evaluator
  report.go                  # output rendering (text, json, markdown)
  baseline.go                # baseline loading and comparison
  recorder.go                # evidence recording (for `record` subcommand)
  types.go                   # BenchmarkResult, CaseResult, etc.
```

### Integration with existing code

The benchmark runner reuses existing packages directly:

| Existing package | Used by benchmark for |
|---|---|
| `pkg/evidence` | Reading pre-recorded evidence chains |
| `internal/signal` | Running 5 signal detectors on evidence |
| `internal/score` | Computing scorecard per case |
| `internal/canon` | Adapter selection during `record` |
| `internal/risk` | Risk classification during `record` |
| `internal/sarif` | SARIF parsing during `record` |

No new dependencies. The benchmark is a **consumer** of the existing signal engine.

### CLI wiring

```go
// cmd/evidra/main.go — add to switch
case "benchmark":
    return cmdBenchmark(args[1:], stdout, stderr)
```

```go
// cmd/evidra/benchmark.go
func cmdBenchmark(args []string, stdout, stderr io.Writer) int {
    if len(args) == 0 {
        printBenchmarkUsage(stdout)
        return 0
    }
    switch args[0] {
    case "run":
        return cmdBenchmarkRun(args[1:], stdout, stderr)
    case "list":
        return cmdBenchmarkList(args[1:], stdout, stderr)
    case "validate":
        return cmdBenchmarkValidate(args[1:], stdout, stderr)
    case "record":
        return cmdBenchmarkRecord(args[1:], stdout, stderr)
    case "compare":
        return cmdBenchmarkCompare(args[1:], stdout, stderr)
    case "version":
        return cmdBenchmarkVersion(args[1:], stdout, stderr)
    default:
        fmt.Fprintf(stderr, "unknown benchmark command: %s\n", args[0])
        return 6 // usage error
    }
}
```

### Key types

```go
// internal/benchmark/types.go

// Manifest represents benchmark.yaml
type Manifest struct {
    Version     string            `yaml:"version"`
    Description string            `yaml:"description"`
    Suites      map[string]Suite  `yaml:"suites"`
    Cases       []CaseRef         `yaml:"cases"`
}

type Suite struct {
    Description string   `yaml:"description"`
    Cases       []string `yaml:"cases"`
}

type CaseRef struct {
    ID              string   `yaml:"id"`
    Category        string   `yaml:"category"`
    Difficulty      string   `yaml:"difficulty"`
    Signals         []string `yaml:"signals"`
    RiskDetails     []string `yaml:"risk_details"`
    GroundTruthRisk string   `yaml:"ground_truth_risk"`
}

// Expected represents expected.json per case
type Expected struct {
    CaseID              string                    `json:"case_id"`
    Difficulty          string                    `json:"difficulty"`
    GroundTruth         GroundTruth               `json:"ground_truth"`
    RiskDetailsExpected []string                  `json:"risk_details_expected"`
    SignalsExpected      map[string]SignalExpect   `json:"signals_expected"`
    RiskLevel           string                    `json:"risk_level"`
    ScoreRange          *ScoreRange               `json:"score_range,omitempty"`
    Performance         *PerformanceThresholds    `json:"performance,omitempty"`
    Tags                []string                  `json:"tags"`
}

type GroundTruth struct {
    InfrastructureRisk    string `json:"infrastructure_risk"`
    BlastRadiusResources  int    `json:"blast_radius_resources"`
    SecurityImpact        string `json:"security_impact"`
    Category              string `json:"category"`
    AttackSurface         string `json:"attack_surface"`
}

type SignalExpect struct {
    Expected bool `json:"expected,omitempty"`
    MinCount int  `json:"min_count,omitempty"`
}

type ScoreRange struct {
    Min float64 `json:"min"`
    Max float64 `json:"max"`
}

type PerformanceThresholds struct {
    MaxDetectionLatencyMs int `json:"max_detection_latency_ms"`
    ExpectedSignalCount   int `json:"expected_signal_count"`
    FalsePositiveTolerance int `json:"false_positive_tolerance"`
}

// CaseResult is the output of evaluating one case.
// Structure mirrors the per-case JSON output exactly.
type CaseResult struct {
    CaseID      string           `json:"case_id"`
    Suite       string           `json:"suite"`
    Tags        []string         `json:"tags"`
    Difficulty  string           `json:"difficulty"`
    Status      string           `json:"status"`       // "pass" | "fail" | "error"
    Risk        RiskComparison   `json:"risk"`
    Signals     SignalComparison `json:"signals"`
    RiskDetails DetailComparison `json:"risk_details"`
    Evidence    EvidenceRef      `json:"evidence"`
    Notes       []string         `json:"notes"`
}

type RiskComparison struct {
    Expected string `json:"expected"`
    Observed string `json:"observed"`
}

type SignalComparison struct {
    Expected   map[string]SignalExpect        `json:"expected"`
    Observed   map[string]SignalObserved      `json:"observed"`
    Missing    []string                       `json:"missing"`
    Unexpected []string                       `json:"unexpected"`
}

type SignalObserved struct {
    Count    int      `json:"count"`
    EventIDs []string `json:"event_ids"`
}

type DetailComparison struct {
    Expected   []string `json:"expected"`
    Observed   []string `json:"observed"`
    Missing    []string `json:"missing"`
    Unexpected []string `json:"unexpected"`
}

type EvidenceRef struct {
    Path      string        `json:"path"`
    LastHash  string        `json:"last_hash"`
    Validated ValidatedInfo `json:"validated"`
}

type ValidatedInfo struct {
    HashChain  bool `json:"hash_chain"`
    Signatures bool `json:"signatures"`
}

// BenchmarkResult is the top-level results.json output.
// Structure mirrors the JSON output exactly.
type BenchmarkResult struct {
    SchemaVersion string                        `json:"schema_version"`
    GeneratedAt   string                        `json:"generated_at"`
    Evidra        EvidraInfo                    `json:"evidra"`
    Dataset       DatasetInfo                   `json:"dataset"`
    Selection     SelectionInfo                 `json:"selection"`
    Summary       Summary                       `json:"summary"`
    Baseline      *BaselineInfo                 `json:"baseline,omitempty"`
    Baselines     map[string]BaselineComparison `json:"baselines"`
    ByDifficulty  map[string]GroupResult        `json:"by_difficulty"`
    BySuite       map[string]GroupResult        `json:"by_suite"`
    Cases         []CaseResult                  `json:"cases"`
}

type EvidraInfo struct {
    Version          string `json:"version"`
    Commit           string `json:"commit"`
    SignalSpecVersion string `json:"signal_spec_version"`
    ScoringVersion   string `json:"scoring_version"`
}

type DatasetInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Source  string `json:"source"`
}

type SelectionInfo struct {
    Suites     []string `json:"suites"`
    Cases      []string `json:"cases"`
    Tags       []string `json:"tags"`
    Difficulty []string `json:"difficulty"`
}

type BaselineInfo struct {
    Used         bool   `json:"used"`
    Path         string `json:"path"`
    Regressions  int    `json:"regressions"`
    Improvements int    `json:"improvements"`
}

type Summary struct {
    CasesTotal        int                `json:"cases_total"`
    CasesPassed       int                `json:"cases_passed"`
    CasesFailed       int                `json:"cases_failed"`
    InfraErrors       int                `json:"infra_errors"`
    Score             ScoreValue         `json:"score"`
    RiskCoverage      float64            `json:"risk_coverage"`
    FalsePositiveRate float64            `json:"false_positive_rate"`
    FalseNegativeRate float64            `json:"false_negative_rate"`
    RiskDistribution  map[string]int     `json:"risk_distribution"`
}

type ScoreValue struct {
    Value float64 `json:"value"`
    Scale string  `json:"scale"` // "0..1"
}

type BaselineComparison struct {
    Score float64 `json:"score"`
    Delta float64 `json:"delta"`
}

type GroupResult struct {
    Passed int     `json:"passed"`
    Total  int     `json:"total"`
    Score  float64 `json:"score"`
}
```

### Scenario parser

```go
// internal/benchmark/scenario.go

// Scenario represents scenario.yaml per case
type Scenario struct {
    ID              string         `yaml:"id"`
    Category        string         `yaml:"category"`
    Difficulty      string         `yaml:"difficulty"`
    Isolation       string         `yaml:"isolation"`  // "namespace" or "cluster-reset"
    Tools           []string       `yaml:"tools"`
    Steps           []Step         `yaml:"steps"`
    SignalsExpected  map[string]SignalExpect `yaml:"signals_expected"`
    RiskLevel       string         `yaml:"risk_level"`
}

type Step struct {
    Action        string `yaml:"action"`     // scan, prescribe, execute, report
    Tool          string `yaml:"tool"`
    Operation     string `yaml:"operation"`
    Artifact      string `yaml:"artifact"`
    ScannerReport string `yaml:"scanner_report"`
    Command       string `yaml:"command"`
    Namespace     string `yaml:"namespace"`
    ExitCode      *int   `yaml:"exit_code,omitempty"` // nil when not applicable (prescribe/execute steps)
    Args          []string `yaml:"args"`
    Output        string `yaml:"output"`
}
```

---

## 10. CI Integration

### GitHub Action: `evidra/benchmark-action`

One-step integration. The action:
1. Downloads `evidra` binary (release artifact)
2. Checks out dataset repo (or uses cached version)
3. Runs `evidra benchmark run`
4. Uploads results + evidence as workflow artifacts
5. Writes Job Summary

**User experience goal: one step.**

```yaml
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: evidra/benchmark-action@v1
        with:
          dataset: evidra/evidra-bench@v1.0.0
          suite: kubernetes
          fail-on: "any,regression,critical"
```

### Action inputs (v1)

| Input | Default | Notes |
|---|---|---|
| `dataset` | required | `org/repo@tag` or local path |
| `suite` | `all` | Comma-separated |
| `case` | | Optional comma-separated |
| `out` | `bench-out` | Artifact directory |
| `format` | `json` | Action should use json |
| `fail-on` | `any` | e.g. `any,regression,critical` |
| `baseline` | | Path or artifact from previous run |
| `upload-artifacts` | `true` | Upload results.json + summary |
| `results-name` | `evidra-benchmark-results` | Artifact name |

### Action outputs (v1)

| Output | Description |
|---|---|
| `score` | Aggregate benchmark score |
| `risk_level_max` | Highest risk level observed |
| `cases_failed` | Number of failed cases |
| `results_path` | Path to results.json |

### Manual workflow (without action)

```yaml
# .github/workflows/benchmark.yml
name: Benchmark
on:
  push:
    branches: [main]
  pull_request:

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build Evidra
        run: make build

      - name: Run benchmark
        run: |
          ./bin/evidra benchmark run \
            --dataset ./tests/benchmark \
            --format json \
            --out ./bench-out \
            --fail-on any \
            --fail-on critical

      - name: Job summary
        if: always()
        run: |
          ./bin/evidra benchmark run \
            --dataset ./tests/benchmark \
            --format markdown >> $GITHUB_STEP_SUMMARY

      - name: Upload results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: benchmark-results
          path: bench-out/results.json
```

### PR regression check

```yaml
      - name: Compare with main
        if: github.event_name == 'pull_request'
        run: |
          # Download baseline from main branch artifact
          ./bin/evidra benchmark compare \
            baseline-main.json bench-out/results.json \
            --format markdown >> $GITHUB_STEP_SUMMARY
```

---

## 11. Leaderboard Format

When the dataset moves to `evidra-dataset` repo (Phase 3), benchmark results become submittable:

```json
{
  "submission": {
    "system": "evidra",
    "version": "0.4.0",
    "submitted_at": "2026-03-15T10:00:00Z",
    "dataset_version": "1.0",
    "contact": "team@example.com"
  },
  "results": {
    "aggregate_score": 0.92,
    "risk_coverage": 0.96,
    "false_positive_rate": 0.01,
    "false_negative_rate": 0.02,
    "by_difficulty": {
      "easy": 1.0,
      "medium": 0.94,
      "hard": 0.88,
      "catastrophic": 0.80
    },
    "by_suite": {
      "kubernetes": 0.93,
      "terraform": 0.90,
      "helm": 0.80,
      "argocd": 0.80,
      "scanners": 1.0,
      "incidents": 0.80,
      "agents": 1.0
    }
  }
}
```

Leaderboard is rendered from submitted JSON files. No backend required initially — can be a static page generated from a directory of submission files.

```
Evidra Benchmark Leaderboard (dataset v1.0)

System              Version   Score   Risk Cov.   FP Rate   FN Rate
------------------- --------- ------- ----------- --------- ---------
evidra-reference    0.4.0     0.92    96%         1%        2%
langchain-agent     0.3.1     0.63    71%         8%        15%
custom-pipeline     2.1.0     0.82    88%         3%        6%
```

---

## 12. Suites and Case Taxonomy

### Standard suites (v1)

| Suite | Description |
|---|---|
| `kubernetes` | Manifests, RBAC, privileged pods, namespace escalation, system namespace changes |
| `helm` | Upgrade/rollback, install into forbidden namespaces, drift between prescribe/apply |
| `terraform` | Destructive plans, wildcard IAM, open security groups, drift, retry patterns |
| `argocd` | GitOps sync risks, app-of-apps, force-sync loops |
| `scanners` | SARIF-driven findings influencing risk (Trivy/Kubescape/Checkov) |
| `incidents` | Recreated real-world outage patterns |
| `agents` | Behavioral patterns (retry loop, scope escalation, protocol violations) |

### Case naming

Case IDs MUST be stable, kebab-case, and unique across dataset:
- `k8s-privileged-deploy`
- `tf-mass-destroy-100`
- `helm-upgrade-breaking`
- `agent-retry-loop-5x`
- `incident-mass-delete-prod`

---

## 13. Delivery Plan

### Phase 1: Core `run` command (v0.4.0)

- `evidra benchmark run` with text and JSON output
- Dataset loader + validator
- Case evaluator using existing signal engine
- Suite and difficulty filtering
- `evidra benchmark list`
- `evidra benchmark validate`

### Phase 2: Recording + baselines (v0.4.x)

- `evidra benchmark record` with kind cluster integration
- Baseline profiles and comparison
- `evidra benchmark compare` for run-to-run diff
- Markdown output for GitHub Actions job summary
- `--verbose` mode

### Phase 3: Leaderboard + extraction (v0.5.0+)

- Move dataset to `evidra-dataset` repo
- Submission format and validation
- Static leaderboard generator
- `evidra benchmark version` showing dataset origin

---

## 14. Constraints

### What the benchmark CLI is NOT

- NOT a live test runner — it evaluates pre-recorded evidence, not live infrastructure
- NOT a policy engine — it measures detection accuracy, not enforcement
- NOT a CI gate by default — use `--format json` + `jq` in CI to build gates
- NOT coupled to Evidra internals — any system implementing the signal spec can produce compatible evidence chains

### Performance budget

- `evidra benchmark run` on 50 cases: < 5 seconds
- No network calls during case evaluation (dataset fetch via `git+https://` happens at startup before evaluation begins)
- No disk writes during `run` (unless `--out` specified)
- `record` is slow (minutes) — involves kind cluster operations

### Compatibility

- Dataset version is semver. CLI declares supported dataset versions.
- New cases can be added without bumping dataset major version.
- Changing expected.json schema or scoring model requires dataset major bump.
- Older CLI versions gracefully skip unknown fields in benchmark.yaml and expected.json.
- Benchmark runner MUST use the same scoring engine as `evidra scorecard`.
- Dataset versioning MUST be independent from Evidra versioning.
- `results.json` MUST include both versions to enable reproducibility.

---

## 15. Related Documents

- [Benchmark Dataset Proposal](../plans/2026-03-05-benchmark-dataset-proposal.md) — dataset design, cases, recording pipeline
- [Signal Spec](EVIDRA_SIGNAL_SPEC.md) — normative signal definitions
- [Agent Reliability Benchmark](EVIDRA_AGENT_RELIABILITY_BENCHMARK.md) — scoring, comparison methodology
