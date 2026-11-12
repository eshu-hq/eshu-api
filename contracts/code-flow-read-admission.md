# Code-flow read admission contract

Versioned admission-contract port for the Code-flow 4 reads (`POST
/api/v0/code/flow/cfg-summary`, `/pdg-summary`, `/reaching-def`,
`/taint-path`) (Refs `eshu-hq/eshu#9669`, W6a; satellite
`eshu-hq/eshu-api#98`). Canonical reads persist through explicit
core-owned admission contracts: `eshu-api` keeps the HTTP shape and
calls this versioned port, which the core-owned Postgres code-flow
store satisfies. `eshu-api` never takes a direct store handle.

This is the W6a pin the arbiter W-wave ruling approves
(APPROVED-WITH-CONDITIONS, `eshu-hq/eshu#10552` at core
`e504e328a7`): the Code-flow 4 admit only through the narrow
Code-flow-scoped port, never through a shared or blanket port. The
pin re-declares the single mirrored signature in this Code-flow-scoped
shape instead of embedding or aliasing the staying port, so no other
family can cite it as coverage — and this lane cites no other pin.

## Version

`CodeFlowReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type CodeFlowReadAdmission interface {
	ListCodeFlow(context.Context, codemodel.CodeFlowFilter) (codemodel.CodeFlowReadModel, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion (`var _ CodeFlowReadAdmission =
CodeFlowStore(nil)` against the staying handler-facing port), and the
surrounding contract comment (narrow by construction, explicitly OUT,
rollback) are vendored verbatim in
`contracts/upstream/code-flow-read-wire-shapes.txt`. The frozen method
set is `frozenCodeFlowReadAdmission` in
`contracts/upstream/code_flow_read_admission_contract_test.go`
(1 method: `ListCodeFlow`).

## Read surface

The seam is read-only on purpose: the declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve Code-flow 4 reads but
cannot acquire a write it did not bargain for through it.

## In scope

- `POST /api/v0/code/flow/cfg-summary` admits its bounded parser
  dataflow read through `ListCodeFlow` with Kind `cfg_summary`.
- `POST /api/v0/code/flow/pdg-summary` admits its bounded parser
  dataflow read through `ListCodeFlow` with Kind `pdg_summary`.
- `POST /api/v0/code/flow/reaching-def` admits its bounded parser
  dataflow read through `ListCodeFlow` with Kind `reaching_def`.
- `POST /api/v0/code/flow/taint-path` admits its reducer-owned
  taint-evidence read through `ListCodeFlow` with Kind `taint_path`.

Transport: `NewCodeFlowReadAdmissionWithReadStore(db.Queryer)`
delegates to the existing
`codemodel.NewPostgresCodeFlowStoreWithReadStore` (no parallel wiring);
the guarded-Queryer fail-closed proof plus the full wire-object byte
mirror against the direct constructor covers the satellite path end
to end. Concrete-double-only proof is rejected.

Explicitly OUT of this pin: the grant check
(`codeGrantAccessFilter` plus the scoped repository selector), the
unsupported-language short-circuit, the fixed failure messages
(`"code-flow read failed"`, `"code-flow store is unavailable"`), and
QueryProfile are handler behavior, not port shape; no search, list,
ledger, stored-summary, or snapshot surface exists on this family
(sealed out regardless).

## Relay posture

The Code-flow 4 terminate in this W6a lane (see
`contracts/code-flow-termination.md`): shared-key callers terminate on
the native lane ahead of the generic proxy with byte parity to the
relay, while scoped-token, OIDC-bearer, and browser-session classes
relay explicitly until D8-grade parity. The pin was recorded here
first so the termination lane consumed a minted pin, never an
unminted one.

## Enforcement

`TestCodeFlowReadAdmissionContractIsVersioned` (vendored verbatim)
pins the version and the frozen method set. A renamed, added,
removed, or re-signed method fails the frozen test until the list is
deliberately updated; the version bumps alongside (manually — the
bump is convention, the frozen list is the enforcement).
