# D7 clone-latency evidence

D7 latency record for the admin-four tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the explicit relay (`BenchmarkD7RelayWrite`),
  foreign-bearer POST `/api/v0/impact/change-surface` on its
  wave-mounted route through the full production chain. The
  change-surface route has since terminated natively in the impact
  lane's Wave-3 (so the shared-key shape would run
  terminated-vs-terminated); the Compare POST it replaces
  terminated in its gap-04 lane.
- After legs: the D7 clones (`BenchmarkD7NativeWrite`), shared-key
  dead-letter, reindex, backfill, and skip POSTs through the full
  production chain with the explicit route match and the family
  authorizer. All four callers carry zero bodies: the benchmark
  isolates route-match plus authorizer cost, not body parsing.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d7-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when a clone leg exceeds relay by
  more than 50%. The threshold is generous on purpose: both legs do
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The relay leg is POST `/api/v0/impact/change-surface` via
foreign-bearer explicit relay: the W6 scan lane re-pointed it there
when it terminated the last code POST, and this lane re-pointed the
caller to the foreign bearer when change-surface terminated natively
in the impact lane's Wave-3. Both legs do exactly one server-side
fetch, so the 1.50 bound stays a real pathology check.

## Numbers (local, 2026-11-07; relay leg re-pointed 2026-11-15)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Dead-letter POST | 45298 | 44347 | 1.02 | ≤ 1.50 |
| Reindex POST | 44151 | 44347 | 1.00 | ≤ 1.50 |
| Backfill POST | 44208 | 44347 | 1.00 | ≤ 1.50 |
| Skip POST | 43897 | 44347 | 0.99 | ≤ 1.50 |

Re-measured 2026-11-27 with all four admin-four legs (backfill and
skip added as zero-body callers, same as the existing legs): earlier
runs on the same host measured 83493/83737 (1.00) and 82477/83737
(0.98) for dead-letter and reindex. The absolutes moved with
scheduling noise while every leg stays at parity with the relay.

Re-measured 2026-11-15 after the W6 scan lane re-pointed the relay
leg from the terminated code bundles POST to the impact
change-surface POST (then the relay leg; change-surface has since
terminated natively in the impact lane's Wave-3, and the leg now
rides foreign-bearer explicit relay).

Within budget on all four legs: the lane adds the explicit route
match and one authorizer call around the same single fetch. Absolute
deployment numbers against a pinned core image ride flip prep (D11);
this gate runs hermetically on every PR and in CI. An earlier run on
the same host and code measured 168043/168119 (1.00) and
163105/168119 (0.97): shared-host scheduling noise moves the
absolutes while every leg moves together, so the ratio stays
comfortably inside the budget either way. Script wall time on this
host: ~4s.
