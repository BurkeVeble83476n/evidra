# Evidra Canonicalization Test Strategy

## Status
Implemented. Replaces the previous 8000-test proposal.

## Principle
One test type per guarantee. No test infrastructure that costs
more to maintain than the code it tests.

---

## 1. Test Categories

All tests live in `internal/canon/canon_test.go`.

### Test A: Golden Corpus (10 cases)

Each case is one file. Input + tool/operation → expected intent_digest.

```
tests/golden/
  k8s_deployment.yaml          → k8s_deployment_digest.txt
  k8s_multidoc.yaml            → k8s_multidoc_digest.txt
  k8s_privileged.yaml          → k8s_privileged_digest.txt
  k8s_rbac.yaml                → k8s_rbac_digest.txt
  k8s_crd.yaml                 → k8s_crd_digest.txt
  tf_create.json               → tf_create_digest.txt
  tf_destroy.json              → tf_destroy_digest.txt
  tf_mixed.json                → tf_mixed_digest.txt
  tf_module.json               → tf_module_digest.txt
  helm_output.yaml             → helm_output_digest.txt
```

Each golden test is an individual function (`TestGolden_K8sDeployment`, etc.)
that validates:
- No parse error
- Correct `CanonVersion` (e.g., `k8s/v1`, `terraform/v1`)
- Correct `OperationClass`, `ResourceCount`, `ResourceIdentity`
- `IntentDigest` matches saved golden digest

```go
func TestGolden_K8sDeployment(t *testing.T) {
    t.Parallel()
    data := readGolden(t, "k8s_deployment.yaml")
    result := Canonicalize("kubectl", "apply", "", data)

    if result.ParseError != nil {
        t.Fatalf("parse error: %v", result.ParseError)
    }
    // ... validate canon version, resource count, operation class, identity ...

    digestFile := "k8s_deployment_digest.txt"
    if shouldUpdate() {
        writeGoldenDigest(t, digestFile, result.IntentDigest)
    } else {
        want := readGoldenDigest(t, digestFile)
        if result.IntentDigest != want {
            t.Errorf("intent digest mismatch\n got: %s\nwant: %s", result.IntentDigest, want)
        }
    }
}
```

Update mechanism: `EVIDRA_UPDATE_GOLDEN=1` environment variable (not a test flag).

If a digest changes unexpectedly → test fails → investigate.

Version bump = `EVIDRA_UPDATE_GOLDEN=1 go test -run TestGolden ./internal/canon/...`
+ review diff + commit. One command, not a 30-file edit.

### Test B: Noise Immunity (5 tests)

Each test injects one K8s noise field into `k8s_deployment.yaml`,
canonicalizes, and asserts `ResourceShapeHash` is unchanged.

| Test | Noise injected |
|------|---------------|
| `TestNoiseImmunity_MetadataUID` | `uid: abc-123-def` |
| `TestNoiseImmunity_ResourceVersion` | `resourceVersion: "12345"` |
| `TestNoiseImmunity_ManagedFields` | `managedFields:` block |
| `TestNoiseImmunity_GenerationTimestamp` | `generation:` + `creationTimestamp:` |
| `TestNoiseImmunity_Status` | `status:` block appended |

Noise fields are stripped by `noise.go` which maintains a frozen list
of K8s noise fields (`uid`, `resourceVersion`, `generation`,
`creationTimestamp`, `deletionTimestamp`, `deletionGracePeriodSeconds`,
`managedFields`, `selfLink`, `generateName`) plus annotation prefix
filtering (`kubectl.kubernetes.io/`, etc.).

### Test C: Shape Hash Sensitivity (1 test)

`TestShapeHashSensitivity_ReplicaChange` — verifies that changing
`replicas: 3` to `replicas: 5` produces a different `ResourceShapeHash`.
This is the positive case: spec changes must change the hash.

### Test D: Intent Digest Properties (1 test)

`TestIntentDigest_ExcludesShapeHash` — verifies that `IntentDigest`
does not include `ResourceShapeHash` (changing shape hash alone must
not change intent), but changing `Operation` does change the digest.

### Test E: Determinism (1 test)

`TestDeterminism_SameInputSameDigest` — same input canonicalized twice
produces identical `IntentDigest`, `ArtifactDigest`, and `ResourceShapeHash`.

### Test F: Generic Adapter (1 test)

`TestGenericAdapter` — unknown tool falls through to `GenericAdapter`,
returns `generic/v1` version, valid digests, `ResourceCount=1`.

### Test G: Scope Resolution (4 tests, 13 subtests)

- `TestResolveScopeClass_ExplicitEnv` — 5 table-driven cases (production, staging, development, case-insensitive, whitespace)
- `TestResolveScopeClass_NamespaceHints` — 6 table-driven cases (namespace patterns → scope inference)
- `TestResolveScopeClass_EnvOverridesNamespace` — explicit env wins over namespace hint
- `TestResolveScopeClass_NoResourcesNoEnv` — returns `"unknown"`

---

## 2. What We Dropped and Why

| Previous proposal | Why dropped |
|-------------------|-------------|
| 3000 noise mutations per adapter | 5 noise tests on one input. Same bugs caught. |
| Semantic mutation tests | shape_hash = SHA256(spec). Spec changes → hash changes. Trivially correct. One unit test. |
| Identity mutation tests | Identity = 4 strings hashed. Trivially correct. |
| Cross-version boundary tests | Add when v2 exists. Not before. |
| Structured fuzz generator | Go native fuzz seeded from golden is enough. Add when bugs found. |
| Performance benchmarks | Profile when slow. Not before. |
| metadata.json per corpus case | Comment in test file is enough. |
| Separate shape_hash golden files | One unit test for shape_hash logic. Not per-corpus. |
| Mutation test runner framework | Inline test code. No framework. |

---

## 3. When to Add More Tests

Add a golden case when:
- New resource type with unusual structure
- Bug fix where digest was wrong

Add a noise immunity test when:
- New noise field discovered in production

Add fuzz testing when:
- Crash found in production

Do not add tests preemptively.

---

## 4. Version Bump Process

```
1. Change the adapter code
2. go test → golden tests fail
3. EVIDRA_UPDATE_GOLDEN=1 go test -run TestGolden ./internal/canon/... → regenerate digests
4. git diff tests/golden/ → review: are changes expected?
5. Bump canonicalization_version in adapter
6. git commit -m "canon: bump k8s/v2, reason: ..."
```

---

## 5. CI

No CI pipeline configured yet. Run locally:

```bash
go test ./... -v -count=1        # all tests
make golden-update               # regenerate golden digests
```

Add CI when the project has a deployment pipeline.

---

## 6. Total

| Category | Tests | Subtests |
|----------|-------|----------|
| Golden corpus | 10 | — |
| Noise immunity | 5 | — |
| Shape hash sensitivity | 1 | — |
| Intent digest properties | 1 | — |
| Determinism | 1 | — |
| Generic adapter | 1 | — |
| Scope resolution | 4 | 13 |
| **Total** | **23** | **13** |

~580 lines of test code. Covers digest stability, noise immunity,
shape sensitivity, scope resolution, determinism, and adapter fallback.
Maintainable by one person.
