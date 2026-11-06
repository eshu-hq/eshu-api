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

## Native dispatch (tier C-a: the ledger half of the family PR)

The replay ledger rides the terminal-failure native-dispatch lane in
the SAME PR (Decision 5 acceptance box): the acknowledgement POST is
tier C-a HTTP write-through (`api/terminalfailure.go`), and the
claim/complete pair it depends on runs inside the core handler on every
forwarded request. Dest holds no ledger handle, keeps no ledger state,
and takes no direct table handle — canonical mutations persist through
explicit core admission. Core is the sole writer of
`admin_replay_requests`.

Dest-side ledger semantics: none. Dest forwards every duplicate
delivery exactly once (no dest retry, no dest dedup) and relays core's
collapse byte-for-byte, so the stranded-`in_progress` and
fingerprint-mismatch windows stay exactly as core defines them: a
refusal before the claim leaves the key unconsumed for a corrected
retry; a duplicate claim of the same key with the same fingerprint
replays the stored outcome (`duplicate`); the same key with a different
target, or a still-`in_progress` key, answers 409; the ledger row stays
in progress on failure so a retry resumes instead of losing the
outcome.

Concurrency proof (the center of this family PR):
`TestTerminalFailureNative_DuplicateDeliveryCollapsesToOneEffect`,
`TestTerminalFailureNative_KeyReuseAcrossTargetsIs409`, and
`TestTerminalFailureNative_ParallelDuplicateDeliveries` in
`api/terminalfailure_ledger_test.go` run a ledger-faithful core fake
(same claim/complete rules, same target-fingerprint scheme) and assert
the ledger row — one recorded effect, the completed claim, the replayed
stored outcome — after sequential and 16-way parallel duplicate
deliveries. Dest never invents a ledger state: the fake's ledger is the
core contract, dest only forwards and relays.
