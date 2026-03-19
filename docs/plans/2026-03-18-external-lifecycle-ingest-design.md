# External Lifecycle Ingest Design

- Status: Proposed
- Date: 2026-03-18
- Owner: Codex
- Canonical for: server-side external adapter ingest prerequisite

---

## Goal

Add one clean product-side ingest model for external adapters so Evidra can
accept observed or translated lifecycle evidence without requiring adapters to
construct and sign raw evidence entries themselves.

This is a prerequisite for the future AgentGateway bridge and for other
adapters such as Argo CD. The product surface should be Evidra itself, not a
demo-only side path.

---

## Problem

The current server-side ingestion model is split across three unrelated paths:

1. `/v1/evidence/forward` and `/v1/evidence/batch` accept already-built raw
   evidence entries.
2. `/v1/evidence/findings` accepts raw findings payloads.
3. `/v1/hooks/argocd` and `/v1/hooks/generic` bypass the generic ingestion
   routes and synthesize prescribe/report entries directly in the webhook
   handlers via `internal/automationevent`.

That creates two problems:

- external lifecycle evidence has no generic product-side ingest protocol
- webhook/controller ingestion is a special-case implementation path rather than
  a reusable adapter model

There is also internal duplication in the server-built lifecycle code:

- `MappedPrescribeInput`
- `MappedReportInput`
- `ExplicitReportInput`

These are all variants of the same underlying concern: server-side creation of
typed prescribe/report entries from external systems.

---

## Chosen Approach

Implement a generic external lifecycle ingest service under `internal/ingest`
and route all server-built external prescribe/report creation through it.

This change adds two new authenticated API routes:

- `POST /v1/evidence/ingest/prescribe`
- `POST /v1/evidence/ingest/report`

These routes are for external adapters and mapped or observed sources.
Evidra owns validation, taxonomy normalization, risk assembly, idempotency,
signing, and final evidence entry construction on this path.

Existing raw forwarding routes remain unchanged:

- `POST /v1/evidence/forward`
- `POST /v1/evidence/batch`
- `POST /v1/evidence/findings`

Existing webhook routes remain public compatibility surfaces, but they stop
being special architecture paths. They become thin translators that map source
payloads into the new `internal/ingest` service immediately.

---

## Why This Approach

Three options were considered:

1. Add a generic ingest API beside the current webhook logic and migrate later.
2. Add a generic ingest API and migrate the webhook logic now.
3. Replace all ingest modes with one umbrella endpoint.

Option 2 is the right choice.

Option 1 leaves the duplication in place and guarantees another follow-up
cleanup. Option 3 mixes native raw forwarding with server-built lifecycle
ingest and makes the API harder to reason about.

The new model keeps the product boundary clean:

- raw entries stay raw
- findings stay findings
- external lifecycle systems use one reusable ingest protocol

---

## External Ingest Contract

The new ingest API should be explicit, typed, and still flexible enough for
adapters that already have shaped prescribe/report payload fragments.

### Shared Envelope

Both ingest routes share the same envelope concepts:

- `contract_version`
- `claim`
  - `source`
  - `key`
  - `payload` optional raw upstream blob for idempotency/audit
- `actor`
- `session_id`
- `operation_id`
- `trace_id`
- `span_id`
- `parent_span_id`
- `scope_dimensions`
- `flavor`
- `evidence.kind`
- `source.system`

This aligns with the protocol cleanup already merged:

- `payload.flavor` = execution shape
- `payload.evidence.kind` = how Evidra obtained the evidence
- `payload.source.system` = which upstream or adapter produced it

### `POST /v1/evidence/ingest/prescribe`

The prescribe route should accept one of:

- `canonical_action`
- lightweight smart-style target context that Evidra can normalize into a
  canonical action
- optional `payload_override` for adapters that already have an appropriate
  prescribe payload body

Evidra still constructs and signs the final evidence entry. The adapter does
not submit a raw evidence entry.

Response shape:

- `prescription_id`
- `entry_id`
- `effective_risk`
- `duplicate`

### `POST /v1/evidence/ingest/report`

Required:

- `prescription_id`
- `verdict`

Optional:

- `exit_code`
- `decision_context`
- `external_refs`
- `artifact_digest`
- `payload_override`

Evidra resolves the referenced prescription and builds the report through one
canonical path.

### Payload Override Rule

Payload override is allowed only for the typed prescribe/report payload body.

Adapters must not be allowed to bypass:

- actor normalization
- taxonomy fields
- correlation fields
- idempotency claim handling
- signing
- evidence entry construction

This preserves a stable product-side contract while still allowing bridge code
to reuse existing shaped payload fragments when useful.

---

## Internal Architecture

The new package should live under `internal/ingest`.

Recommended responsibilities:

- request contract types
- contract validation
- claim/idempotency handling
- taxonomy normalization
- canonical action selection or normalization
- lifecycle entry construction
- persistence through existing store abstractions

Suggested package shape:

- `internal/ingest/contracts.go`
- `internal/ingest/service.go`
- `internal/ingest/claims.go`
- `internal/ingest/errors.go`

The service should depend on a small store interface that supports:

- `LastHash`
- `SaveRaw`
- `GetEntry`
- `ClaimWebhookEvent`
- `ReleaseWebhookEvent`

This keeps the existing persistence model and avoids inventing a new storage
layer.

---

## Refactor Target

The current split in `internal/automationevent/emitter.go` should be reduced.

Today there are separate models for:

- mapped prescribe
- mapped report
- explicit report

That split is implementation-shaped rather than domain-shaped.

After this change there should be:

- one generic server-side prescribe path
- one generic server-side report path
- one shared claim/idempotency model

`internal/automationevent` may remain temporarily as a thin helper layer if it
reduces patch size, but it should no longer own the product-side contract.
`internal/ingest` should become the authoritative server-side external adapter
boundary.

---

## Route Migration

### New Routes

Add:

- `POST /v1/evidence/ingest/prescribe`
- `POST /v1/evidence/ingest/report`

These use normal tenant auth middleware, just like `forward` and `batch`.

### Existing Webhook Routes

Keep:

- `POST /v1/hooks/argocd`
- `POST /v1/hooks/generic`

But change them into thin translators:

- parse source payload
- resolve tenant from `X-Evidra-API-Key`
- map source payload into ingest request
- call `internal/ingest`

The webhook routes should no longer build evidence entries themselves.

### Existing Raw Routes

Keep unchanged:

- `forward`
- `batch`
- `findings`

These routes represent different integration shapes and should remain distinct.

---

## Auth And Idempotency

### Auth

New ingest routes use normal tenant auth middleware.

Webhook routes keep the existing model:

- route bearer secret
- tenant API key header

Then call the same ingest service underneath after tenant resolution.

### Idempotency

The claim model should be shared between ingest routes and webhook wrappers.

If a claim is duplicated:

- return `duplicate: true`
- do not create a second entry

This keeps webhook retry safety and makes generic adapters safe to retry.

---

## Validation And Error Handling

### Validation Rules

Prescribe:

- require contract version
- require actor and taxonomy fields
- require enough action context to build a prescribe entry
- reject empty or contradictory payload override/input combinations

Report:

- require contract version
- require `prescription_id`
- require valid verdict
- require `decision_context` for `declined`
- reject `exit_code` for `declined`
- require `exit_code` for non-declined verdicts

### Error Model

Handlers should return stable client-facing errors using the same existing
pattern as the rest of the API:

- `400` invalid payload
- `401` unauthorized
- `409` duplicate or conflicting claim when appropriate
- `503` signing unavailable for server-built ingest

The service itself should expose typed errors so the API layer does not rely on
string matching.

---

## Testing Strategy

Required coverage:

1. contract validation tests for prescribe and report ingest requests
2. service tests for:
   - prescribe entry creation
   - report entry creation
   - duplicate claim handling
   - referenced prescription resolution
   - taxonomy propagation
3. API handler tests for the two new ingest routes
4. migration tests proving:
   - `/v1/hooks/generic` still emits the same lifecycle semantics
   - `/v1/hooks/argocd` still emits the same lifecycle semantics
5. OpenAPI and docs verification for the new routes

---

## Non-Goals

This prerequisite does not include:

- AgentGateway-specific bridge logic
- a new public bridge repo
- Kubernetes outcome correlation
- a plugin runtime for arbitrary adapter code
- removal of the existing webhook routes

Those come later. This change is only about cleaning up and productizing the
server-side external lifecycle ingest boundary.

---

## Expected Result

After this change, Evidra itself will expose one coherent external adapter
ingress model:

- raw native entry forwarding
- findings ingestion
- generic lifecycle ingest for observed or translated external systems

That is the right product prerequisite for a future `evidra-agentgateway-bridge`
repo and for other adapters such as Argo CD.
