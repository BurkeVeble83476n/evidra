# Scanner SARIF Quickstart

This quickstart standardizes scanner ingestion in Evidra with two defaults:

- Trivy for Terraform/general IaC
- Kubescape for Kubernetes manifests

Both scanners feed the same Evidra contract:

```bash
--scanner-report <sarif-file>
```

## 1. Trivy (Terraform / IaC default)

```bash
# Generate SARIF report
trivy config . --format sarif > scanner_report.sarif

# Ingest with Evidra
evidra prescribe \
  --tool terraform \
  --artifact plan.json \
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
  --scanner-report scanner_report_k8s.sarif
```

## Why this model

- Single ingestion path in Evidra (`--scanner-report`)
- No per-scanner code path in core logic
- Clear buyer-facing defaults for IaC and Kubernetes
