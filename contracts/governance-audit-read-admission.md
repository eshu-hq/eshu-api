# Governance audit read admission contract

Versioned admission-contract port for the governance-audit reads
backing the tenant-admin audit links (bounded event list plus
shared-operator and tenant-scoped aggregate summaries) (Refs
`eshu-hq/eshu#9669`, D1). Canonical reads persist through explicit
core-owned admission contracts: `eshu-api` keeps the HTTP shape and
calls this versioned port, which the core-owned Postgres store
satisfies. `eshu-api` never takes a direct table handle.

This is the read half the Leaf A `GovernanceAuditAdmission` seam
deliberately omits: that seam stays append-only (a single `Append`
method, no `Summary`, no `List`), while this pin carries the bounded
`List` and aggregate `Summary` surface the audit reads need.

## Version

`GovernanceAuditReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type GovernanceAuditReadAdmission = GovernanceAuditReader
```

Both lines are vendored verbatim in
`contracts/upstream/admin-identity-read-wire-shapes.txt`, with the
surrounding contract comment and the `GovernanceAuditReader` port. The
frozen method set is `frozenGovernanceAuditReadAdmission` in
`contracts/upstream/governance_audit_read_admission_contract_test.go`
(3 methods: `ListAuditEvents`, `SummarizeAuditEvents`,
`SummarizeAuditEventsForTenant`).

## Read surface

The reads stay on the generic relay until the D3 lane (tenant-admin
audit links) and the D6 lane (Status + SupplyChain reads, which need
the D1 audit-read pin) land; no native route moves in this record. A
dest consumer implementing only this contract can serve audit reads
but cannot append an event or acquire a mutation through it.

## Enforcement

`TestGovernanceAuditReadAdmissionContractIsVersioned` pins the version
and the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `GovernanceAuditReadAdmissionContractVersion` alongside (manually
— the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
