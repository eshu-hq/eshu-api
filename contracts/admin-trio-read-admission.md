# Admin trio read admission contract (Gap 02)

Versioned admission-contract ports for the three Admin GETs the D3
lane left on the generic relay for want of a versioned pin
(`delta-union/completeness`, `reindex/progress`,
`shared-projection/tuning-report`) (Refs `eshu-hq/eshu#9669`, Gap 02;
satellite `eshu-hq/eshu-api#70`). The core minted the pins as
`eshu-hq/eshu#10412` (merge `4a587431fd`); this lane records them
byte-identical and terminates the three GETs natively ahead of relay.
This record covers ONLY the trio: it claims no D1 `ReadStore`
coverage (see `contracts/admin-read-admission.md`), and it leaves
the D7 reindex mutation ports and the suppressions mutation row
untouched.

## Version

`DeltaUnionCompletenessReadAdmissionContractVersion = "v0"`,
`ReindexProgressReadAdmissionContractVersion = "v0"`, and
`SharedProjectionTuningReportContractVersion = "v0"`, with the ports
defined as:

```go
type DeltaUnionCompletenessReadAdmission interface {
	Check(ctx context.Context, scopeIDs []string) (recovery.DeltaUnionCompletenessReport, error)
}
```

```go
type ReindexProgressReadAdmission = ReindexProgressReader
```

```go
const SharedProjectionTuningReportStatus = "not_applicable"
const SharedProjectionTuningReportDetail = "Shared-projection tuning is managed internally by the Go projector pipeline."
```

The version lines, the port shapes, and the frozen literals are
vendored verbatim in `contracts/upstream/admin-trio-read-wire-shapes.txt`
(line-presence at the Gap 02 pin). The frozen method sets are
`frozenDeltaUnionCompletenessReadAdmission` (1 method: `Check`) and
`frozenReindexProgressReadAdmission` (3 methods:
`FullReconcileState`, `GetReindexState`,
`RepositoryReindexRequestedAt`) in the vendored contract tests
`contracts/upstream/delta_union_completeness_read_admission_contract_test.go`
and
`contracts/upstream/reindex_progress_read_admission_contract_test.go`;
the tuning report carries no reader interface (there is no graph
read, no store read, no port to freeze) and its frozen shape is the
exact served body (`status` + `detail` with the fixed values above)
pinned by
`contracts/upstream/shared_projection_tuning_report_read_admission_contract_test.go`.

## Read surface

Each seam is read-only on purpose:

- the delta-union check compares the ACTIVE fact side against
  anchored per-repo graph reads and never repairs;
- the reindex progress reader carries the fleet watermark,
  per-repository watermarks, and per-scope newest-activated-full
  reads, and the endpoint records nothing, claims nothing, and
  completes nothing;
- the tuning report is a static body served from the pinned
  constants.

A dest consumer implementing only these contracts can serve the
reports but cannot acquire a write it did not bargain for through
them. Explicitly OUT: the checker's own fact/graph dependencies, the
fact_work_items store surface, and the D7 reindex mutation ports (the
mutation and the progress view meet only at the watermark rows,
never through a shared pin).

## Native termination (Gap 02 trio lane)

The three Admin GETs terminate natively since the Gap 02 trio lane
(`api/d3trio_wave.go`, family `admin`): explicit gated routes ahead
of the relay reusing the D3 wave gate and family authorizer
(`authorizeD3SharedKey`), byte-parity dispatch/conformance proof vs
the relayed reference (`api/d3trio_parity_test.go`,
`api/d3trio_conformance_test.go`), the deny-shape proof
(`api/d3trio_auth_test.go`), per-route latency
(`api/d3trio_latency_test.go`), and the line-verbatim/byte-identical
spec entries (`openapi/paths/status/delta_union_completeness.go`,
`openapi/paths/status/reindex_progress.go`,
`openapi/paths/status/admin_tuning_report.go`, gated byte-identical
by `scripts/check-parity.sh`). See `contracts/d3-admin-reads.md` for
the termination record and rollback.

## Enforcement

`TestDeltaUnionCompletenessReadAdmissionContractIsVersioned`,
`TestReindexProgressReadAdmissionContractIsVersioned`, and
`TestSharedProjectionTuningReportContractIsVersioned` pin the
versions and the frozen method sets/shapes core-side. A renamed,
added, removed, or re-signed method (or a renamed key or changed
tuning value) fails the frozen test until the list is deliberately
updated; bump the corresponding version alongside (manually — the
bump is convention, the frozen list is the enforcement), and this
repository records the new pin before consuming it.
