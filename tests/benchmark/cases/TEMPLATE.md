# Benchmark Case Template

Use this template for every benchmark case directory under:

`tests/benchmark/cases/<case-id>/`

## README.md

```markdown
# <case-id>

## Scenario: <short title>

**Category:** <kubernetes|terraform|helm|argocd>  
**Difficulty:** <easy|medium|hard|catastrophic>  
**Dataset label:** limited-contract-baseline

**Scenario class:** <safe_routine|normal_mutate|high_risk_ambiguous|decline_worthy>  
**Operation class:** <inspect_read|mutate_change|deploy_rollout>  
**Environment class:** <sandbox|staging|prod_like>

**Story:** <what the automation is trying to do>

**Impact:** <why the behavior matters operationally>

**Risk:** <what makes the scenario risky or safe>

**Real-world parallel:** <incident, policy, or upstream pattern>
```

## expected.json

```json
{
  "case_id": "<case-id>",
  "dataset_label": "limited-contract-baseline",
  "case_kind": "artifact",
  "category": "<kubernetes|terraform|helm|argocd>",
  "difficulty": "<easy|medium|hard|catastrophic>",
  "scenario_class": "<safe_routine|normal_mutate|high_risk_ambiguous|decline_worthy>",
  "operation_class": "<inspect_read|mutate_change|deploy_rollout>",
  "environment_class": "<sandbox|staging|prod_like>",
  "ground_truth_pattern": "<signal-or-risk-pattern>",
  "artifact_ref": "../../../artifacts/fixtures/<family>/<filename>",
  "artifact_digest": "sha256:<digest-or-TODO>",
  "risk_details_expected": [],
  "risk_level": "<low|medium|high|critical>",
  "signals_expected": {},
  "tags": [],
  "processing": {},
  "source_refs": [
    {
      "source_id": "<source-id>",
      "composition": "real-derived"
    }
  ]
}
```

## Notes

- These fields describe the scenario itself, not a behavioral run overlay.
- Actor/model/verdict/decision metadata belongs to later run datasets, not static OSS imports.
- `decline_worthy` is an allowed scenario class now even if no current benchmark case uses it yet.
- Benchmark cases should reference shared fixtures directly; do not copy
  per-case duplicates into `tests/benchmark/cases/<case-id>/artifacts/`.
