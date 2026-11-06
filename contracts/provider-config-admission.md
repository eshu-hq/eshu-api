# Provider-config admission contracts

Versioned admission-contract port for admin provider-config mutations
(`POST /api/v0/auth/admin/provider-configs`,
`POST /api/v0/auth/admin/provider-configs/{provider_config_id}`,
`.../revert`, `.../enable`, `.../disable`, plus the
`.../test-connection` probe) (Refs `eshu-hq/eshu#8872`). The API handler
keeps the HTTP shape below and calls these ports; the core-owned Postgres
implementation satisfies them. Write-only secrets stay sealed — no
response, log line, or audit event ever carries plaintext or sealed
ciphertext. `eshu-api` never takes a direct table handle.

Two contracts move together: the mutation port for the writes, and the
read port backing the enable-time login-readiness guard (issue
`eshu-hq/eshu#5604`).

## Version

`ProviderConfigMutationAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ProviderConfigMutationAdmission = MutationStore
```

`ProviderConfigReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ProviderConfigReadAdmission = ReadStore
```

Both lines are vendored verbatim in
`contracts/upstream/provider-config-wire-shapes.txt`, with the surrounding
contract comments, the `MutationStore` (5 methods: `CreateProviderConfig`,
`UpdateProviderConfig`, `RevertProviderConfig`, `EnableProviderConfig`,
`DisableProviderConfig`) and `ReadStore` (3 methods:
`GetProviderConfigDetail`, `ListProviderConfigDetails`,
`ListProviderConfigRevisions`) ports, and the route registration. The
frozen method sets are `frozenProviderConfigMutationAdmission` in
`contracts/upstream/mutation_admission_contract_test.go` and
`frozenProviderConfigReadAdmission` in
`contracts/upstream/read_admission_contract_test.go`.

Out of contract by design: `test-connection` is served by the separate,
unversioned `ConnectionTester` port (the handler's enable path still gates
on its mandatory pass, and `EnableProviderConfig` takes the tested
revision id as a compare-and-swap guard), and env-managed providers
reject edit/delete. Neither is pinned here.

## HTTP shape

The operator-facing provider bodies and results, vendored
verbatim in `contracts/upstream/provider-config-wire-shapes.txt`:

- Create/update: exactly one of the OIDC, SAML, or GitHub field groups is
  read, selected by provider kind. Every secret field travels in plaintext
  over TLS in the request and is sealed by the store; no response echoes
  it back.
- Revert: a prior revision id restores non-secret configuration.
- Enable: requires the revision id a prior `test-connection` call
  actually tested; a concurrent write that changed the active revision
  fails the enable rather than activating untested configuration.
- Disable: deactivates the provider for login resolution.
- Every attempt is governance-audited, allowed or denied.
- Unconfigured handler (nil store): 503.

## Enforcement

`TestProviderConfigMutationAdmissionContractIsVersioned` and
`TestProviderConfigReadAdmissionContractIsVersioned` pin the versions and
the frozen method sets. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump the
matching `...ContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (tier C-b: HTTP write-through)

The six admin provider-config mutations are the tier C-b
native-dispatch lane: `api/providerconfig.go` registers the explicit
routes inside `RequireAuth` ahead of the generic proxy —

- `POST /api/v0/auth/admin/provider-configs` (create),
- `POST /api/v0/auth/admin/provider-configs/{provider_config_id}` (update),
- `POST /api/v0/auth/admin/provider-configs/{provider_config_id}/revert`,
- `POST /api/v0/auth/admin/provider-configs/{provider_config_id}/enable`,
- `POST /api/v0/auth/admin/provider-configs/{provider_config_id}/disable`,
- `POST /api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection`

— each with the AllScopes family authorizer (plus the trailing-slash
twin on the same handler, so the family is never split across dispatch
paths), and forwards to the same core route with method, path, query,
headers (including the original `Authorization` header), and body
relayed; core bytes come back unmodified and core re-judges every
forwarded call as backstop. The create and update POSTs share path keys
with the family list/detail GETs, but Go matches method and path
together, so the POSTs ride the native route while those GETs stay on
the generic relay. The family's reads stay on the generic relay.
`TestProviderConfigNative_AuthorizerRejectsNonAllScopes` pins the
helper; see Scope honesty below for what is and is not live. Label the
family native-dispatch (not native auth termination) until the dest
scoped resolver lands. `test-connection` rides the same write-through
(the separate, unversioned `ConnectionTester` port stays core-side;
dest adds no probe of its own).

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live routes until one lands — the live 403s on these
routes are core's, which re-judges all-scope admin (`all-scope admin
authentication is required`), the admin tenant scope (`admin tenant
scope is required`), and the
`identity_admin.provider_config_*` permission catalog as backstop. Core
is the sole writer. The deny-shape test pins the contract for the
resolver-landing day, not a live path.

Idempotent-retry parity: every mutation is idempotent under retry at
the active-row conflict keys — a revert to the already-active revision
(`changed=false`), the not-found 404s (`provider config not found`,
`revision not found`), and the enable compare-and-swap guard,
whose stale-revision answer 409 (`the provider config's active revision
changed since it was tested; run test-connection again and retry
enable`) is the conflict case, while a duplicate create 409 (`a
provider config already exists for this tenant, kind, and identity
key`) refuses the second identical create. Dest never retries a
forwarded mutation (the 409 tests pin exactly one upstream hit), so a
client retry crosses core exactly once per attempt and observes core's
idempotent outcome.

Audit parity: governance audit stays core-side on forwarded requests.
Like sign-in-policy and admin-identity, the in-tree provider-config
mutation handlers at the provenance pin DO emit audit —
`EventTypeIDPConfigChange` for the allowed outcome and every denied
outcome including the idempotency-conflict answers — and they emit from
inside the core handlers that still run on every forwarded request, so
write-through keeps every audit event core-side with no gap. Dest emits
none. A family whose handlers emit audit must never be re-implemented
dest-side without a versioned audit-append contract; this lane
re-implements nothing. `scripts/check-parity.sh` gates the lane with
the no-core-import proof (`go list -deps` over the runtime packages
must show no `eshu-hq/eshu` import) alongside the family route, 403/503,
and 409 wire-shape excerpts.

OpenAPI: native since this lane (satellite-design Decision 5(c),
overriding the `docs/parity.md` deferral for this family). The
mutation-only fragment moves as a line-verbatim subset of the pinned
core commit `99381691b1ac49254e813e5a013cfb654d432491` —
`go/internal/query/openapi/paths/auth/admin_provider_configs.go` into
dest `openapi/paths/auth/admin_provider_config_mutations.go`, carrying
only the four mutation path keys that are unique to the mutation
surface (revert, enable, disable, test-connection; operationIds
`revertAdminProviderConfig`, `enableAdminProviderConfig`,
`disableAdminProviderConfig`, `testAdminProviderConfigConnection`
preserved; `scripts/check-parity.sh` gates the line-verbatim
provenance plus the core file's sha256). Core ships reads and mutations
in one file, so no whole-file identity port can serve mutations only —
the subset keeps the line-verbatim property instead, and the served
spec stays duplicate-key free. Only the routes whose path keys are
unique to the mutation surface move: the create and update POSTs share
path keys with the family GETs, so per the core OpenAPI convention they
live in the reads fragment, which this lane does not port — reads stay
relay (the reads-split follow-up ports them alongside the GETs). The
fragment needs one new shared-block schema entry,
`AdminProviderConfigWriteResult`, ported from
`components_provider_configs.go` at the same pin. The dest assembly
(`openapi/spec.go`, extended minimal shared block in
`openapi/components.go`) serves `GET /api/v0/openapi.json` from
compiled constants ahead of the generic proxy — no core import, no core
fetch, served even with no core origin configured — inside `RequireAuth`
like every dest `/api/*` route. `TestSpec_*` pin the family section
(with the reads-split deferral recorded), lossless assembly, and
local-$ref resolution; `TestOpenAPINative_*` pin the served route, the
v1 alias landing, and the key gate.

Scope honesty: the dest document serves exactly the landed families
(sign-in-policy first, this family fifth). Other families' entries are
absent until their lanes port their fragments — this endpoint no longer
relays core's full spec. Core serves the spec public; dest keeps it
behind the shared key, adding no new public surface in this lane.

Rollback: additive. Revert this lane's dest commit (or re-pin the prior
digest) and the six mutations fall back to the generic byte-relay with
the same wire contract — no core change, no migration, no flag.
