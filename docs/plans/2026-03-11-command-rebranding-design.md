# Command Rebranding Design

## Goal

Rebrand the user-facing Evidra CLI command surface to better match the project
positioning as a flight recorder for infrastructure automation, while keeping
the deeper lifecycle/domain model unchanged.

The required command renames are:

- `run` -> `record`
- `record` -> `import`
- `ingest-findings` -> `import-findings`

This wave also updates user-facing docs, landing copy, and tests so the new
command language is consistent everywhere users see the product.

## Problem

The current CLI has a naming mismatch with the intended product story:

- `run` is the live execution path that best matches "record what happened"
- `record` is the post-hoc ingest path that behaves more like "import a result"
- `ingest-findings` is mechanically clear but inconsistent with the new
  `import` naming family

That mismatch creates avoidable friction in docs and product positioning. It is
especially visible in the main onboarding path, where the preferred compact
command should feel polished and minimal:

```bash
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml
```

## Desired End State

### Public CLI names

The active public command surface becomes:

- `evidra record` = execute and record a live operation
- `evidra import` = ingest a completed operation from structured input
- `evidra import-findings` = ingest SARIF findings

The following commands remain unchanged:

- `prescribe`
- `report`
- `scorecard`
- `explain`
- `compare`
- `validate`
- `keygen`
- `prompts`
- `detectors`
- `benchmark`

### No backwards compatibility

Old command names are removed from the CLI surface:

- no `evidra run`
- no old `evidra record` meaning
- no `evidra ingest-findings`

There are no compatibility aliases and no deprecation warnings.

### Internal scope boundary

Rename the CLI package internals too, so maintainers do not live with permanent
command confusion inside `cmd/evidra/`.

Included:

- command registry entries
- CLI file/function/flag/help naming
- CLI tests and help output

Excluded:

- deeper lifecycle/domain naming such as `Prescribe`, `Report`,
  `OperationProcessor`, evidence entry types, MCP tool names, and API semantics
- docs that describe lifecycle semantics but do not expose user-facing CLI
  command names

## Compact Command Form

The compact polished path is a first-class requirement, not a nice-to-have.

### Live execution

Primary ergonomic form:

```bash
evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml
```

Requirements:

- add `-f` as the short form of `--artifact`
- keep the wrapped command form with `--`
- require `--` and a wrapped command; `record` without the separator is a clear
  usage error
- infer `tool` and `operation` only for a fixed v1 allowlist of wrapped command
  forms
- require explicit flags when inference is unsupported or ambiguous rather than
  guessing
- keep `evidra` flags before `--` and wrapped-command flags after `--`, so
  `evidra record -f deploy.yaml -- kubectl apply -f deploy.yaml` is valid and
  the two `-f` flags do not collide

### Deterministic inference scope (v1)

Inference is intentionally limited to exactly these five tools:

- `kubectl <op> -f <file>` -> `tool=kubectl`, `operation=<op>`,
  `artifact=<file>`
- `oc <op> -f <file>` -> `tool=oc`, `operation=<op>`, `artifact=<file>`
- `helm <op> <release> <dir>` -> `tool=helm`, `operation=<op>`
- `terraform <op> <file|dir>` -> `tool=terraform`, `operation=<op>`,
  `artifact=<file|dir>` when directly inferable
- `docker <op>` -> `tool=docker`, `operation=<op>`

Everything else fails with a clear error:

```text
please specify --tool and --operation explicitly
```

### Risk assessment before execution

Also support compact artifact flag:

```bash
evidra prescribe -f deploy.yaml --tool kubectl --operation apply
```

### Post-hoc import

The post-hoc path stays simple:

```bash
evidra import --input result.json
```

### Findings import

The findings path becomes:

```bash
evidra import-findings --sarif scanner.sarif
```

The public flag name stays `--sarif`.

## Documentation Scope

Update user-facing docs and UI where commands appear:

- `README.md`
- landing page copy
- CLI/help examples
- guides and docs that show human-invoked commands
- tests that grep docs/help output for command names

Do not mechanically rename every occurrence of `record` or `run` in the repo.
Only change places where they refer to the public CLI command names or user
positioning.

## Recommended Approach

### Option 1: Surface-only rename

Rename only registry entries and docs, leaving CLI implementation files/functions
with old names.

Pros:

- smallest code churn

Cons:

- maintainers keep a confusing mismatch inside `cmd/evidra/`
- future CLI work becomes error-prone

### Option 2: CLI-package rename with stable deeper layers

Rename the CLI surface and the corresponding code inside `cmd/evidra/`, while
leaving deeper lifecycle/domain naming unchanged.

Pros:

- matches the public CLI
- avoids chaos inside the CLI package
- contains the blast radius

Cons:

- more churn than a registry-only patch

### Option 3: Full repo semantic rename

Rename CLI, domain docs, internal naming, and adjacent concepts everywhere.

Pros:

- maximum terminology consistency

Cons:

- exceeds the requested scope
- high churn and regression risk

### Recommendation

Use Option 2.

This gives a clean public CLI and a maintainable `cmd/evidra/` package without
dragging MCP/API/lifecycle layers into a much larger rename.

## Risks

### Risk: mixed terminology in docs

Some docs talk about CLI commands, while others talk about lifecycle semantics.
Mechanical replacements will overreach.

Mitigation:

- update only docs that expose user-invoked commands
- leave deeper prescribe/report protocol docs unchanged unless they show CLI
  examples that now changed

### Risk: ambiguous compact inference

Inferring `tool` and `operation` from wrapped commands can become unreliable if
done too aggressively.

Mitigation:

- support only the fixed five-tool deterministic matrix above
- fail with `please specify --tool and --operation explicitly` for everything
  else
- add explicit tests for the dual-`-f` wrapped kubectl example and for
  `record` without `--`

### Risk: accidental release metadata churn

This rename wave changes the public CLI but does not itself require a runtime
version bump.

Mitigation:

- update `CHANGELOG.md`
- leave `pkg/version/version.go` unchanged in this task

### Risk: test/doc grep drift

This repo has many grep-based shell tests over docs and command names.

Mitigation:

- audit all grep-based command tests
- add or update guards so old CLI names do not remain in user-facing docs/help

## Verification Strategy

Required verification after implementation:

- main CLI help lists `record`, `import`, `import-findings` and not the old
  names
- live execution behavior still works through the new `record` command
- post-hoc ingest behavior still works through the new `import` command
- SARIF ingest behavior still works through `import-findings`
- user-facing docs and landing references are updated
- compact `-f` artifact path works for `record` and `prescribe`
- `record -f deploy.yaml -- kubectl apply -f deploy.yaml` works and parses both
  `-f` flags correctly
- `record` without `--` fails with a clear usage error
- unsupported wrapped commands fail with `please specify --tool and --operation explicitly`
- `CHANGELOG.md` is updated and runtime version remains unchanged

Suggested verification commands:

```bash
go test ./cmd/evidra -count=1
bash scripts/check-doc-commands.sh
rg -n '\bevidra run\b|\bevidra record\b|\bevidra ingest-findings\b' README.md docs ui tests
```

## Output

At the end of this wave:

- users see a coherent CLI naming scheme aligned with the product story
- `cmd/evidra/` internals match the public CLI names
- compact command examples become the default documentation style
- deeper lifecycle semantics remain stable and do not absorb unnecessary churn
