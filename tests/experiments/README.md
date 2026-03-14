# Experiment Tests

Structure:
- `internal/experiments/*_test.go` — adapter and runner unit/integration tests in Go.

Quick run:

```bash
go test ./internal/experiments -count=1
```

## Execution-Mode Testing

Execution-mode experiments (real agent + real cluster + prescribe/report protocol)
have moved to **evidra-infra-bench**. See: https://github.com/vitas/evidra-infra-bench
