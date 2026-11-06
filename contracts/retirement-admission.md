# Retirement admission contract

Versioned admission-contract port for the operator repository retirement
trigger at `POST /api/v0/admin/repository-retirements` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Trigger satisfies it. The retire sequence
(mark, fence, purge) persists through explicit core admission; `eshu-api`
never takes a direct trigger handle.

## Version

`RetirementAdmissionContractVersion = "v0"`, with the port defined as:

```go
type RetirementAdmission = Triggerer
```

Both lines are vendored verbatim in
`contracts/upstream/retirement-wire-shapes.txt`, with the surrounding
contract comment and the `Triggerer` port. The frozen method set is
`frozenRetirementAdmission` in
`contracts/upstream/retirement_admission_contract_test.go`
(1 method: `TriggerRetirement`).

## HTTP shape

The operator-facing POST body and the trigger result, vendored
verbatim in `contracts/upstream/retirement-wire-shapes.txt`:

- Request: repository selector resolved through the catalog, retired
  through mark, fence, and purge in the core trigger.
- Response: HTTP 200 with the trigger result for the retired scope.
- Auth: operator authentication; otherwise 403.
- Unconfigured handler (nil catalog or nil/typed-nil trigger): 503.

## Enforcement

`TestRetirementAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the trigger shape fails the frozen test
until the list is deliberately updated; bump
`RetirementAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (tier C-a: HTTP write-through)

The operator retirement trigger POST is the tier C-a native-dispatch
lane: `api/retirement.go` registers the explicit route inside
`RequireAuth` ahead of the generic proxy and forwards to the same core
route with method, path, query, headers (including the original
`Authorization` header), and body relayed; core bytes come back
unmodified and core re-judges every forwarded call as backstop. Dest
holds the AllScopes family authorizer on the route
(`TestRetirementNative_AuthorizerRejectsNonAllScopes` pins the helper);
see Scope honesty below for what is and is not live. Label the family
native-dispatch (not native auth termination) until the dest scoped
resolver lands.

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live route until one lands — the live 403s on this
route are core's, which re-judges the admin (all-scopes) token as
backstop. Core is the sole writer. The deny-shape test pins the
contract for the resolver-landing day, not a live path.

Audit parity: governance audit stays core-side on forwarded requests.
The in-tree retirement handler records denied attempts and the core
trigger records `admin_retirement_action` events, both from inside core
code that still runs on every forwarded request, so write-through keeps
every audit event core-side with no gap. Dest emits none. A family whose
handler emits audit must never be re-implemented dest-side without a
versioned audit-append contract; this lane re-implements nothing.
`scripts/check-parity.sh` gates the lane with the no-core-import proof
(`go list -deps` over the runtime packages must show no `eshu-hq/eshu`
import) alongside the route/403/503 wire-shape excerpt.

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The
fragment moved row-for-row from the pinned core commit `ebaf4dfa` —
`go/internal/query/openapi/paths/status/repository_retirement.go` into
dest `openapi/paths/status/repository_retirement.go`, byte-identical
(no operationIds on this fragment; `scripts/check-parity.sh` gates the
identity). The dest assembly (`openapi/spec.go`, minimal shared block
in `openapi/components.go`: the `ErrorResponse` schema plus the
`BadRequest`/`Forbidden`/`Conflict`/`InternalError`/`ServiceUnavailable`
responses the fragment references) serves `GET /api/v0/openapi.json`
from compiled constants ahead of the generic proxy — no core import,
no core fetch, served even with no core origin configured — inside
`RequireAuth` like every dest `/api/*` route. `TestSpec_*` pin the
family section, lossless assembly, and local-$ref resolution;
`TestOpenAPINative_*` pin the served route, the v1 alias landing, and
the key gate.

Scope honesty: the dest document serves exactly the landed families
(retirement first). Other families' entries are absent until their
lanes port their fragments — this endpoint no longer relays core's full
spec. Core serves the spec public; dest keeps it behind the shared key,
adding no new public surface in this lane.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the POST falls back to the generic byte-relay with the
same wire contract — no core change, no migration, no flag.
