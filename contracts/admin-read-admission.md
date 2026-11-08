# Admin read admission contract

Versioned admission-contract port for the operator admin reads the
admin endpoints admit through `ReadStore` (work items, dead letters,
input-invalid facts, poisoned links, replay events, decisions, and
decision evidence) (Refs `eshu-hq/eshu#9669`, D1). Canonical reads
persist through explicit core-owned admission contracts: `eshu-api`
keeps the HTTP shape and calls this versioned port, which the
core-owned Postgres store satisfies. `eshu-api` never takes a direct
table handle.

## Version

`AdminReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type AdminReadAdmission = ReadStore
```

Both lines are vendored verbatim in
`contracts/upstream/admin-read-wire-shapes.txt`, with the surrounding
contract comment, the `ReadStore` port, and the compile assertion. The
frozen method set is `frozenAdminReadAdmission` in
`contracts/upstream/admin_read_admission_contract_test.go`
(7 methods: `ListWorkItems`, `ListDeadLetterWorkItems`,
`ListReducerInputInvalidFacts`, `ListChangedSincePoisonedLinks`,
`ListReplayEvents`, `ListDecisions`, `ListEvidence`).

## Read surface

The seam is read-only on purpose: the
aliased interface carries bounded list methods and no mutation
surface, so a dest consumer implementing only this contract can serve
admin reads but cannot acquire a write it did not bargain for through
it.

## Native termination (D3)

The six query POSTs (`admin/work-items/query`, `decisions/query`,
`dead-letters/query`, `input-invalid-facts/query`,
`changed-since/poisoned-links/query`, `replay-events/query`)
terminate natively since the D3 lane (`api/d3_wave.go`, family
`admin`): explicit gated routes ahead of the relay, the shared-key
family authorizer, byte-parity dispatch/conformance proof vs the
relayed reference, and the line-verbatim spec subset
(`openapi/paths/status/admin_reads.go`, gated byte-identical by
`scripts/check-parity.sh`). See `contracts/d3-admin-reads.md` for the
termination record and rollback. The 3 Gap 02 Admin GETs
(`delta-union/completeness`, `reindex/progress`,
`shared-projection/tuning-report`) terminate separately since the
Gap 02 trio lane (`api/d3trio_wave.go`) under the trio pins in
`contracts/admin-trio-read-admission.md` — this record claims no
`ReadStore` coverage for them.

## Enforcement

`TestAdminReadAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method fails
the frozen test until the list is deliberately updated; bump
`AdminReadAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
