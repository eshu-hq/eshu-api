# Recovery admission contracts

Versioned admission-contract ports for the four admin recovery
mutations (`POST /api/v0/admin/replay`, `POST /api/v0/admin/reopen`,
`POST /api/v0/admin/refinalize`, `POST
/api/v0/admin/recover-generations`) (Refs `eshu-hq/eshu#8872`,
arbiter C-c2 SPLIT on `eshu-hq/eshu#9669`). The API handler keeps the
HTTP shape below and calls these ports; the core-owned Postgres store
and recovery handler satisfy them. `eshu-api` never takes a direct
table handle.

Two contracts move together: the recovery-operation port for the
refinalize/recover-generations work (plus the replay-failed operation
replay rides on), and the replay-ledger port for the
claim/complete pair the replay, reopen, and recover-generations routes
share. Refinalize carries no idempotency key and records no ledger
row; replay, reopen, and recover-generations do.

## Version

`RecoveryAdmissionContractVersion = "v0"`, with the port defined as:

```go
type RecoveryAdmission = RecoveryService
```

`ReplayLedgerAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ReplayLedgerAdmission = Ledger
```

The recovery lines are vendored verbatim in
`contracts/upstream/recovery-wire-shapes.txt`, with the surrounding
contract comment, the port alias, the compile assertion, the
`RecoveryService` port (2 methods: `Refinalize`, `ReplayFailed`),
and the four in-tree `Mount` registrations. The frozen method set is
`frozenRecoveryAdmission` in
`contracts/upstream/recovery_admission_contract_test.go`
(2 methods). The ledger port stays where the terminal-failure lane
put it (`contracts/replay-ledger-admission.md`,
`contracts/upstream/replay_ledger_admission_contract_test.go` with
`frozenReplayLedgerAdmission`, 2 methods: `ClaimReplayIdempotency`,
`CompleteReplayIdempotency`); this lane consumes it unchanged.

Out of contract by design: backfill, dead-letter, reindex, and skip
have no `RecoveryAdmission` or `ReplayLedgerAdmission` pin and stay on
the generic relay — nobody infers them into this lane. They move only
with their own admission pins in later admin-mutation lanes.

## HTTP shape

The operator-facing recovery bodies and results, with the
claim/complete pair vendored in
`contracts/upstream/recovery-wire-shapes.txt`:

- Replay: explicit reason and idempotency key; at least one selector
  (`work_item_ids`, `scope_id`, `stage`, `failure_class`); unsafe
  failure classes refused without `force`; explicit unsafe or
  superseded-generation ids refused whole before the claim (the key
  stays unconsumed); a 200 with `replayed_count` 0 means nothing
  matched.
- Reopen: explicit domain (one of `repo_dependency`,
  `workload_materialization`, `submodule_pin`), scope, reason, and
  idempotency key. The scope resolves before the claim: an unknown
  scope 404s, an ambiguous scope 409s, and a generation-less scope
  422s without consuming the key.
- Refinalize: `scope_ids`, no idempotency key, no ledger row. A 409
  with `blocked/claim_fence_busy` means the reducer claim fence stayed
  busy through its bounded retry window.
- Recover-generations: either `scope_ids` or `all_scopes` (never
  both), plus reason and idempotency key. A duplicate delivery returns
  the prior outcome (`duplicate=true`); a key reused across the two
  modes 409s.
- Ledger protocol through dest: a refusal before the claim leaves the
  key unconsumed for a corrected retry; a duplicate claim of the same
  key with the same fingerprint replays the stored outcome
  (`duplicate`); the same key with a different target, or a
  still-`in_progress` key, answers 409; the ledger row stays in
  progress on failure so a retry resumes instead of losing the
  outcome.
- Every mutating route gates on admin (all-scopes) authorization.
- Unconfigured handler (nil recovery, store, or origin): 503.

## Enforcement

`TestRecoveryAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A renamed, added, removed, or re-signed method fails the frozen
test until the list is deliberately updated; bump
`RecoveryAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.

## Native dispatch (tier C-c2: HTTP write-through)

The four admin recovery mutations are the tier C-c2 native-dispatch
lane (arbiter C-c2 SPLIT on `eshu-hq/eshu#9669`): `api/recovery.go`
registers the explicit routes inside `RequireAuth` ahead of the
generic proxy —

- `POST /api/v0/admin/replay`,
- `POST /api/v0/admin/reopen`,
- `POST /api/v0/admin/refinalize`,
- `POST /api/v0/admin/recover-generations`

— each with the AllScopes family authorizer (plus the trailing-slash
twin on the same handler, so the family is never split across dispatch
paths), and forwards to the same core route with method, path, query,
headers (including the original `Authorization` header), and body
relayed; core bytes come back unmodified and core re-judges every
forwarded call as backstop. The relay-only four and the family's reads
stay on the generic relay (`TestRecoveryNative_RelayOnlyStaysProxy`,
`TestRecoveryNative_ReadsStillProxy`).

Scope honesty: dest holds the AllScopes authorizer per the dispatch
ruling, but with no scoped/session resolver the deny branch is
unreachable on the live routes until one lands — the live 403s on these
routes are core's, which re-judges all-scope admin, the admin tenant
scope, and the recovery permission catalog as backstop. Core is the
sole writer. The deny-shape test pins the contract for the
resolver-landing day, not a live path. Label the family
native-dispatch (not native auth termination) until the dest scoped
resolver lands.

Idempotent-retry parity: every ledger-backed mutation is safe under
retry at the idempotency-key claim — a duplicate delivery replays the
stored outcome (`duplicate=true`), a reused key across targets
conflicts 409, and an in-progress key waits with 409 whose single
upstream hit proves dest never retries a conflict. Refinalize has no
key: its 409 (`blocked/claim_fence_busy`) is the fence telling the
operator to retry after contention subsides, and its single upstream
hit proves the same. A forwarded POST is never retried — the refused
422s, the unknown-scope 404s, the 409s, the dead-core 503, and the
caller-cancel 503 each prove exactly one upstream hit.

Cancel and timeout parity for long recoveries: dest never retries, so
a caller cancel or dispatch-timeout answer is the permanent local 503
`backend_unavailable` with exactly one upstream attempt
(`TestRecoveryNative_CanceledContextFailsClosedWithoutRetry`, pinned
on the recover-generations route). Core still records a committed
outcome on its cancel-proof finalize context, so the idempotency-key
retry replays the stored outcome instead of losing it; refinalize runs
in one transaction, so a cancel rolls it back unless it lands while
the commit itself runs.

Audit parity: governance audit stays core-side on forwarded requests.
The in-tree recovery handlers emit `replay_accepted`,
`reopen_accepted`, the refinalize/recover-generations outcomes, and
every denied cause (missing reason/key, unauthorized, unsafe-class,
superseded, key-reused, in-progress, and the reopen resolve refusals)
from inside the core handlers that still run on every forwarded
request, so write-through keeps every audit event core-side with no
gap. Dest emits none. A family whose handlers emit audit must never
be re-implemented dest-side without a versioned audit-append contract;
this lane re-implements nothing.
`scripts/check-parity.sh` gates the lane with the no-core-import proof
(`go list -deps` over the runtime packages must show no
`eshu-hq/eshu` import) alongside the carve-out, sealed-state, 401,
no-retry, and ledger-protocol proofs above.

OpenAPI: the four route entries move as a line-verbatim subset of the
pinned core file (satellite-design Decision 5(c); see below). The
four path keys (`replay`, `reopen`, `refinalize`,
`recover-generations`) are unique to this lane and separable from the
relay-only four without duplicating a key or misrepresenting posture:
the only shared `$ref`s (`SkippedScopesReport`,
`DeltaActiveScopesReport`, `ReindexRequestsWrittenReport`) are
referenced solely by the refinalize and recover-generations entries,
so this lane ports the entries plus exactly those three schemas and
nothing the relay-only routes document. Assembly adds the schemas this
lane's entries reference; `TestSpec_LocalRefsResolve` fails closed on
a dangling `$ref`. The served dest spec therefore covers exactly the
landed families; backfill, dead-letter, reindex, and skip stay
relayed-from-core for spec purposes until their lanes move their
entries.

Rollback: additive. Revert this lane's dest commit (or re-pin the
prior digest) and the four recovery routes fall back to the generic
byte-relay — where they answer dest 401s until the lane returns. No
core change, no migration, no flag; no dest-side recovery or ledger
state exists to unwind.
