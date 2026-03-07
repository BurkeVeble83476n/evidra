# Setup Evidra Marketplace Checklist

Target external action repo: `evidra-io/setup-evidra`

## Scope

This checklist is for publishing a standalone installation action to GitHub Marketplace.
It does not replace the in-repo benchmark composite action.

## Checklist

1. Create public repository `evidra-io/setup-evidra`.
2. Copy `.github/actions/setup-evidra/action.yml` to repo root as `action.yml`.
3. Add `README.md` with:
   - inputs/outputs table
   - supported OS/arch matrix
   - minimum `permissions` needed
   - example workflow snippets
4. Add semantic tags (`v1`, `v1.0.0`) and release notes.
5. Verify install against current Evidra release assets for:
   - Linux amd64
   - Linux arm64
   - macOS amd64
   - macOS arm64
6. Add CI in action repo:
   - shellcheck for install script section
   - smoke workflow invoking action + `evidra version`
7. Publish to Marketplace:
   - set category
   - add branding icon/color
   - verify metadata and examples
8. Back-link docs:
   - update Evidra README to reference Marketplace action
   - update `docs/guides/setup-evidra-action.md` to prefer marketplace URL once published

## Post-publish guardrails

- Keep `v1` tag moving to latest compatible patch.
- Do not ship breaking input/output changes under `v1`.
- Run smoke tests against every Evidra release before moving `v1`.
