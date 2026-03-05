# MCP Inspector E2E Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add deterministic MCP Inspector e2e coverage for `prescribe/report/get_event` with `local-mcp` default mode plus opt-in `local-rest`, `hosted-mcp`, and `hosted-rest`.

**Architecture:** Reuse the old `evidra-mcp` inspector runner pattern, but adapt transport and assertions to the benchmark protocol (`prescribe/report`). Keep one shell runner with mode abstraction, special checks, curated case files, and explicit skip-by-default behavior for networked modes.

**Tech Stack:** Bash, `jq`, `curl`, MCP Inspector CLI (`npx @modelcontextprotocol/inspector`), Go binary `cmd/evidra-mcp`, Makefile.

---

### Task 1: Scaffold Inspector Test Layer

**Files:**
- Create: `tests/inspector/run_inspector_tests.sh`
- Create: `tests/inspector/README.md`
- Create: `tests/inspector/mcp-config.json`
- Create: `tests/inspector/special/.gitkeep`
- Create: `tests/inspector/cases/.gitkeep`
- Create: `tests/inspector/fixtures/.gitkeep`

**Step 1: Write the failing test**

Run:

```bash
bash tests/inspector/run_inspector_tests.sh
```

Expected: shell error (`No such file or directory`).

**Step 2: Run test to verify it fails**

Run the same command and confirm non-zero exit.

**Step 3: Write minimal implementation**

- Add runner skeleton with:
  - `set -euo pipefail`
  - mode parsing (`EVIDRA_TEST_MODE`, default `local-mcp`)
  - counters (`PASS/FAIL/SKIP`)
  - `pass/fail/skip/section` helpers
  - final summary block and exit-on-fail behavior
- Add minimal README describing modes and prerequisites.
- Add MCP config:

```json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--evidence-dir", "/tmp/evidra-inspector-evidence"]
    }
  }
}
```

**Step 4: Run test to verify it passes**

Run:

```bash
bash tests/inspector/run_inspector_tests.sh
```

Expected: successful summary with zero failures.

**Step 5: Commit**

```bash
git add tests/inspector
git commit -m "test: scaffold MCP inspector e2e runner structure"
```

### Task 2: Add `local-mcp` Transport Helpers

**Files:**
- Modify: `tests/inspector/run_inspector_tests.sh`

**Step 1: Write the failing test**

Add a minimal special test that calls `call_list_tools` before helper implementation:

- Create temporary check in runner invoking `call_list_tools`.

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: failure due undefined helper / command path.

**Step 2: Run test to verify it fails**

Confirm non-zero exit and helper failure.

**Step 3: Write minimal implementation**

Implement helpers in runner:

- `check_prerequisites` (`jq`, `npx`, Go toolchain for local mode)
- build local binary:

```bash
go build -o bin/evidra-mcp ./cmd/evidra-mcp
```

- `inspector_call_tool` wrapper (tools/call)
- `inspector_list_tools` wrapper (tools/list)
- `extract_body` parser:

```bash
jq '.structuredContent // (.content[0].text | fromjson) // .'
```

**Step 4: Run test to verify it passes**

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: runner starts and reaches summary without transport helper failures.

**Step 5: Commit**

```bash
git add tests/inspector/run_inspector_tests.sh
git commit -m "test: add local MCP inspector transport helpers"
```

### Task 3: Add Special Test `t_list_tools.sh`

**Files:**
- Create: `tests/inspector/special/t_list_tools.sh`
- Modify: `tests/inspector/run_inspector_tests.sh`

**Step 1: Write the failing test**

Implement `t_list_tools.sh` assertions for required tools and schema fields:

- tools: `prescribe`, `report`, `get_event`
- required fields:
  - prescribe: `tool`, `operation`, `raw_artifact`, `actor`
  - report: `prescription_id`, `exit_code`
  - get_event: `event_id`

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: one or more assertion failures before special loader wiring is complete.

**Step 2: Run test to verify it fails**

Confirm failure is from missing/incorrect special test integration.

**Step 3: Write minimal implementation**

- Add special script sourcing loop:

```bash
for script in "$SCRIPT_DIR"/special/t_*.sh; do
  source "$script"
done
```

- Ensure `MODE`, `pass/fail/skip`, and transport helpers are available to sourced scripts.

**Step 4: Run test to verify it passes**

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: tool/schema checks pass.

**Step 5: Commit**

```bash
git add tests/inspector/special/t_list_tools.sh tests/inspector/run_inspector_tests.sh
git commit -m "test: verify MCP tool registration and schema requirements"
```

### Task 4: Add Special Test `t_schema_error.sh`

**Files:**
- Create: `tests/inspector/special/t_schema_error.sh`
- Modify: `tests/inspector/run_inspector_tests.sh`

**Step 1: Write the failing test**

Create invalid `prescribe` input (missing required fields/invalid actor) and assert:

- local/hosted MCP: JSON-RPC invalid params (`-32602`).
- rest modes: HTTP 400.

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: failure until schema error path and stderr capture are wired.

**Step 2: Run test to verify it fails**

Confirm failure originates from schema error check.

**Step 3: Write minimal implementation**

- Add optional stderr passthrough in `inspector_call_tool`.
- Add mode-aware schema error assertions.

**Step 4: Run test to verify it passes**

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: schema error special test passes.

**Step 5: Commit**

```bash
git add tests/inspector/special/t_schema_error.sh tests/inspector/run_inspector_tests.sh
git commit -m "test: add schema error coverage for inspector transport modes"
```

### Task 5: Add Fixtures and `t_get_event_chain.sh`

**Files:**
- Create: `tests/inspector/fixtures/safe-nginx-deployment.yaml`
- Create: `tests/inspector/fixtures/privileged-pod.yaml`
- Create: `tests/inspector/special/t_get_event_chain.sh`
- Modify: `tests/inspector/run_inspector_tests.sh`

**Step 1: Write the failing test**

Create chain test:

1. `prescribe` safe manifest
2. capture `prescription_id`
3. `report` with exit code `0`
4. `get_event` for both IDs
5. assert returned entries are `prescribe` and `report`

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: failure before `call_prescribe/call_report/call_get_event/reset_evidence` are complete.

**Step 2: Run test to verify it fails**

Confirm chain assertions fail in red state.

**Step 3: Write minimal implementation**

- Add runner helpers:
  - `reset_evidence`
  - `call_prescribe`
  - `call_report`
  - `call_get_event`
- Keep `get_event` checks skipped in REST modes.

**Step 4: Run test to verify it passes**

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: chain test passes locally.

**Step 5: Commit**

```bash
git add tests/inspector/fixtures tests/inspector/special/t_get_event_chain.sh tests/inspector/run_inspector_tests.sh
git commit -m "test: add prescribe-report-get_event chain inspector test"
```

### Task 6: Add Curated Scenario Case Loop

**Files:**
- Create: `tests/inspector/cases/lifecycle_ok.json`
- Create: `tests/inspector/cases/parse_error_artifact.json`
- Create: `tests/inspector/cases/risk_tag_privileged.json`
- Create: `tests/inspector/cases/report_unknown_prescription.json`
- Create: `tests/inspector/cases/cross_actor_report.json`
- Create: `tests/inspector/cases/schema_error_prescribe.json`
- Modify: `tests/inspector/run_inspector_tests.sh`

**Step 1: Write the failing test**

Add case files and scenario loop with assertions not yet implemented.

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: failing assertions due missing case executor.

**Step 2: Run test to verify it fails**

Confirm failures are case-loop related.

**Step 3: Write minimal implementation**

Implement case executor:

- load each `cases/*.json`
- dispatch action sequence (`prescribe`, `report`, optional `get_event`)
- support variable capture (`prescription_id`, `report_id`)
- assert:
  - `ok`
  - `error.code`
  - tag contains
  - protocol behavior for unknown/cross-actor report

**Step 4: Run test to verify it passes**

Run:

```bash
EVIDRA_TEST_MODE=local-mcp bash tests/inspector/run_inspector_tests.sh
```

Expected: curated case set passes.

**Step 5: Commit**

```bash
git add tests/inspector/cases tests/inspector/run_inspector_tests.sh
git commit -m "test: add curated inspector scenario corpus for prescribe/report"
```

### Task 7: Add `local-rest` and `hosted-*` Mode Abstraction

**Files:**
- Modify: `tests/inspector/run_inspector_tests.sh`
- Modify: `tests/inspector/special/t_schema_error.sh`
- Modify: `tests/inspector/README.md`

**Step 1: Write the failing test**

Run without required env:

```bash
EVIDRA_TEST_MODE=local-rest bash tests/inspector/run_inspector_tests.sh
EVIDRA_TEST_MODE=hosted-mcp bash tests/inspector/run_inspector_tests.sh
EVIDRA_TEST_MODE=hosted-rest bash tests/inspector/run_inspector_tests.sh
```

Expected: red (mode unsupported or hard failure).

**Step 2: Run test to verify it fails**

Confirm failure path before skip logic.

**Step 3: Write minimal implementation**

- Add modes:
  - `local-mcp` (default)
  - `local-rest` (opt-in)
  - `hosted-mcp` (opt-in + `EVIDRA_ENABLE_NETWORK_TESTS=1`)
  - `hosted-rest` (opt-in + `EVIDRA_ENABLE_NETWORK_TESTS=1`)
- Add REST endpoints in helpers:
  - `POST /v1/prescribe`
  - `POST /v1/report`
  - optional evidence fetch path
- Add retry wrapper for hosted modes only.
- Add skip-with-reason when env prerequisites are missing.

**Step 4: Run test to verify it passes**

Run:

```bash
bash tests/inspector/run_inspector_tests.sh
EVIDRA_TEST_MODE=local-rest bash tests/inspector/run_inspector_tests.sh
EVIDRA_TEST_MODE=hosted-mcp bash tests/inspector/run_inspector_tests.sh
EVIDRA_TEST_MODE=hosted-rest bash tests/inspector/run_inspector_tests.sh
```

Expected:
- default local-mcp run executes.
- local-rest/hosted modes skip cleanly unless env is configured.

**Step 5: Commit**

```bash
git add tests/inspector/run_inspector_tests.sh tests/inspector/special/t_schema_error.sh tests/inspector/README.md
git commit -m "test: add local-rest and hosted inspector mode abstractions"
```

### Task 8: Wire Makefile Targets + Final Verification

**Files:**
- Modify: `Makefile`

**Step 1: Write the failing test**

Run:

```bash
make test-mcp-inspector
```

Expected: `No rule to make target`.

**Step 2: Run test to verify it fails**

Confirm missing target.

**Step 3: Write minimal implementation**

Add targets:

```make
test-mcp-inspector:
	bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-local-rest:
	EVIDRA_TEST_MODE=local-rest bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-hosted:
	EVIDRA_TEST_MODE=hosted-mcp bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-hosted-rest:
	EVIDRA_TEST_MODE=hosted-rest bash tests/inspector/run_inspector_tests.sh
```

**Step 4: Run test to verify it passes**

Run:

```bash
make test-mcp-inspector
go test ./pkg/mcpserver -v -count=1
```

Expected: inspector local mode passes and MCP package tests remain green.

**Step 5: Commit**

```bash
git add Makefile
git commit -m "build: add make targets for inspector e2e test modes"
```

### Task 9: Full Verification Before Completion

**Files:**
- Verify only (no new files expected)

**Step 1: Run complete verification**

```bash
make test-mcp-inspector
go test ./... -count=1
```

Optional network verification when env is available:

```bash
EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-mcp bash tests/inspector/run_inspector_tests.sh
EVIDRA_ENABLE_NETWORK_TESTS=1 EVIDRA_TEST_MODE=hosted-rest bash tests/inspector/run_inspector_tests.sh
```

**Step 2: Confirm outputs**

- Zero failing inspector checks in default local mode.
- Hosted/rest checks are skipped cleanly when env is absent.
- No regression in existing Go tests.

**Step 3: Final commit (if any remaining changes)**

```bash
git add tests/inspector Makefile
git commit -m "test: add MCP inspector e2e baseline with transport modes"
```
