# Audit append admission contract

Versioned admission-contract port for the audit-append port behind the 18
tier-C audit-emitting routes (Refs `eshu-hq/eshu#9669`, O2 ORDER).
Canonical mutations persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned governance-audit appender satisfies.
`eshu-api` never takes a direct store handle.

This is the one shared pin covering all 18 routes (O2 ORDER): every
emitter admits through the single `Append` method, mirroring
`GovernanceAuditAppender`. Pinning the shared port itself would be a
blanket pin the minting rule forbids permanently. The pin re-declares
the single mirrored signature in this audit-scoped shape instead of
embedding or aliasing the shared port, so no other family can cite it
as coverage.

## Version

`AuditAppendAdmissionContractVersion = "v0"`, with the port defined as:

```go
type AuditAppendAdmission interface {
	Append(ctx context.Context, events []governanceaudit.Event) error
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, D-wave consumer, rollback) are vendored verbatim in
`contracts/upstream/audit-append-wire-shapes.txt`. The frozen method
set is `frozenAuditAppendAdmission` in
`contracts/upstream/audit_append_admission_contract_test.go`
(1 method: `Append`).

## Append surface

The seam carries the single bounded append every emitter admits
through. The witness is a minimal recorder carrying exactly the pinned
method; production satisfaction stays with the existing
`GovernanceAuditAppender` implementations named by the compile-time
mirror assertion on the pin.

## In scope

- The 18 tier-C audit-emitting routes admit their audit append
  through `Append` (mirroring `GovernanceAuditAppender`).

Explicitly OUT of this pin: every non-audit consumer that appends
through the shared port keeps its own standing until a deliberate pin
covers it; no other family may cite this pin as coverage.

## Relay posture

All 18 routes stay on their tier-C dispatch lanes until the
termination lane lands; no native route moves in this record. The pin
is recorded here first so the termination lane consumes a minted pin,
never an unminted one. Until mint plus arbiter review plus record,
all 18 legs carry tier-C dispatch.

## Enforcement

`TestAuditAppendAdmissionContractIsVersioned` pins the version and
the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `AuditAppendAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
