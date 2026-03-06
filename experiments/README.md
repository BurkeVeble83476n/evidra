# Experiments

This folder is for running and storing real-agent benchmark experiment outputs.

## What to use

- Runner script: `scripts/run-agent-experiments.sh`
- LiteLLM agent command wrapper: `scripts/agent-cmd-litellm.sh`
- Matrix definition: `docs/experimental/EXPERIMENT_MATRIX.md`
- Result schema: `docs/experimental/RESULT_SCHEMA.json`
- LiteLLM prompt contract: `prompts/experiments/litellm/system_instructions.txt`
- Prompt source contract: `prompts/source/contracts/v1.0.1/`
- Prompt source-of-truth spec: `docs/system-design/EVIDRA_PROMPT_FACTORY_SPEC.md`

Prompt editing policy:
- Edit only `prompts/source/contracts/<version>/...`
- Regenerate active prompts with `make prompts-generate`
- Verify no drift with `make prompts-verify`

## Quick Start

Dry run (sanity check):

```bash
bash scripts/run-agent-experiments.sh \
  --model-id test/model \
  --dry-run \
  --repeats 1 \
  --max-cases 1
```

Real run (agent command must write JSON to `$EVIDRA_AGENT_OUTPUT`):

```bash
bash scripts/run-agent-experiments.sh \
  --model-id anthropic/claude-3-5-haiku \
  --provider anthropic \
  --prompt-version v1 \
  --repeats 3 \
  --timeout-seconds 300 \
  --agent-cmd '...your harness command...'
```

LiteLLM run (prompted, contract-versioned):

```bash
export LITELLM_MODEL_ID="anthropic/claude-3-5-haiku"
export LITELLM_TEMPERATURE="0"

bash scripts/run-agent-experiments.sh \
  --model-id "$LITELLM_MODEL_ID" \
  --provider anthropic \
  --mode local-mcp \
  --prompt-file prompts/experiments/litellm/system_instructions.txt \
  --repeats 3 \
  --timeout-seconds 300 \
  --agent-cmd 'bash scripts/agent-cmd-litellm.sh'
```

Notes:
- If `--prompt-version` is omitted, the runner uses prompt file `# contract: ...` header.
- `EVIDRA_PROMPT_FILE`, `EVIDRA_PROMPT_VERSION`, and `EVIDRA_PROMPT_CONTRACT_VERSION`
  are exported to each agent run.

## Expected Agent Output JSON

```json
{
  "predicted_risk_level": "critical",
  "predicted_risk_details": ["k8s.privileged_container"]
}
```

## Output Layout

By default, results are written to `experiments/results/<timestamp>/`.

Each run contains:
- `agent_stdout.log`
- `agent_stderr.log`
- `agent_output.json`
- `result.json`

A run index is written to `summary.jsonl` in the timestamp folder.
