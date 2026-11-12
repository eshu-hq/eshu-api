# Retire-unclaimed admission contract

Versioned admission-contract port for the never-claimed workflow
retirement mutation at `POST /api/v0/admin/workflow-retire-unclaimed`
(Refs `eshu-hq/eshu#9669`, `eshu-hq/eshu-api#103`). The API handler keeps
the HTTP shape below and calls this port; the core-owned Postgres
workflow control store satisfies it. `eshu-api` never takes a direct
table handle.

The core pin merged first as `eshu-hq/eshu#10583`: the satellite records
that pin byte-identically (a satellite PR consuming an unminted pin is
invalid) and terminates this route ahead of the relay. No traffic flip,
no deploy, no in-tree removal in this lane.

## Version

`RetireUnclaimedAdmissionContractVersion = "v0"`, with the port defined as:

```go
type RetireUnclaimedAdmission interface {
	RetireUnclaimedWorkflowItems(ctx context.Context, kind, instanceID, note string) ([]RetiredUnclaimedItem, error)
}
```

The const block, the interface, and the compile assertion are vendored
verbatim in `contracts/upstream/retire-unclaimed-wire-shapes.txt`, with
the core mount line, the two sentinel declarations, the request fields,
and the fixed 400/response rows. The frozen method set is
`frozenRetireUnclaimedAdmission` in
`contracts/upstream/retire_unclaimed_admission_contract_test.go`
(1 method: `RetireUnclaimedWorkflowItems`).

## HTTP shape

The operator-facing POST body and the retirement result, vendored
verbatim in `contracts/upstream/retire-unclaimed-wire-shapes.txt`:

- Request: `collector_kind` and `collector_instance_id` (required,
  trimmed, 400 when either is empty) plus `operator_note` (optional,
  trimmed; truncated to 500 runes in storage, never split mid-rune).
- Response: HTTP 200 `{count, items[{work_item_id, scope_id}]}` in
  `work_item_id` order, counting only rows this call transitioned; an
  empty result is a safe no-op.
- Sentinels: `ErrRetireUnclaimedUnknownInstance` and
  `ErrRetireUnclaimedInstanceActive`, wrapped with `"%w: kind/instance"`
  so `errors.Is` still matches while the 404/409 body names the pair.
- Status map: 404 unknown instance, 409 still-configured instance, 400
  validation, 500 unexpected store fault through the fixed
  `retireUnclaimedFailedMessage` (499 on cancel), 503 "admin store not
  configured" on a nil store.
- Eligibility: pending + `attempt_count` 0 + no current claim for a
  known deconfigured (disabled or deactivated) instance, with the gate
  probe plus the in-snapshot deconfiguration recheck, so a reactivation
  between gate and retire retires nothing.
- Concurrency: per-row lock plus predicate recheck gives every row
  exactly one winner versus a concurrent claim.
- Auth: operator authentication; otherwise 403.
- Unconfigured handler (nil store): 503.

## Enforcement

`TestRetireUnclaimedAdmissionContractIsVersioned` pins the version and
the frozen method set; the core-owned `Store` satisfies the port at
compile time (`var _ RetireUnclaimedAdmission = Store(nil)`). A
breaking change to the mutation shape fails the frozen test until the
list is deliberately updated; bump
`RetireUnclaimedAdmissionContractVersion` alongside (manually — the bump
is convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (HTTP write-through)

The never-claimed retirement POST is the writer native-dispatch lane:
`api/retireunclaimed.go` registers the explicit route inside
`RequireAuth` ahead of the generic proxy with the AllScopes family
authorizer and forwards to the same core route with method, path, query,
headers (including the original `Authorization` header), and body
relayed; core bytes come back unmodified and core re-judges every
forwarded call as backstop. Dest holds no workflow control state and
performs no mutation of its own: the gate probe, the retire, and the
audit append all run inside the core handler on every forwarded request.

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live route until one lands — the live 403s on this
route are core's, which re-judges the admin (all-scopes) token as
backstop. Core is the sole writer. The deny-shape test pins the
contract for the resolver-landing day, not a live path.

Dest performs exactly one forward attempt per caller call and never
retries a forwarded mutation: a caller cancel or dispatch-timeout answer
is the permanent local 503 `backend_unavailable` with exactly one
upstream attempt.

Audit parity: governance audit stays core-side on forwarded requests —
the in-tree handler appends `retire_unclaimed_accepted` on success and
the `retire_unclaimed_refused_unknown_instance` /
`retire_unclaimed_refused_instance_active` denies from inside the core
handler that still runs on every forwarded request, so write-through
keeps every audit event core-side with no gap. The 400, 503, and 500
paths emit no governance audit in core, and dest adds none anywhere. A
family whose handler emits audit must never be re-implemented dest-side
without a versioned audit-append contract; this lane re-implements
nothing.

Idempotency posture: the endpoint claims no idempotency key and holds
no ledger row, and runs no detached bounded finalize, because the
mutation is already state-idempotent, not exactly-once: a re-run
touches only rows still pending with no claim, so concurrent duplicate
requests both execute against disjoint row sets and a duplicate submit
returns the residue or a zero count, never a replayed outcome. Dest
synthesizes no key, no ledger, and no outcome: a sequential duplicate
forwards twice and relays both core answers byte-for-byte, which is
no-worse-than-core by construction.

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The fragment
moved row-for-row from the pinned core commit `bc150cc0c6` —
`go/internal/query/openapi/paths/status/workflow_retire_unclaimed.go`
into dest `openapi/paths/status/workflow_retire_unclaimed.go`,
byte-identical (no operationIds on this fragment;
`scripts/check-parity.sh` gates the identity). The dest assembly
(`openapi/spec.go`, no new components: the fragment references only
`ErrorResponse` plus the existing
`BadRequest`/`NotFound`/`Conflict`/`InternalError`/`ServiceUnavailable`
responses) serves `GET /api/v0/openapi.json` from compiled constants
ahead of the generic proxy — no core import, no core fetch, served even
with no core origin configured — inside `RequireAuth` like every dest
`/api/*` route. `TestSpec_*` pin the family section, lossless assembly,
and local-$ref resolution; `TestOpenAPINative_*` pin the served route,
the v1 alias landing, and the key gate.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the POST falls back to the generic byte-relay with the
same wire contract — no core change, no migration, no flag. The
revert-to-relay rehearsal test proves the relay path serves this route
with identical bytes when the native registration is absent.
