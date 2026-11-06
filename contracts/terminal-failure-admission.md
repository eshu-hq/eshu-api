# Terminal-failure admission contract

Versioned admission-contract port for the operator terminal-failure
acknowledgement mutation at
`POST /api/v0/admin/collector-failures/acknowledge` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`TerminalFailureAdmissionContractVersion = "v0"`, with the port defined as:

```go
type TerminalFailureAdmission = Acknowledger
```

Both lines are vendored verbatim in
`contracts/upstream/terminal-failure-wire-shapes.txt`, with the surrounding
contract comment and the compile assertion. The frozen method set is
`frozenTerminalFailureAdmission` in
`contracts/upstream/terminal-failure_admission_contract_test.go`
(4 methods: `ResolveTarget`, `Acknowledge`, `ListTerminalFailureAcks`,
`ListTerminalFailures`).

## HTTP shape

The operator-facing POST body and the acknowledgement response, vendored
verbatim in `contracts/upstream/terminal-failure-wire-shapes.txt`:

- Request: `collector_kind`, `instance_id`, `scope_id` (all required),
  `reason` (required, at most 512 characters, must say why the failure is
  expected), `idempotency_key` (required; a refused request leaves its key
  unconsumed for a corrected retry).
- Response: HTTP 200 `acknowledged` with the idempotency key and the
  per-request result; 409 when the key was already used with a different
  target or is already in progress.
- Auth: admin (all-scopes) token; unauthenticated dev mode stays open
  and a scoped token is denied with 403.
- Unconfigured handler (nil store): 503.

## Enforcement

`TestTerminalFailureAdmissionContractIsVersioned` pins the version and the
frozen method set; the core-owned `TerminalFailureAckStore` satisfies the
port at compile time. A breaking change to the admission shape fails the
frozen test until the list is deliberately updated; bump
`TerminalFailureAdmissionContractVersion` alongside (manually — the bump
is convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (tier C-a: HTTP write-through)

The operator acknowledgement POST is the tier C-a native-dispatch lane:
`api/terminalfailure.go` registers the explicit route inside
`RequireAuth` ahead of the generic proxy and forwards to the same core
route with method, path, query, headers (including the original
`Authorization` header), and body relayed; core bytes come back
unmodified and core re-judges every forwarded call as backstop. Dest
holds the AllScopes family authorizer on the route
(`TestTerminalFailureNative_AuthorizerRejectsNonAllScopes` pins the
helper); see Scope honesty below for what is and is not live. The
family's two query POSTs stay on the generic byte-relay. Label the
family native-dispatch (not native auth termination) until the dest
scoped resolver lands.

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live route until one lands — the live 403s on this
route are core's, which re-judges all-scope admin (and denies scoped
tokens) as backstop. Core is the sole writer: the acknowledgement and
the replay-ledger claim/complete pair run inside the core handler on
every forwarded request. The deny-shape test pins the contract for the
resolver-landing day, not a live path.

Audit parity: governance audit stays core-side on forwarded requests.
Like the sign-in-policy mutation and unlike suppression, the in-tree
acknowledgement handler at the provenance pin DOES emit audit —
`EventTypeAdminRecoveryAction` for every decision (the allowed outcome,
every guardrail refusal, and every denied outcome including the
idempotency-conflict answers) — and it emits from inside the core
handler that still runs on every forwarded request, so write-through
keeps every audit event core-side with no gap. Dest emits none. A
family whose handler emits audit must never be re-implemented
dest-side without a versioned audit-append contract; this lane
re-implements nothing. `scripts/check-parity.sh` gates the lane with
the no-core-import proof (`go list -deps` over the runtime packages
must show no `eshu-hq/eshu` import) alongside the family-403/store-503
wire-shape excerpt.

Idempotency proof: the lane's center is the replay-ledger concurrency
proof, not an add-on. Duplicate POSTs collapse to one effect in core
(the ledger claim/complete pair in
`contracts/replay-ledger-admission.md`); dest forwards every delivery
exactly once — no dest retry, no dest dedup — and relays the duplicate
collapse byte-for-byte.
`TestTerminalFailureNative_DuplicateDeliveryCollapsesToOneEffect` pins
two identical deliveries (acknowledged then duplicate with the stored
outcome replayed), one recorded effect, and the completed ledger row;
`TestTerminalFailureNative_KeyReuseAcrossTargetsIs409` pins the 409
half (same key, different target records nothing, original target still
replays); `TestTerminalFailureNative_ParallelDuplicateDeliveries` pins
the collapse under 16-way concurrency (16 hits, 1 effect, 1
acknowledged + 15 duplicate, completed ledger row).

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The
fragment moved row-for-row from the pinned core commit `428790ab` —
`go/internal/query/openapi/paths/status/collector_terminal_failure_acknowledgement.go`
into dest
`openapi/paths/status/collector_terminal_failure_acknowledgement.go`,
byte-identical (the pinned fragment carries no operationId entries, so
there are none to preserve or invent; `scripts/check-parity.sh` gates
the identity). It documents the acknowledge POST plus the family's two
query POSTs; the dest assembly (`openapi/spec.go`, minimal shared
block in `openapi/components.go`: `ErrorResponse` and the
`BadRequest`/`Forbidden`/`Conflict`/`InternalError`/`ServiceUnavailable`
responses the fragment references) serves `GET /api/v0/openapi.json`
from compiled constants ahead of the generic proxy — no core import,
no core fetch, served even with no core origin configured — inside
`RequireAuth` like every dest `/api/*` route. `TestSpec_*` pin the
family section, lossless assembly, and local-$ref resolution;
`TestOpenAPINative_*` pin the served route, the v1 alias landing, and
the key gate.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the POST falls back to the generic byte-relay with the
same wire contract — no core change, no migration, no flag.
