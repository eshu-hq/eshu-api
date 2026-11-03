# Suppression admission contract

Versioned admission-contract port for the single bounded in-API
mutation: the operator suppression upsert at
`POST /api/v0/supply-chain/impact/suppressions` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`SuppressionAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SuppressionAdmission = VulnerabilitySuppressionMutationStore
```

Both lines are vendored verbatim in
`contracts/upstream/suppression-wire-shapes.txt`, with the surrounding
contract comment. The frozen method set is `frozenSuppressionAdmission`
in `contracts/upstream/suppression_admission_contract_test.go`
(1 method: `UpsertVulnerabilitySuppression`).

## HTTP shape

The operator-facing POST body and the durable-commit response, vendored
verbatim in `contracts/upstream/suppression-wire-shapes.txt`:

- Request: `suppression_id`, `justification` (one of `not_affected`,
  `accepted_risk`, `false_positive`, `ignored`), RFC 3339 `authored_at`,
  optional `expires_at` (required when justification is `ignored`),
  `reason`, optional `evidence_ref`, and a `scope` carrying at least one
  discoverable identity (`cve_id`, `advisory_id`, `package_id`, `purl`,
  `repository_id`, or `subject_digest`). Source and author are absent:
  the server derives them from the authenticated mutation path.
- Response: `suppression_id`, `generation_id`, `status` (`created` or
  `unchanged`), with HTTP 201 on creation and 200 when unchanged.
- Auth: all-scopes operator authorization; browser sessions are
  rejected with 403. Unknown fields are rejected with 400.

## Enforcement

`TestSuppressionAdmissionContractIsVersioned` pins the version and the
frozen method set; `TestPostgresStoreSatisfiesAdmissionContract` proves
the core-owned Postgres mutation store satisfies the port and fails
closed with the wrapper-level guard error (never a panic) when storage
is absent. A breaking change to the admission shape must bump
`SuppressionAdmissionContractVersion` in the same diff, and this
repository records the new pin before consuming it.

## Native-dispatch pilot (arbiter ruling (a): HTTP write-through)

The suppression upsert is the pilot family for native dispatch:
`api/suppression.go` registers the explicit route inside `RequireAuth`
ahead of the generic proxy and forwards to the same core route with
method, path, query, headers (including the original `Authorization`
header), and body relayed; core bytes come back unmodified and core
re-judges every forwarded call as backstop. Dest holds the AllScopes
authorizer on the route (`TestSuppressionPilot_AuthorizerRejectsNonAllScopes`
pins the helper); see Scope honesty below for what is and is not live.

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live route until one lands — the live 403s on this
route are core's, which re-judges every forwarded call as backstop. The
deny-shape test pins the contract for the resolver-landing day, not a
live path.

Audit parity: governance audit stays core-side on forwarded requests.
The in-tree suppression mutation handler at the provenance pin emits no
audit event, and dest emits none either, so this route has no audit gap.
Families whose in-tree handlers do emit audit stay on the generic relay
until a versioned audit-append contract exists. `scripts/check-parity.sh`
gates the pilot with the no-core-import proof (`go list -deps` over the
runtime packages must show no `eshu-hq/eshu` import) alongside the
403/503 wire-shape excerpt.
