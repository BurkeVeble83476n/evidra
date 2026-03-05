# Scanner SARIF Quickstart

This quickstart standardizes scanner ingestion in Evidra with two defaults:

- Trivy for Terraform/general IaC
- Kubescape for Kubernetes manifests

Both scanners feed the same Evidra contract:

```bash
--scanner-report <sarif-file>
```

Signing guidance:
- Default (`strict`): configure `EVIDRA_SIGNING_KEY` or `EVIDRA_SIGNING_KEY_PATH`
- Local smoke mode: use `--signing-mode optional` or `EVIDRA_SIGNING_MODE=optional`

## 1. Trivy (Terraform / IaC default)

```bash
# Generate SARIF report
trivy config . --format sarif > scanner_report.sarif

# Ingest with Evidra
evidra prescribe \
  --tool terraform \
  --artifact plan.json \
  --signing-mode optional \
  --scanner-report scanner_report.sarif
```

## 2. Kubescape (Kubernetes default)

```bash
# Generate SARIF report
kubescape scan . --format sarif --output scanner_report_k8s.sarif

# Ingest with Evidra
evidra prescribe \
  --tool kubectl \
  --artifact manifest.yaml \
  --signing-mode optional \
  --scanner-report scanner_report_k8s.sarif
```

## Why this model

- Single ingestion path in Evidra (`--scanner-report`)
- No per-scanner code path in core logic
- Clear buyer-facing defaults for IaC and Kubernetes
