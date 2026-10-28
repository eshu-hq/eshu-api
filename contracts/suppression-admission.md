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
