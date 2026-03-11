# Compact Record Examples Design

## Summary

This change standardizes how `evidra record` examples are presented across user-facing materials. The compact form should be the first example users see in fast-start surfaces, while comprehensive guides should show both the compact and expanded forms.

## Problem

The CLI already supports concise wrapped-command usage such as:

```bash
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml
```

But the public docs emphasize expanded multi-line examples inconsistently. That makes the common happy path harder to discover, especially in landing and quickstart surfaces.

## Goals

- Make the compact `record` form prominent in high-traffic onboarding surfaces.
- Preserve expanded examples where additional flags are useful for explanation.
- Keep one consistent editorial rule that contributors can follow.
- Add a small regression guard so the compact form remains present in key docs.

## Non-Goals

- No CLI behavior changes unless an existing compact-form path lacks test coverage.
- No broad rewrite of long CI/YAML command blocks purely to force one-line formatting.
- No changes to internal architecture or command semantics.

## Doc Policy

### 1. Fast-start surfaces use compact form first

The first `evidra record` example in these surfaces should use the compact form:

- landing page
- README quickstart/setup sections
- short setup or quickstart guides

Canonical example:

```bash
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml
```

### 2. Full guides show both forms

Guides that teach production usage or explain flags in detail should present:

1. a compact example for recognition and copy/paste
2. an expanded multi-line example for readability when using more flags such as `--environment`, `--actor`, `--url`, or `--api-key`

### 3. CI/YAML stays readable

YAML and CI snippets may remain multi-line when line length or readability would suffer. The goal is discoverability of the compact form, not forcing every example into one line.

## Scope

### In scope

- `README.md`
- `ui/src/pages/Landing.tsx`
- `cmd/evidra-api/static/index.html`
- `docs/integrations/CLI_REFERENCE.md`
- quickstart/setup guides that demonstrate `evidra record`

### Out of scope

- internal or archival design docs
- examples that do not use `record`
- automated reformatting unrelated to `record` examples

## Verification

- Update or add a lightweight documentation guard to ensure key user-facing docs contain the compact `record -f ... -- <tool>` example.
- Keep existing CLI test coverage for the compact form; add coverage only if a gap is discovered while updating docs/tests.

## Expected Outcome

New users should encounter the compact `record` invocation immediately in landing and quickstart materials, while advanced guides still provide the expanded version for clarity around optional flags.
