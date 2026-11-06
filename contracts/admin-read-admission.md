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

The reads stay on the generic relay until the D3 lane lands; no native
route moves in this record. The seam is read-only on purpose: the
aliased interface carries bounded list methods and no mutation
surface, so a dest consumer implementing only this contract can serve
admin reads but cannot acquire a write it did not bargain for through
it.

## Enforcement

`TestAdminReadAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method fails
the frozen test until the list is deliberately updated; bump
`AdminReadAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
