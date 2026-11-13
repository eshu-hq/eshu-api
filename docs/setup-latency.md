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
  port fetch plus the dest-synthesized 200) and the tier-C claim POST
  (explicit public route plus the write-through forward) through the
  full production chain (`BenchmarkSetupState` and
  `BenchmarkSetupWrite` against the D10 tree, 2000x5 via
  `bash scripts/measure-setup-latency.sh`).
- Relay legs: POST `/api/v0/code/bundles` for
  writes and GET `/api/v0/dependencies` for reads, which stay on the
  generic relay (the D3/D4/Leaf B lanes pin them as their relay legs;
  the Compare POST previously used for writes terminated in its
  gap-04 lane). Both legs hit the same
  in-process stub core, so the numbers isolate dest-side cost, not
  core or network time.
- The gate takes the median ns/op and fails when a setup leg exceeds
  relay by more than 50%. The threshold is generous on purpose: every
  leg does exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Relay-route disclosure

The write-relay leg is POST `/api/v0/code/bundles` (the still-relaying
write the D3/D4/Leaf B lanes pin as their relay POST leg); the
read-relay leg is GET
`/api/v0/dependencies` (the still-relaying read the D6 lane
disclosed). Both legs do exactly one server-side fetch, so the 1.50
bound stays a real pathology check.

## Numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks):

| Leg | Before (median ns/op) | After (median ns/op) | Relay (median ns/op) | Ratio (after/relay) | Budget |
| --- | --- | --- | --- | --- | --- |
| Setup-state GET | 80898 | 74232 | 84021 | 0.88 | ≤ 1.50 |
| Claim POST | 85844 | 84379 | 83561 | 1.01 | ≤ 1.50 |

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
