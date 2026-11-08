# Status read admission contract

Versioned admission-contract port for the Status family reads the
status endpoints admit through `Reader` (snapshot reads)
(Refs `eshu-hq/eshu#9669`, D6). Canonical reads persist through
explicit core-owned admission contracts: `eshu-api` keeps the HTTP
shape and calls this versioned port, which the core-owned Postgres
store satisfies. `eshu-api` never takes a direct table handle.

## Version

`StatusReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type StatusReadAdmission = Reader
```

Both lines are vendored verbatim in
`contracts/upstream/status-read-wire-shapes.txt`, with the surrounding
contract comment, the `Reader` port, and the rollback note. The frozen
method set is `frozenStatusReadAdmission` in
`contracts/upstream/status_read_admission_contract_test.go`
(2 methods: `ReadStatusSnapshot`,
`ReadStatusSnapshotFiltered`).

## Read surface

The seam is read-only on purpose: the
aliased interface carries bounded snapshot reads and no mutation
surface, so a dest consumer implementing only this contract can serve
status reads but cannot acquire a write it did not bargain for through
it.

## Native termination (D6)

The 12 snapshot-pure Status GETs (collectors, ingesters, pipelines,
freshness causality, operator control plane, semantic extraction,
collector readiness, each in its legacy and `/status/` spellings)
terminate natively since the D6 lane (`api/d6_wave.go`, family
`status`): explicit gated routes ahead of the relay, the shared-key
family authorizer, byte-parity dispatch/conformance proof vs the
relayed reference, and the line-verbatim spec subsets
(`openapi/paths/status/status_reads.go`,
`openapi/paths/status/collector_readiness.go`,
`openapi/paths/status/operator_control_plane.go`,
`openapi/paths/status/semantic.go`,
`openapi/paths/freshness/causality.go`, gated byte-identical by
`scripts/check-parity.sh`). The 3 phase-2 extra-port legs terminate
under `StatusExtraReadAdmission v0` (see
`contracts/status-extra-read-admission.md`). See
`contracts/d6-status-supplychain.md` for the termination record and
rollback. The 3 remaining extra-port Status aspects stay relay: their
readers carry no versioned pin (the GovernanceAudit Summary, the
NarrationPosture override, index counts via Neo4j).

## Enforcement

`TestStatusReadAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump
`StatusReadAdmissionContractVersion` alongside (manually — the bump
is convention, the frozen list is the enforcement), and this
repository records the new pin before consuming it.
