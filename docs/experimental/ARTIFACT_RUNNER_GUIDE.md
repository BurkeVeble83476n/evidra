# Artifact Runner Guide

This guide covers the `evidra-exp` artifact runner:
- `artifact run` (single-model evaluation)
- `artifact baseline` (multi-model comparison on the same case set)

Use it to understand purpose, execution flow, commands, and result interpretation.

## Purpose

`artifact run` answers: "How well does one model classify risk on the benchmark artifact set?"

`artifact baseline` answers: "Given identical prompt/cases/settings, which model performs better?"

Both commands evaluate model output against case expectations in `tests/benchmark/cases` and produce machine-readable artifacts for audit and comparison.

## How It Works

1. Load cases from `expected.json` files under `--cases-dir` (default `tests/benchmark/cases`).
2. Apply `--case-filter` (regex on `case_id`) and `--max-cases` if provided.
3. Resolve prompt metadata from `--prompt-file` and optional `--prompt-version`.
4. For each selected case and repeat:
   - Build run id: `<timestamp>-<safe_model_id>-<case_id>-r<repeat>`
   - Execute adapter (`claude`, `bifrost`, `dry-run`) with timeout.
   - Persist raw logs/artifacts per run.
   - Evaluate predicted risk level/tags vs expected values.
   - Append one row to `summary.jsonl`.
5. Print run counters (`success`, `failure`, `timeout`, `dry_run`, `eval_pass`, `eval_fail`).

`artifact baseline` wraps `artifact run` per model id, then aggregates per-model metrics into `<out_dir>/summary.json` (`schema_version: evidra.llm-baseline.v1`).

## How To Run

### 1) Sanity check (`dry-run`)

```bash
go run ./cmd/evidra-exp artifact run \
  --model-id test/model \
  --provider test \
  --agent dry-run \
  --repeats 1 \
  --max-cases 1
```

### 2) Single-model real run

```bash
go run ./cmd/evidra-exp artifact run \
  --model-id anthropic/claude-3-5-haiku \
  --provider bifrost \
  --agent bifrost \
  --prompt-file prompts/experiments/runtime/system_instructions.txt \
  --prompt-version v1 \
  --repeats 3 \
  --timeout-seconds 300 \
  --delay-between-runs 2s
```

### 3) Multi-model baseline (aggregated summary)

```bash
go run ./cmd/evidra-exp artifact baseline \
  --model-ids anthropic/claude-3-5-haiku,openai/gpt-4o-mini \
  --provider bifrost \
  --agent bifrost \
  --prompt-file prompts/experiments/runtime/system_instructions.txt \
  --prompt-version v1 \
  --repeats 3 \
  --timeout-seconds 300 \
  --delay-between-runs 2s
```

### 4) Baseline dry-run for CI smoke

```bash
go run ./cmd/evidra-exp artifact baseline \
  --model-ids test/model-a,test/model-b \
  --provider test \
  --agent dry-run \
  --repeats 1 \
  --max-cases 1
```

## Output Layout

Single-model run default root:
- `experiments/results/<timestamp>/`

Baseline run default root:
- `experiments/results/llm/<timestamp>/`
- per-model subdir: `<safe_model_id>/`

Per-run files (inside each run directory):
- `agent_stdout.log`
- `agent_stderr.log`
- `agent_output.json`
- `agent_raw_stream.jsonl`
- `result.json`

Indexes:
- `summary.jsonl` (per-run rows) for each model run directory
- `summary.json` (baseline aggregate) at baseline root

## How To Interpret Results

### A) Runtime health vs quality

- `execution.status` (`success|failure|timeout|dry_run`) is runtime behavior of the adapter invocation.
- `evaluation.pass` is benchmark quality against expected risk outputs.

These are intentionally separate. A run can have `status=success` but `pass=false`.

### B) What counts as pass

For artifact mode, `pass=true` only when:
- expected risk level matches predicted risk level (if expected level is provided), and
- predicted risk detail tags have zero false positives and zero false negatives.

This is strict exact-match behavior on risk tags.

### C) Metric semantics

In `result.json`:
- `precision = TP / (TP + FP)` (or `null` if denominator is zero)
- `recall = TP / (TP + FN)` (or `null` if denominator is zero)
- `f1` computed from precision/recall (or `null` if undefined)

In baseline `summary.json` (`models[]`):
- `pass_rate = pass_count / runs_total`
- `avg_precision`, `avg_recall`, `avg_f1` are means across runs with non-null values
- `risk_level_match_rate` is measured only on runs where expected risk level is non-empty
- `status_counts` shows operational stability (`timeout` and `failure` spikes indicate harness or model execution issues)

`comparison` block:
- `best_pass_rate_model`: model with highest pass rate (tie -> lexicographically smaller model id)
- `best_f1_model`: model with highest average F1 among models with non-null F1

### D) Practical reading order

1. Check `status_counts` first (execution reliability).
2. Compare `pass_rate` across models (primary quality KPI).
3. Use `avg_f1` to compare detail-tag quality when pass rates are close.
4. Inspect failed run `result.json` files for false-positive / false-negative patterns.

## Quick Inspection Commands

Per-model pass rate from `summary.jsonl`:

```bash
jq -s '{total:length, pass:(map(select(.pass==true))|length), pass_rate:(if length==0 then 0 else ((map(select(.pass==true))|length)/length) end)}' \
  experiments/results/<stamp>/summary.jsonl
```

Baseline leaderboard from `summary.json`:

```bash
jq '.models | sort_by(-.pass_rate) | map({model_id, pass_rate, avg_f1, status_counts})' \
  experiments/results/llm/<stamp>/summary.json
```

## Common Pitfalls

- `--model-id` is required for `artifact run`; `--model-ids` is required for `artifact baseline`.
- `--agent` is required in both modes.
- `--clean-out-dir` removes existing files in target output dir; use cautiously.
- `--case-filter` uses Go regex syntax.
- `--prompt-file` and `--cases-dir` must exist, otherwise run fails early.
