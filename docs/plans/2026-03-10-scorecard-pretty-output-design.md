# Scorecard Pretty Output Design

**Date:** 2026-03-10
**Status:** Approved

## Context

`evidra scorecard` currently emits JSON only. That is correct for machine
consumption, but it is weak for direct CLI use in terminals, CI logs, and
manual inspection.

The immediate need is a human-oriented output mode that can be requested
explicitly without breaking the current JSON contract.

The design also needs to address one usability gap in the current scorecard:
`period=30d` describes the query window, but it does not show how much real
activity exists inside that window. A score built from one day of activity and a
score built from eight distinct days of activity currently look the same.

## Goals

1. Add a human-readable scorecard mode behind `--pretty`.
2. Keep JSON as the default output to avoid breaking automation.
3. Add `days_observed` so users can distinguish query window from actual
   observed activity.
4. Keep the implementation dependency-light and easy to snapshot-test.

## Non-Goals

1. This does not introduce a general `--format` system yet.
2. This does not redesign score computation or signal semantics.
3. This does not add color, terminal capability detection, or paging.
4. This does not change `evidra explain` or `evidra compare` in the first pass.

## Options Considered

### Option 1: Built-in ASCII renderer

Render a bordered ASCII summary and signal table directly in Go using a small
renderer helper.

**Pros**
- No new dependency
- Deterministic output
- Easy to keep stable in tests
- Fits the lightweight CLI shape

**Cons**
- Slightly more manual formatting code

### Option 2: Third-party table library

Use a package such as `tablewriter` to render the pretty layout.

**Pros**
- Faster initial implementation
- Less width-calculation code

**Cons**
- Adds dependency weight for a small feature
- Harder to guarantee exact stable output over time

### Option 3: Introduce generic output modes now

Replace `--pretty` with `--format json|pretty|compact` immediately.

**Pros**
- Cleaner long-term CLI model

**Cons**
- Larger contract/design surface than needed for this feature
- More churn across docs and tests

## Recommendation

Use **Option 1** now.

Add a dedicated built-in ASCII renderer for `scorecard` and keep the feature
behind `--pretty`. This keeps the implementation small, avoids unnecessary
dependencies, and leaves room to refactor toward `--format` later if more
renderers appear.

## CLI Behavior

### Default

`evidra scorecard` continues to emit JSON.

### New flag

`evidra scorecard --pretty`

This switches the output from JSON to human-readable ASCII.

The flag is explicit. There is no auto-detection of TTY/non-TTY in this change.

## New Data Field

### `days_observed`

Definition:

- Count of distinct **UTC calendar days** with at least one matching
  prescription entry in the filtered scorecard dataset.

Why this definition:

- It answers the real question: "How many days of activity does this score
  represent?"
- It is more useful than elapsed span between first and last operation.
- It does not confuse sparse activity with continuous observation.

Examples:

- `period: 30d`, `days_observed: 1`
- `period: 30d`, `days_observed: 8`

`period` remains the requested window. `days_observed` describes observed
activity inside that window.

## Output Shape

### Summary section

The pretty output should show:

- actor
- session
- period
- days_observed
- total_operations
- score
- band
- penalty
- sufficient
- confidence
- scoring_version
- spec_version
- evidra_version

### Signals section

One row per signal:

- signal
- count
- rate
- profile
- weight

### Footer notes

Show only when relevant:

- insufficient data note
- confidence ceiling / caveat

## Formatting Rules

1. ASCII only
2. Plain bordered tables
3. Stable row order
4. Fixed numeric formatting for score, penalty, and rates
5. No colors
6. No Unicode box drawing

## Architecture

The implementation should build one shared scorecard view model and then render
it through one of two paths:

1. JSON renderer
2. Pretty ASCII renderer

This avoids maintaining two separate scorecard assembly paths.

Recommended shape:

- keep score computation unchanged
- compute `days_observed` from the filtered entries
- build a render-ready scorecard model
- route to JSON or pretty renderer based on `--pretty`

## Data Source for `days_observed`

`days_observed` should be computed from the **filtered evidence set** already
selected by:

- actor
- period
- session
- tool
- scope

Only prescription entries should contribute to `days_observed`, because the
scorecard is operation-based.

## Testing Strategy

1. JSON regression test:
   - `days_observed` appears in JSON output
   - existing fields remain intact
2. Pretty output test:
   - summary header renders
   - signal table renders
   - `days_observed` appears correctly
3. Filtering test:
   - `days_observed` respects session and period filters
4. Stability test:
   - output uses predictable ordering and formatting

## Documentation Impact

If implemented, update:

- CLI help text in `cmd/evidra/main.go`
- CLI reference
- any scorecard examples that should show `days_observed`

## Acceptance Criteria

The feature is complete when:

1. `evidra scorecard` still outputs JSON by default.
2. `evidra scorecard --pretty` outputs a human-readable ASCII report.
3. JSON output includes `days_observed`.
4. Pretty output includes both summary and per-signal sections.
5. `days_observed` is computed from distinct UTC days with matching
   prescriptions.
6. Tests cover both JSON and pretty modes.
