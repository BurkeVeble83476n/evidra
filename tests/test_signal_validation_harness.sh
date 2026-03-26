#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -Fq 'run_evidra_json()' tests/signal-validation/helpers.sh \
  || fail "helpers.sh should validate evidra JSON responses"

if grep -Fq '|| true' tests/signal-validation/helpers.sh; then
  fail "helpers.sh should not swallow evidra command failures"
fi

grep -Fq 'backdate_evidence_entries()' tests/signal-validation/helpers.sh \
  || fail "helpers.sh should support deterministic backdating for TTL-based checks"

grep -Fq 'check_sequence_coverage()' tests/signal-validation/validate-signals-engine.sh \
  || fail "validation harness should verify expectation coverage"

grep -Fq 'register_sequence "I_escalation"' tests/signal-validation/validate-signals-engine.sh \
  || fail "validation harness should execute risk escalation sequence"

grep -Fq 'RESULTS_DIR="${RESULTS_DIR:-/tmp/evidra-signal-validation-results/$RUN_STAMP}"' tests/signal-validation/validate-signals-engine.sh \
  || fail "validation harness should write results to /tmp by default"

grep -Fq 'backdate_evidence_entries "$SEQ_C_DIR" 120' tests/signal-validation/validate-signals-engine.sh \
  || fail "protocol violation sequence should backdate evidence instead of relying on wall clock timing"

grep -Fq 'docs/guides/signal-validation.md' tests/signal-validation/README.md \
  || fail "signal validation README should point at the public guide"

grep -Fq 'retained signal/scoring harness' tests/signal-validation/README.md \
  || fail "signal validation README should describe the retained harness accurately"

if grep -Eq 'removed experiments harness|experiments decommissioning' tests/signal-validation/README.md docs/guides/signal-validation.md docs/system-design/EVIDRA_ARCHITECTURE_V1.md; then
  fail "public docs should not mention the removed experiments surface"
fi

grep -Fq '/tmp/evidra-signal-validation-results/<timestamp>/' tests/signal-validation/README.md \
  || fail "signal validation README should document the /tmp results directory"

grep -Eq '^test-signals: build$' Makefile \
  || fail "make test-signals should rebuild the CLI before running the harness"

echo "PASS: test_signal_validation_harness"
