# Scorecard Pretty Output Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `evidra scorecard --pretty` for human-readable ASCII output and add `days_observed` to scorecard output.

**Architecture:** Keep JSON as the default contract, add a shared scorecard view model, compute `days_observed` from filtered prescription entries, and render either JSON or ASCII from the same assembled data. Avoid third-party table dependencies in the first pass.

**Tech Stack:** Go, standard library CLI rendering, existing score/signal pipeline, Go tests

---

### Task 1: Add failing scorecard regression tests

**Files:**
- Modify: `cmd/evidra/main_test.go`

**Step 1: Write the failing JSON regression test**

Add a test that:
- creates scorecard evidence across multiple UTC dates
- runs `evidra scorecard`
- asserts JSON contains `days_observed`

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./cmd/evidra -count=1
```

Expected: FAIL because `days_observed` is not present yet.

**Step 3: Write the failing pretty-output regression test**

Add a test that:
- runs `evidra scorecard --pretty`
- asserts output includes:
  - summary header
  - `days_observed`
  - signal table section

**Step 4: Run test to verify it fails**

Run:

```bash
go test ./cmd/evidra -count=1
```

Expected: FAIL because `--pretty` is not implemented yet.

### Task 2: Introduce a shared scorecard view model

**Files:**
- Modify: `cmd/evidra/main.go`

**Step 1: Extract scorecard assembly into a shared helper**

Create a helper that assembles:
- summary fields
- signal rows
- `days_observed`

Use the same filtered evidence/signal path already used by `cmdScorecard`.

**Step 2: Compute `days_observed`**

Count distinct UTC calendar dates from matching prescription entries only.

**Step 3: Keep JSON output wired to the new model**

Ensure the default JSON renderer still emits the existing fields plus
`days_observed`.

**Step 4: Run tests**

Run:

```bash
go test ./cmd/evidra -count=1
```

Expected: JSON regression passes, pretty-output test still fails.

### Task 3: Implement the ASCII renderer

**Files:**
- Modify: `cmd/evidra/main.go`

**Step 1: Add the `--pretty` flag**

Add a bool flag to `scorecard`.

**Step 2: Implement a small ASCII table renderer**

Render:
- title/header
- summary table
- signals table
- optional footer note

Requirements:
- ASCII only
- stable row ordering
- no external dependencies

**Step 3: Switch output mode when `--pretty` is set**

Do not change default JSON behavior.

**Step 4: Run tests**

Run:

```bash
go test ./cmd/evidra -count=1
```

Expected: pretty-output regression now passes.

### Task 4: Update help text and docs

**Files:**
- Modify: `cmd/evidra/main.go`
- Modify: `docs/integrations/CLI_REFERENCE.md`
- Modify: `docs/system-design/EVIDRA_END_TO_END_EXAMPLE_v2.md` (if scorecard example is kept user-facing)

**Step 1: Document `--pretty`**

Update CLI help and reference docs.

**Step 2: Document `days_observed`**

Add concise explanation that it is distinct UTC calendar days with matching
prescriptions inside the selected window.

**Step 3: Run doc consistency checks if applicable**

Run:

```bash
bash scripts/check-doc-commands.sh
```

If the script does not cover these docs, note that explicitly.

### Task 5: Full verification

**Files:**
- Verify only

**Step 1: Run targeted CLI tests**

```bash
go test ./cmd/evidra -count=1
```

**Step 2: Run full Go suite**

```bash
go test ./... -count=1
```

**Step 3: Run formatting check**

```bash
files=$(gofmt -l .)
printf '%s\n' "$files"
```

Expected: no output.

**Step 4: Commit**

```bash
git add cmd/evidra/main.go cmd/evidra/main_test.go docs/integrations/CLI_REFERENCE.md docs/system-design/EVIDRA_END_TO_END_EXAMPLE_v2.md
git commit -m "feat: add pretty scorecard output"
```
