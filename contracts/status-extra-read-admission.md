# Status extra-port read admission contract

Versioned admission-contract port for the extra-port Status family
legs the phase-2 D6 wave terminates (Refs `eshu-hq/eshu#9669`, gap
03 phase 2). Canonical reads persist through explicit core-owned
admission contracts: `eshu-api` keeps the HTTP shape and calls this
versioned port, which the core-owned Postgres stores satisfy.
`eshu-api` never takes a direct table handle.

## Version

`StatusExtraReadAdmissionContractVersion = "v0"`, with the port
defined as:

```go
type StatusExtraReadAdmission interface {
	LiveActivityReader
	status.IndexRetirementReader
}
```

Both lines are vendored verbatim in
`contracts/upstream/status-extra-read-wire-shapes.txt`, with the
surrounding contract comment, the in-scope leg list, and the rollback
note. The frozen method set is `frozenStatusExtraReadAdmission` in
`contracts/upstream/status_extra_read_admission_contract_test.go`
(2 methods: `ReadLiveActivity`,
`ReadRetirementSummary`).

## Read surface

The seam is read-only on purpose: every embedded port carries
bounded snapshot/summary reads and no mutation surface, so a dest
consumer implementing only this contract can serve these legs but
cannot acquire a write it did not bargain for through it.

## In scope

- `GET /api/v0/status/operations` admits its live-activity leg
  through `LiveActivityReader.ReadLiveActivity`, the bounded,
  separately-queried in-flight work-item read model.
- `GET /api/v0/index-status` and `GET /api/v0/status/index` admit
  their retirements leg through
  `status.IndexRetirementReader.ReadRetirementSummary`, the tombstone
  aggregate behind the `repository_retirements` block.

## Native termination (D6 phase 2)

The 3 extra-port Status legs terminate natively since the phase-2 D6
lane (`api/d6_wave.go`, family `status`): explicit gated routes ahead
of the relay, the shared-key family authorizer, byte-parity
dispatch/conformance proof vs the relayed reference, and the
line-verbatim spec subsets
(`openapi/paths/status/status_extra_reads.go`, whole-file
`openapi/paths/status/operations.go`, gated byte-identical by
`scripts/check-parity.sh`). See `contracts/d6-status-supplychain.md`
for the termination record and rollback. The snapshot leg on every one
of these routes stays covered by `StatusReadAdmission v0`, not widened
here. The remaining extra-port Status aspects stay relay: governance
(the `GovernanceAuditSummaryReader.Summary` port has no pin, carried
relay-disclosed per the row-7 Path B ruling), answer-narration (the
`NarrationPosture` handler field is not a port), hosted-readiness
(shared graph port plus workload leftovers), and the Neo4j-count legs
(shared `GraphQuery` port — pinning it would be a blanket pin).

## Enforcement

`TestStatusExtraReadAdmissionContractIsVersioned` pins the version and
the frozen method set. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump
`StatusExtraReadAdmissionContractVersion` alongside (manually — the
bump is convention, the frozen list is the enforcement), and this
repository records the new pin before consuming it.
