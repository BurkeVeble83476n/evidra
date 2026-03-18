# External Lifecycle Ingest Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a generic product-side external lifecycle ingest API in Evidra and migrate webhook ingestion onto it.

**Architecture:** Introduce a new `internal/ingest` package that owns server-side prescribe/report ingest contracts, validation, claim handling, and evidence entry construction. Add authenticated `/v1/evidence/ingest/prescribe` and `/v1/evidence/ingest/report` routes, then refactor existing webhook handlers into thin translators over that shared service.

**Tech Stack:** Go, `net/http`, existing `internal/store`, existing evidence/signing pipeline, OpenAPI YAML, Markdown docs, Go tests

---

### Task 1: Define the ingest contract types and validation rules

**Files:**
- Create: `internal/ingest/contracts.go`
- Create: `internal/ingest/contracts_test.go`
- Reference: `pkg/evidence/payloads.go`
- Reference: `internal/lifecycle/types.go`

**Step 1: Write the failing contract tests**

Cover:
- valid prescribe request with canonical action
- valid report request with prescription id + verdict
- missing contract version
- missing taxonomy fields
- declined report missing `decision_context`
- declined report incorrectly including `exit_code`
- non-declined report missing `exit_code`
- invalid payload override combinations

**Step 2: Run the contract tests to verify they fail**

Run: `go test ./internal/ingest -run 'TestValidate' -v`

Expected: FAIL because the package and validators do not exist yet.

**Step 3: Add contract types and validators**

Define:
- shared envelope types
- prescribe request type
- report request type
- typed validation errors
- helper validation functions

Keep the contract explicit and small. Do not add AgentGateway-specific fields.

**Step 4: Run the contract tests to verify they pass**

Run: `go test ./internal/ingest -run 'TestValidate' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingest/contracts.go internal/ingest/contracts_test.go
git commit -s -m "feat: define external ingest contracts"
```

### Task 2: Build the `internal/ingest` service

**Files:**
- Create: `internal/ingest/service.go`
- Create: `internal/ingest/service_test.go`
- Create: `internal/ingest/errors.go`
- Reference: `internal/automationevent/emitter.go`
- Reference: `internal/store/entries.go`

**Step 1: Write failing service tests**

Cover:
- ingest prescribe creates and stores a signed prescribe entry
- ingest report creates and stores a signed report entry
- duplicate claim returns duplicate result without storing twice
- report resolves referenced prescription
- taxonomy fields propagate into typed payloads

**Step 2: Run the service tests to verify they fail**

Run: `go test ./internal/ingest -run 'TestService' -v`

Expected: FAIL because the service does not exist yet.

**Step 3: Implement the service**

Add:
- a small store dependency interface
- claim/idempotency handling
- shared entry save flow
- prescribe build path
- report build path
- typed service errors

Reuse existing evidence/building logic where possible. Avoid copying the current
`automationevent` logic verbatim if a helper can be extracted cleanly.

**Step 4: Run the service tests to verify they pass**

Run: `go test ./internal/ingest -run 'TestService' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingest/service.go internal/ingest/service_test.go internal/ingest/errors.go
git commit -s -m "feat: add external ingest service"
```

### Task 3: Add the API handlers for external ingest

**Files:**
- Create: `internal/api/ingest_handler.go`
- Create: `internal/api/ingest_handler_test.go`
- Modify: `internal/api/router.go`

**Step 1: Write failing handler tests**

Cover:
- authenticated prescribe ingest route accepts valid input
- authenticated report ingest route accepts valid input
- invalid payload returns 400
- duplicate claim response is stable
- route requires auth

**Step 2: Run the handler tests to verify they fail**

Run: `go test ./internal/api -run 'TestIngest' -v`

Expected: FAIL because the routes and handlers do not exist yet.

**Step 3: Implement the handlers and route wiring**

Add:
- JSON decode/validate
- tenant extraction from auth middleware
- service invocation
- stable JSON responses
- router wiring for:
  - `POST /v1/evidence/ingest/prescribe`
  - `POST /v1/evidence/ingest/report`

**Step 4: Run the handler tests to verify they pass**

Run: `go test ./internal/api -run 'TestIngest' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/ingest_handler.go internal/api/ingest_handler_test.go internal/api/router.go
git commit -s -m "feat: add external ingest api routes"
```

### Task 4: Migrate webhook ingestion onto the shared service

**Files:**
- Modify: `internal/api/webhooks.go`
- Modify: `internal/api/webhooks_test.go`
- Modify or reduce: `internal/automationevent/emitter.go`
- Modify or add: `internal/automationevent/emitter_test.go`

**Step 1: Write migration-focused tests**

Add assertions that webhook handlers:
- still accept the same source payloads
- still produce equivalent prescribe/report semantics
- still propagate taxonomy fields
- now exercise the shared ingest service path instead of direct server-side entry construction

**Step 2: Run the focused webhook tests to verify the current refactor target**

Run: `go test ./internal/api ./internal/automationevent -run 'TestHandle|TestEmitter' -v`

Expected: tests should either fail or require updates once the shared service is introduced.

**Step 3: Refactor webhook handlers**

Change `webhooks.go` so it:
- parses source payloads
- resolves tenant
- maps to ingest requests
- calls `internal/ingest`

Reduce duplication in `internal/automationevent` where practical. If a full
removal is too invasive for one pass, leave it as a thin compatibility helper,
not as the primary product boundary.

**Step 4: Run the webhook tests to verify they pass**

Run: `go test ./internal/api ./internal/automationevent -run 'TestHandle|TestEmitter' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/webhooks.go internal/api/webhooks_test.go internal/automationevent/emitter.go internal/automationevent/emitter_test.go
git commit -s -m "refactor: route webhook ingest through shared service"
```

### Task 5: Update OpenAPI and public docs

**Files:**
- Modify: `cmd/evidra-api/static/openapi.yaml`
- Modify: `docs/guides/self-hosted-setup.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/system-design/EVIDRA_PROTOCOL_V1.md`
- Modify: `docs/system-design/EVIDRA_ARCHITECTURE_V1.md`

**Step 1: Write or extend documentation checks**

If there are existing doc/openapi verification tests, extend them. Otherwise add
focused assertions in the most relevant API tests to confirm the new routes are
documented.

**Step 2: Update docs**

Document:
- the new ingest routes
- their role relative to `forward`/`batch`
- the taxonomy fields on external ingest
- webhook routes as compatibility wrappers over shared ingest

**Step 3: Verify docs and OpenAPI**

Run:

```bash
git diff --check
go test ./internal/api -v
```

Expected:
- no whitespace/diff errors
- API tests pass

**Step 4: Commit**

```bash
git add cmd/evidra-api/static/openapi.yaml docs/guides/self-hosted-setup.md docs/ARCHITECTURE.md docs/system-design/EVIDRA_PROTOCOL_V1.md docs/system-design/EVIDRA_ARCHITECTURE_V1.md
git commit -s -m "docs: document external lifecycle ingest"
```

### Task 6: Full verification and branch review

**Files:**
- Review: entire branch diff

**Step 1: Run focused verification**

Run:

```bash
go test ./internal/ingest ./internal/api ./internal/automationevent -v
```

Expected: PASS

**Step 2: Run broader repo verification**

Run:

```bash
go test ./...
make lint
```

Expected:
- `go test ./...` may still fail only on the known promptfactory `v1.0.1` missing `SKILL_SMART.tmpl` baseline unless that separate issue is fixed as part of the work
- `make lint` may still fail only on the existing baseline in `cmd/evidra-mcp/main.go`, `pkg/proxy/evidence.go`, and `pkg/proxy/proxy.go`

**Step 3: Review the final diff**

Check for:
- duplicated ingest logic left behind in webhook code
- taxonomy drift
- inconsistent response shapes
- accidental AgentGateway-specific leakage into the generic contract

**Step 4: Commit any final cleanup**

```bash
git add -A
git commit -s -m "chore: finalize external ingest prerequisite"
```
