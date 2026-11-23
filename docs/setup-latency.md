# Setup latency evidence

D10 latency record for the setup lane (Refs `eshu-hq/eshu#9669`,
part of `eshu-hq/eshu#6707`). The census assigns the satellite this
gate: core keeps the contract-registry and static-contract gates;
this repo proves the terminated setup-state GET and the
terminated-writer setup trio (claim/admin/mfa) are not slower than the
relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before legs: the generic byte-relay write-through the C-c1 lane
  rode (`BenchmarkSetupState` and `BenchmarkSetupWrite` against the
  pre-D10 tree, 2000x3).
- After legs: the terminated setup-state GET (single SetupNeeded
  port fetch plus the dest-synthesized 200) and the terminated-writer
  claim POST (explicit public route plus the write-through forward)
  through the
  full production chain (`BenchmarkSetupState` and
  `BenchmarkSetupWrite` against the D10 tree, 2000x5 via
  `bash scripts/measure-setup-latency.sh`).
- Relay legs: foreign-bearer POST `/api/v0/impact/change-surface`
  for writes and foreign-bearer GET `/api/v0/dependencies` for reads,
  both via explicit relay on their wave-mounted routes (change-surface
  has since terminated natively in the impact lane's Wave-3 and
  dependencies in W8b-narrow, so the shared-key shape would run
  terminated-vs-terminated). Both legs hit the same
  in-process stub core, so the numbers isolate dest-side cost, not
  core or network time.
- The gate takes the median ns/op and fails when a setup leg exceeds
  relay by more than 50%. The threshold is generous on purpose: every
  leg does exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Relay-route disclosure

The write-relay leg is POST `/api/v0/impact/change-surface` via
foreign-bearer explicit relay (change-surface has since terminated
natively in the impact lane's Wave-3); the read-relay leg is GET
`/api/v0/dependencies` via foreign-bearer explicit relay
(dependencies has since terminated natively in W8b-narrow). Both
legs do exactly one server-side fetch, so the 1.50 bound stays a real
pathology check.

## Numbers (local, 2026-11-07; relay leg re-pointed 2026-11-15)

Shared host, in-process stub core (`go test` benchmarks):

| Leg | Before (median ns/op) | After (median ns/op) | Relay (median ns/op) | Ratio (after/relay) | Budget |
| --- | --- | --- | --- | --- | --- |
| Setup-state GET | 80898 | 74173 | 84281 | 0.88 | ≤ 1.50 |
| Claim POST | 85844 | 83072 | 84172 | 0.99 | ≤ 1.50 |

Re-measured 2026-11-15 after the W6 scan lane re-pointed the relay
leg from the terminated code bundles POST to the impact
change-surface POST (then the relay leg; change-surface has since
terminated natively in the impact lane's Wave-3, and the leg now
rides foreign-bearer explicit relay) (before column unchanged).

No regression on either route: the terminated GET is faster than
the write-through it replaces (no body relay — dest synthesizes the
200 from the single port fetch), and the claim POST is unchanged
transport with the same single fetch. Absolute deployment numbers
against a pinned core image ride flip prep (D11); this gate runs
hermetically on every PR and in CI.

## RED/GREEN pair

The gate demonstrates its FAIL path: with a planted 2 ms sleep per
state-leg iteration (scratch worktree at the merged D10 head, since
removed), the script reports `leg state: clone=2372372ns
relay=84278ns ratio=28.15` and exits 1 with `FAIL: clone leg state
regresses past 1.50x of relay`. On the clean tree the same script
exits 0 with both legs within budget (table above). A real pathology
— a double fetch, a retry, a body re-read — exceeds the 1.50 bound
the same way; scheduling noise does not.
