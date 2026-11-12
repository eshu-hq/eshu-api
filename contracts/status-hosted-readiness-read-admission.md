# Status hosted-readiness read admission contract

Versioned admission-contract port for the Status hosted-readiness
graph legs (`GET /api/v0/status/hosted-readiness`) (Refs
`eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#100`, core
`eshu-hq/eshu#10538`). Canonical reads persist through explicit
core-owned admission contracts: `eshu-api` keeps the HTTP shape and
calls this versioned port, which the core-owned graph reader
satisfies. `eshu-api` never takes a direct graph handle.

This is the deliberate pin the W8a lane consumes: the pin
re-declares exactly the two graph reads in this Status-scoped shape
instead of embedding the shared `GraphQuery` port, so no other
family can cite it as coverage — pinning the shared port here would
grant every family that reads through it, a blanket pin the arbiter
ruling forbids. It is not combined with the governance Summary pin:
each admits only its own legs.

## Version

`StatusHostedReadinessReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type StatusHostedReadinessReadAdmission interface {
	Run(context.Context, string, map[string]any) ([]map[string]any, error)
	RunSingle(context.Context, string, map[string]any) (map[string]any, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/status-hosted-readiness-read-wire-shapes.txt`.
The frozen method set is
`frozenStatusHostedReadinessReadAdmission` in
`contracts/upstream/status_hosted_readiness_read_admission_contract_test.go`
(2 methods: `Run`, `RunSingle`). W8a consumes the pin through the
forward transport only (see `contracts/w8a-termination.md`).

## Read surface

The seam is read-only on purpose: both declared methods are bounded
graph reads and the port carries no mutation, session, or
transaction surface, so a dest consumer implementing only this
contract can serve the hosted-readiness graph legs but cannot acquire
a graph write through it.

## In scope

- `GET /api/v0/status/hosted-readiness` admits its repository-count
  leg (RunSingle over the repository-count read) and its
  workload-leftovers detection leg (Run through the Detect read)
  through the two re-declared methods (mirroring
  `querycontract.GraphQuery`); the snapshot leg rides the
  already-recorded `StatusReadAdmission v0` (`status.Reader`).

Explicitly OUT of this pin, recorded not silently assumed (each
keeps its own standing until a deliberate pin covers it):

- The `GraphBackend` selector is config, not a port: there is no
  method set to freeze (the non-Neo4j-backend unknown leg rides the
  terminated forward as a core-served answer).
- The Detect algorithm (candidate paging, examined caps, twin
  classification) is callee behavior, not port shape.
- The answer-narration posture override (`NarrationPosture` func
  field) is a handler field, not a port: there is no method set to
  freeze.
- The repository-count shape and the grant checks are handler
  behavior, not port shape.
- No suppression leg exists on this route; the suppressions mutation
  row stays the Decision 2 survivor, never a read termination here.

## Relay posture

The hosted-readiness legs terminate in the W8a lane (see
`contracts/w8a-termination.md`): shared-key callers terminate on the
native lane ahead of the generic proxy with byte parity to the
relay, while scoped-token, OIDC-bearer, and browser-session classes
relay explicitly until D8-grade parity. The pin is recorded here
first so the termination lane consumes a minted pin, never an
unminted one.

## Enforcement

`TestStatusHostedReadinessReadAdmissionContractIsVersioned` pins the
version and the frozen method set. A renamed, added, removed, or
re-signed method fails the frozen test until the list is deliberately
updated; bump `StatusHostedReadinessReadAdmissionContractVersion`
alongside (manually — the bump is convention, the frozen list is the
enforcement), and this repository records the new pin before
consuming it.
