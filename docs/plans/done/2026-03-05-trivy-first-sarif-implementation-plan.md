# Trivy + Kubescape SARIF Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Trivy and Kubescape the documented default scanners while keeping one scanner-agnostic SARIF ingestion path in Evidra.

**Architecture:** Keep `--scanner-report` as the single ingestion contract, harden parser/CLI reliability, and prove compatibility with Trivy + Kubescape fixtures. Keep findings as evidence context and avoid scanner-specific code paths in core logic.

**Tech Stack:** Go 1.x CLI (`cmd/evidra`), SARIF parser (`internal/sarif`), evidence model (`pkg/evidence`), docs (`README.md`, `docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md`).

---

### Task 1: Add Trivy and Kubescape SARIF Fixtures + Parser Tests

**Files:**
- Create: `tests/testdata/sarif_trivy.json`
- Create: `tests/testdata/sarif_kubescape.json`
- Modify: `internal/sarif/parser_test.go`

**Step 1: Write failing parser tests first**

```go
func TestParseSARIF_Trivy(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../tests/testdata/sarif_trivy.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	findings, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	if findings[0].Tool != "trivy" {
		t.Fatalf("tool: got %q, want trivy", findings[0].Tool)
	}
}

func TestParseSARIF_Kubescape(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../tests/testdata/sarif_kubescape.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	findings, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	if findings[0].Tool != "kubescape" {
		t.Fatalf("tool: got %q, want kubescape", findings[0].Tool)
	}
}
```

**Step 2: Run tests to verify RED**

Run: `go test ./internal/sarif -run 'TestParseSARIF_(Trivy|Kubescape)' -v`  
Expected: FAIL (fixtures missing)

**Step 3: Add minimal fixtures**

```json
{
  "version": "2.1.0",
  "runs": [
    {
      "tool": { "driver": { "name": "Trivy" } },
      "results": [
        {
          "ruleId": "AVD-AWS-0001",
          "level": "error",
          "message": { "text": "Example high issue" },
          "locations": [
            { "physicalLocation": { "artifactLocation": { "uri": "main.tf" } } }
          ]
        }
      ]
    }
  ]
}
```

```json
{
  "version": "2.1.0",
  "runs": [
    {
      "tool": { "driver": { "name": "Kubescape" } },
      "results": [
        {
          "ruleId": "KSV001",
          "level": "warning",
          "message": { "text": "Container is running as root" },
          "locations": [
            { "physicalLocation": { "artifactLocation": { "uri": "deployment.yaml" } } }
          ]
        }
      ]
    }
  ]
}
```

**Step 4: Run parser suite to verify GREEN**

Run: `go test ./internal/sarif -run 'TestParseSARIF_(Trivy|Kubescape|Empty|InvalidJSON)' -v`  
Expected: PASS

**Step 5: Commit Task 1**

```bash
git add tests/testdata/sarif_trivy.json tests/testdata/sarif_kubescape.json internal/sarif/parser_test.go
git commit -m "test: add Trivy and Kubescape SARIF parser fixtures"
```

### Task 2: Harden Parser Normalization Behavior

**Files:**
- Modify: `internal/sarif/parser.go`
- Modify: `internal/sarif/parser_test.go`

**Step 1: Write failing edge-case tests**

```go
func TestMapSeverity_CriticalAlias(t *testing.T) {
	t.Parallel()
	got := mapSeverity("critical")
	if got != "critical" {
		t.Fatalf("mapSeverity(critical) = %q, want critical", got)
	}
}

func TestParseSARIF_MissingToolNameDefaultsUnknown(t *testing.T) {
	t.Parallel()
	data := []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{}},"results":[{"ruleId":"X","level":"warning","message":{"text":"m"}}]}]}`)
	findings, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if findings[0].Tool != "unknown" {
		t.Fatalf("tool: got %q, want unknown", findings[0].Tool)
	}
}
```

**Step 2: Run tests to verify RED**

Run: `go test ./internal/sarif -run 'Test(MapSeverity_CriticalAlias|ParseSARIF_MissingToolNameDefaultsUnknown)' -v`  
Expected: FAIL

**Step 3: Implement minimal normalization changes**

```go
func Parse(data []byte) ([]evidence.FindingPayload, error) {
	// unmarshal
	for _, run := range report.Runs {
		toolName := strings.ToLower(strings.TrimSpace(run.Tool.Driver.Name))
		if toolName == "" {
			toolName = "unknown"
		}
		for _, result := range run.Results {
			resource := ""
			if len(result.Locations) > 0 {
				resource = result.Locations[0].PhysicalLocation.ArtifactLocation.URI
			}
			findings = append(findings, evidence.FindingPayload{
				Tool:     toolName,
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
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return "critical"
	case "error", "high":
		return "high"
	case "warning", "medium":
		return "medium"
	case "note", "low":
		return "low"
	default:
		return "info"
	}
}
```

**Step 4: Run parser package**

Run: `go test ./internal/sarif -v`  
Expected: PASS

**Step 5: Commit Task 2**

```bash
git add internal/sarif/parser.go internal/sarif/parser_test.go
git commit -m "feat: improve SARIF normalization for tool and severity"
```

### Task 3: Improve CLI Scanner Ingestion Reliability Signals

**Files:**
- Create: `cmd/evidra/main_test.go`
- Modify: `cmd/evidra/main.go`
- Reuse fixture: `tests/testdata/sarif_trivy.json`

**Step 1: Write failing CLI tests**

```go
func TestRunPrescribe_ScannerReportParseError(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "artifact.json")
	badSarif := filepath.Join(tmp, "bad.sarif")
	os.WriteFile(artifact, []byte(`{"noop":true}`), 0o644)
	os.WriteFile(badSarif, []byte(`not json`), 0o644)

	args := []string{
		"prescribe",
		"--tool", "terraform",
		"--artifact", artifact,
		"--canonical-action", `{"tool":"terraform","operation":"apply","operation_class":"mutate","scope_class":"production","resource_count":1,"resource_shape_hash":"sha256:test"}`,
		"--scanner-report", badSarif,
		"--evidence-dir", tmp,
	}
	var out, err bytes.Buffer
	code := run(args, &out, &err)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(err.String(), "parse scanner report") {
		t.Fatalf("stderr missing parse scanner report: %s", err.String())
	}
}
```

**Step 2: Run tests to verify RED**

Run: `go test ./cmd/evidra -run 'TestRunPrescribe_ScannerReport' -v`  
Expected: FAIL

**Step 3: Implement minimal reliability updates in `cmdPrescribe`**

```go
writtenFindings := 0
for _, f := range findings {
	findingPayload, _ := json.Marshal(f)
	lastHash, _ := evidence.LastHashAtPath(evidencePath)
	findingEntry, err := evidence.BuildEntry(evidence.EntryBuildParams{
		Type:           evidence.EntryTypeFinding,
		TraceID:        cr.ArtifactDigest,
		Actor:          actor,
		ArtifactDigest: cr.ArtifactDigest,
		Payload:        findingPayload,
		PreviousHash:   lastHash,
		SpecVersion:    "0.3.0",
		AdapterVersion: version.Version,
	})
	if err != nil {
		fmt.Fprintf(stderr, "warning: build finding entry failed for rule %s: %v\n", f.RuleID, err)
		continue
	}
	if err := evidence.AppendEntryAtPath(evidencePath, findingEntry); err != nil {
		fmt.Fprintf(stderr, "warning: write finding entry failed for rule %s: %v\n", f.RuleID, err)
		continue
	}
	writtenFindings++
}
result["findings_count"] = writtenFindings
```

**Step 4: Run verification tests**

Run:
- `go test ./cmd/evidra -run 'TestRunPrescribe_ScannerReport' -v`
- `go test ./internal/sarif ./pkg/evidence -v`

Expected: PASS

**Step 5: Commit Task 3**

```bash
git add cmd/evidra/main.go cmd/evidra/main_test.go
git commit -m "fix: improve scanner ingestion warnings and success counts"
```

### Task 4: Document Trivy + Kubescape Defaults

**Files:**
- Modify: `README.md`
- Modify: `docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md`
- Create: `docs/integrations/SCANNER_SARIF_QUICKSTART.md`

**Step 1: Write failing docs checklist**

Checklist:
- README includes Trivy + Kubescape default examples.
- Docs clearly state one scanner ingestion contract (`--scanner-report`).
- Scanner quickstart doc exists for these two defaults.

**Step 2: Verify checklist currently fails**

Run: `rg -n "Kubescape|Trivy|scanner-report|SARIF" README.md docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md docs/integrations 2>/dev/null`

Expected: Missing `docs/integrations/SCANNER_SARIF_QUICKSTART.md` and no explicit dual-default framing.

**Step 3: Add minimal docs content**

```md
## Scanner Context (Trivy + Kubescape Defaults)

Terraform/IaC default:

```bash
trivy config . --format sarif > scanner_report.sarif
evidra prescribe --tool terraform --artifact plan.json --scanner-report scanner_report.sarif
```

Kubernetes default:

```bash
kubescape scan . --format sarif --output scanner_report_k8s.sarif
evidra prescribe --tool kubectl --artifact manifest.yaml --scanner-report scanner_report_k8s.sarif
```

Both scanners use the same SARIF ingestion contract in Evidra.
```

**Step 4: Re-run docs checks**

Run: `rg -n "Trivy|Kubescape|--scanner-report|SARIF" README.md docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md docs/integrations/SCANNER_SARIF_QUICKSTART.md -S`

Expected: Matches in all three files.

**Step 5: Commit Task 4**

```bash
git add README.md docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md docs/integrations/SCANNER_SARIF_QUICKSTART.md
git commit -m "docs: define Trivy and Kubescape as default scanner paths"
```

### Task 5: Verification and Handoff

**Files:**
- Verify all touched files.

**Step 1: Run focused tests**

Run: `go test ./internal/sarif ./cmd/evidra ./pkg/evidence -v`

Expected: PASS

**Step 2: Run full suite**

Run: `go test ./... -count=1`

Expected: PASS

**Step 3: Verify docs artifacts**

Run:
- `test -f docs/integrations/SCANNER_SARIF_QUICKSTART.md`
- `rg -n "SCANNER_SARIF_QUICKSTART|Kubescape|Trivy" README.md docs/system-design/done/EVIDRA_INTEGRATION_ROADMAP.md -S`

Expected: file exists and references present.

**Step 4: Prepare buyer-confidence summary evidence**

Capture:
- Trivy default command.
- Kubescape default command.
- Shared SARIF contract statement.
- Passing parser/CLI verification outputs.

**Step 5: Final commit if needed**

```bash
git add -A
git commit -m "chore: finalize Trivy + Kubescape scanner integration verification"
```

(Only if pending tracked changes remain after verification.)

---

## Execution Notes

- Apply `@superpowers:test-driven-development` before each code change.
- Apply `@superpowers:verification-before-completion` before completion claims.
- Keep core scanner handling contract-based and scanner-agnostic.
