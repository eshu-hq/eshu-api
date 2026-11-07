# Admin identity admission contract

Versioned admission-contract port for tenant-scoped admin identity
mutations (invitation revoke, role grant/revoke, IdP group mapping
create/delete under `/api/v0/auth/admin/...`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`AdminIdentityAdmissionContractVersion = "v0"`, with the port defined as:

```go
type AdminIdentityAdmission = MutationStore
```

Both lines are vendored verbatim in
`contracts/upstream/admin-identity-wire-shapes.txt`, with the surrounding
contract comment and the `MutationStore` port. The frozen method set is
`frozenAdminIdentityAdmission` in
`contracts/upstream/admin_identity_admission_contract_test.go`
(5 methods: `RevokeAdminInvitation`, `GrantAdminRoleAssignment`,
`RevokeAdminRoleAssignment`, `CreateAdminIdPGroupMapping`,
`DeleteAdminIdPGroupMapping`).

## Read pin (D1, unwired)

`AdminIdentityReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type AdminIdentityReadAdmission = ReadStore
```

Both lines are vendored verbatim in
`contracts/upstream/admin-identity-read-wire-shapes.txt`, with the
surrounding contract comment and the `ReadStore` port. The frozen
method set is `frozenAdminIdentityReadAdmission` in
`contracts/upstream/admin_identity_read_admission_contract_test.go`
(6 methods: `ListAdminInvitations`, `ListAdminRoleAssignments`,
`ListAdminRoles`, `ListAdminIdPProviders`,
`ListAdminIdPGroupMappings`, `ListAdminAPITokens`).

Every method is scoped strictly to
the caller's tenant (and workspace where applicable) and none returns
a secret, hash, invite code, credential handle, or external group
hash.

## Native termination (D3, reads)

The six tenant-scoped identity GETs (`auth/admin/api-tokens`,
`idp-group-mappings`, `idp-providers`, `role-assignments`, `roles`,
and `auth/local/invitations`) terminate natively since the D3 lane
(`api/d3_wave.go`, family `admin_identity`): explicit gated routes
ahead of the relay, the shared-key family authorizer, byte-parity
dispatch/conformance proof vs the relayed reference, and the served
spec entries — the five `auth/admin/*` GETs ride the already-ported
whole-file `openapi/paths/auth/admin_reads.go`, while the invitations
GET rides the new line-verbatim subset
(`openapi/paths/auth/admin_invitations.go`, gated byte-identical by
`scripts/check-parity.sh`; get-only, because the create POST sharing
its key stays relay until D8). See `contracts/d3-admin-reads.md` for
the termination record and rollback. The five tier C-b mutation routes
above stay on the HTTP write-through lane; reads and mutations share
nothing but path keys.

## HTTP shape

The operator-facing mutation bodies and results, vendored
verbatim in `contracts/upstream/admin-identity-wire-shapes.txt`:

- Requests: invitation revoke, role grant/revoke, IdP mapping
  create/delete — every method scoped strictly to the caller's tenant
  (and workspace where applicable). Writes are idempotent under retry
  via active-row conflict keys.
- Response: HTTP 200 with the per-method result, including the opaque
  `mapping_ref` on mapping create.
- Auth: all-scope admin authentication with an admin tenant scope;
  otherwise 403.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestAdminIdentityAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the mutation shape fails the frozen test
until the list is deliberately updated; bump
`AdminIdentityAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
`TestAdminIdentityReadAdmissionContractIsVersioned` pins the D1 read
version and frozen set the same way.

## Native dispatch (tier C-b: HTTP write-through)

The five admin identity mutations are the tier C-b native-dispatch
lane: `api/adminidentity.go` registers the explicit routes inside
`RequireAuth` ahead of the generic proxy —

- `POST /api/v0/auth/local/invitations/{invite_id}/revoke`,
- `POST /api/v0/auth/admin/role-assignments{,/revoke}`,
- `POST /api/v0/auth/admin/idp-group-mappings`,
- `DELETE /api/v0/auth/admin/idp-group-mappings/{mapping_ref}`

— each with the AllScopes family authorizer (plus the trailing-slash
twin on the same handler, so the family is never split across dispatch
paths), and forwards to the same core route with method, path, query,
headers (including the original `Authorization` header), and body
relayed; core bytes come back unmodified and core re-judges every
forwarded call as backstop. The family's reads stay on the generic
relay. `TestAdminIdentityNative_AuthorizerRejectsNonAllScopes` pins the
helper; see Scope honesty below for what is and is not live. Label the
family native-dispatch (not native auth termination) until the dest
scoped resolver lands.

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live routes until one lands — the live 403s on these
routes are core's, which re-judges all-scope admin, the admin tenant
scope (`admin tenant scope is required`), the workspace match
(`workspace_id must match the caller's workspace`), and the
`identity_admin` / `roles_grants` permission catalog as backstop. Core
is the sole writer. The deny-shape test pins the contract for the
resolver-landing day, not a live path.

Idempotent-retry parity: every mutation is idempotent under retry at the
active-row conflict keys — a revoked/accepted/expired invitation revoke,
a reactivated grant or mapping (`changed=false`, `created=false`), and
a missing mapping delete (`deleted=false`) are safe no-ops, while a
stale 32-hex MD5 `mapping_ref` answers 409 (`mapping_ref is stale; list
group mappings again and use the current ref`). Dest never retries a
forwarded mutation (the 409 test pins exactly one upstream hit), so a
client retry crosses core exactly once per attempt and observes core's
idempotent outcome.

Audit parity: governance audit stays core-side on forwarded requests.
Like sign-in-policy, the in-tree identity mutation handlers at the
provenance pin DO emit audit — `EventTypeRoleGrantChange` for
invitation and role decisions, `EventTypeIDPConfigChange` for mapping
decisions, for the allowed outcome and every denied outcome including
the idempotency-conflict answers — and they emit from inside the core
handlers that still run on every forwarded request, so write-through
keeps every audit event core-side with no gap. Dest emits none. A
family whose handlers emit audit must never be re-implemented dest-side
without a versioned audit-append contract; this lane re-implements
nothing. `scripts/check-parity.sh` gates the lane with the
no-core-import proof (`go list -deps` over the runtime packages must
show no `eshu-hq/eshu` import) alongside the family route, 403/503, and
409 wire-shape excerpts.

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The
mutation-only fragment moved row-for-row from the pinned core commit
`ebaf4dfa` —
`go/internal/query/openapi/paths/auth/admin_mutations.go` into dest
`openapi/paths/auth/admin_mutations.go`, byte-identical (operationIds
`revokeAdminInvitation`, `revokeAdminRoleAssignment`,
`deleteAdminIdPGroupMapping` preserved; `scripts/check-parity.sh` gates
the identity). Only the routes whose path keys are unique to the
mutation surface move: the grant and create POSTs share path keys with
the family GETs, so per the core OpenAPI convention they live in the
reads fragment, which this lane does not port — reads stay relay. The
fragment needs one new shared-block entry, `NotFound`, ported from
`components_responses.go` at the same pin. The dest assembly
(`openapi/spec.go`, extended minimal shared block in
`openapi/components.json`) serves `GET /api/v0/openapi.json` from
compiled constants ahead of the generic proxy — no core import, no core
fetch, served even with no core origin configured — inside `RequireAuth`
like every dest `/api/*` route. `TestSpec_*` pin the family section
(with the reads-fragment split recorded), lossless assembly, and
local-$ref resolution; `TestOpenAPINative_*` pin the served route, the
v1 alias landing, and the key gate.

Scope honesty: the dest document serves exactly the landed families
(sign-in-policy first, this family fourth). Other families' entries are
absent until their lanes port their fragments — this endpoint no longer
relays core's full spec. Core serves the spec public; dest keeps it
behind the shared key, adding no new public surface in this lane.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the five mutations fall back to the generic byte-relay with
the same wire contract — no core change, no migration, no flag.
