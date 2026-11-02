# Replay ledger admission contract

Versioned admission-contract port for the shared replay idempotency
ledger (`admin_replay_requests`) that the acknowledgement, replay,
reopen, and recover-generations routes share (Refs
`eshu-hq/eshu#8872`). The API handler keeps the claim/complete semantics
below and calls this port; the core-owned Postgres admin store satisfies
it. Canonical mutations persist through explicit core admission;
`eshu-api` never takes a direct ledger handle.

## Version

`ReplayLedgerAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ReplayLedgerAdmission = Ledger
```

Both lines are vendored verbatim in
`contracts/upstream/replay-ledger-wire-shapes.txt`, with the surrounding
contract comment, the compile assertion, and the two-method ledger
interface. The frozen method set is `frozenReplayLedgerAdmission` in
`contracts/upstream/replay_ledger_admission_contract_test.go`
(2 methods: `ClaimReplayIdempotency`, `CompleteReplayIdempotency`).

## Ledger semantics

The claim/complete pair vendored verbatim in
`contracts/upstream/replay-ledger-wire-shapes.txt`:

- `ClaimReplayIdempotency(key, fingerprint, now)`: atomically claims an
  idempotency key; a refused acknowledgement leaves its key unconsumed
  for a corrected retry, and a duplicate claim of the same key with the
  same fingerprint resumes instead of duplicating.
- `CompleteReplayIdempotency(key, count, workItemIDs, now)`: records the
  outcome of a claimed key; the ledger row stays in progress on failure
  so a retry can resume.
- Unconfigured handler (nil ledger): the acknowledgement write answers
  503.

## Enforcement

`TestReplayLedgerAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port, and the core-owned admin store covers the port at compile time. A
breaking change to the ledger shape fails the frozen test until the
list is deliberately updated; bump
`ReplayLedgerAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
