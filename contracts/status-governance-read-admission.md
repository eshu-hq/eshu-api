# Status governance read admission contract

Versioned admission-contract port for the Status governance
audit-summary leg (`GET /api/v0/status/governance`) (Refs
`eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#100`, core
`eshu-hq/eshu#10538`). Canonical reads persist through explicit
core-owned admission contracts: `eshu-api` keeps the HTTP shape and
calls this versioned port, which the core-owned Postgres audit store
satisfies. `eshu-api` never takes a direct store handle.

This is the deliberate pin the W8a lane consumes: the pin
re-declares exactly the Summary read in this Status-scoped shape
instead of embedding the `GovernanceAuditSummaryReader` port or
aliasing the audit store, so no other family can cite it as
coverage. It is not combined with the hosted-readiness pin: each
admits only its own legs.

## Version

`StatusGovernanceReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type StatusGovernanceReadAdmission interface {
	Summary(context.Context) (governanceaudit.Summary, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/status-governance-read-wire-shapes.txt`. The
frozen method set is `frozenStatusGovernanceReadAdmission` in
`contracts/upstream/status_governance_read_admission_contract_test.go`
(1 method: `Summary`). The excerpt stops at the mirror assertion:
the `NewStatusGovernanceReadAdmissionWithQueryer` satisfier is
minted core-side but unconsumed in W8a — the lane terminates through
the forward transport only (see `contracts/w8a-termination.md`).

## Read surface

The seam is read-only on purpose: the single declared method is a
bounded aggregate read and the port carries no mutation or
detailed-event surface, so a dest consumer implementing only this
contract can serve the audit-summary leg but cannot acquire an audit
write or a detailed event read through it.

## In scope

- `GET /api/v0/status/governance` admits its aggregate-only audit
  counts through `Summary` (mirroring
  `GovernanceAuditSummaryReader`); the snapshot leg rides the
  already-recorded `StatusReadAdmission v0` (`status.Reader`), and
  `GovernanceStatusConfig` (env-derived readback settings) stays
  config, not a port.

Explicitly OUT of this pin, recorded not silently assumed (each
keeps its own standing until a deliberate pin covers it):

- The D1 `GovernanceAuditAdmission` pin (the single-method Append
  surface) stays separate: neither pin widens to cover the other.
- The detailed List surface (`GovernanceAuditReader.List`) and the
  tenant-scoped summary (`SummaryForTenant`) stay unpinned here.
- The grant checks and the Summary error shape are handler behavior,
  not port shape: there is no method set to freeze.
- No suppression leg exists on this route; the suppressions mutation
  row stays the Decision 2 survivor, never a read termination here.

## Relay posture

The governance leg terminates in the W8a lane (see
`contracts/w8a-termination.md`): shared-key callers terminate on the
native lane ahead of the generic proxy with byte parity to the
relay, while scoped-token, OIDC-bearer, and browser-session classes
relay explicitly until D8-grade parity. The pin is recorded here
first so the termination lane consumes a minted pin, never an
unminted one.

## Enforcement

`TestStatusGovernanceReadAdmissionContractIsVersioned` pins the
version and the frozen method set. A renamed, added, removed, or
re-signed method fails the frozen test until the list is deliberately
updated; bump `StatusGovernanceReadAdmissionContractVersion`
alongside (manually — the bump is convention, the frozen list is the
enforcement), and this repository records the new pin before
consuming it.
