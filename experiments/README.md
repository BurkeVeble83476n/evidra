# Experiments

This folder is for running and storing artifact-classification benchmark outputs.

## What to use

- Go CLI: `evidra-exp` (`go run ./cmd/evidra-exp ...` or `bin/evidra-exp` after build)
- CLI flags and experiment commands: `docs/integrations/cli-reference.md`
- Run modes and output layout: this file (`experiments/README.md`)
- Experiment prompt contract: `prompts/experiments/runtime/system_instructions.txt`
- Prompt source contract: `prompts/source/contracts/v1.3.0/`
- Prompt source-of-truth spec: `docs/system-design/EVIDRA_PROMPT_FACTORY_SPEC_V1.md`

Prompt editing policy:
- Edit only `prompts/source/contracts/<version>/...`
- Regenerate active prompts with `make prompts-generate`
- Verify no drift with `make prompts-verify`

## Execution-Mode Testing

Execution-mode experiments (real agent + real cluster + prescribe/report protocol)
have moved to **evidra-infra-bench**:

```bash
# In the evidra-infra-bench repo:
infra-bench run --provider claude --model sonnet --scenario kubernetes/broken-deployment
infra-bench run --provider bifrost --model openai/gpt-4o --scenario ...
infra-bench lab  # interactive TUI
```

See: https://github.com/vitas/evidra-infra-bench

## Quick Start

Dry run (sanity check):

```bash
go run ./cmd/evidra-exp artifact run \
  --model-id test/model \
  --agent dry-run \
  --repeats 1 \
  --max-cases 1
```

Real run with Bifrost adapter:

```bash
go run ./cmd/evidra-exp artifact run \
  --model-id anthropic/claude-3-5-haiku \
  --provider bifrost \
  --agent bifrost \
  --prompt-version v1 \
  --delay-between-runs 2s \
  --repeats 3 \
  --timeout-seconds 300
```

Multi-model baseline (same cases/settings, aggregated output):

```bash
go run ./cmd/evidra-exp artifact baseline \
  --model-ids anthropic/claude-3-5-haiku,openai/gpt-4o-mini \
  --provider bifrost \
  --agent bifrost \
  --prompt-version v1 \
  --delay-between-runs 2s \
  --repeats 3 \
  --timeout-seconds 300
```

Claude headless run (chat subscription path, no Anthropic API credits required):

```bash
# prerequisite: claude CLI installed and logged in
go run ./cmd/evidra-exp artifact run \
  --model-id claude/haiku \
  --provider claude \
  --agent claude \
  --mode local-mcp \
  --prompt-file prompts/experiments/runtime/system_instructions.txt \
  --delay-between-runs 2s \
  --repeats 3 \
  --max-cases 1 \
  --clean-out-dir \
  --timeout-seconds 300
```

The Claude adapter maps `claude/<alias>` to CLI `--model <alias>` (for example `claude/haiku`, `claude/sonnet`, `claude/opus`).

Notes:
- If `--prompt-version` is omitted, the runner uses prompt file `# contract: ...` header.

## Output Layout

By default, results are written to `experiments/results/<timestamp>/`.

Each run contains:
- `agent_stdout.log`
- `agent_stderr.log`
- `agent_output.json`
- `agent_raw_stream.jsonl`
- `result.json`

A run index is written to `summary.jsonl` in the timestamp folder with fields:
- `run_id`, `case_id`, `status`, `pass`, `result_json`

For `artifact baseline`, each model gets its own `summary.jsonl`, and the baseline root adds `summary.json` with per-model aggregate metrics (`pass_rate`, average precision/recall/F1, risk-level match rate, and comparison winners).
