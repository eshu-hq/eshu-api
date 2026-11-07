# D7 clone-latency evidence

D7 latency record for the admin-four tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkD7RelayWrite`), the
  Compare POST (`/api/v0/compare/environments`) through the full
  production chain. The Compare route stays on the generic relay by D6
  disclosure (no pin covers its shared graph/content ports).
- After legs: the D7 clones (`BenchmarkD7NativeWrite`), shared-key
  dead-letter and reindex POSTs through the full production chain with
  the explicit route match and family authorizer.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d7-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when a clone leg exceeds relay by
  more than 50%. The threshold is generous on purpose: both legs do
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The relay leg is the Compare POST: it is the still-relaying write the
D6 lane disclosed (Compare admits only through the shared
graph/content ports, deliberately unpinned). Both legs do exactly one
server-side fetch, so the 1.50 bound stays a real pathology check.

## Numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Dead-letter POST | 168043 | 168119 | 1.00 | ≤ 1.50 |
| Reindex POST | 163105 | 168119 | 0.97 | ≤ 1.50 |

Within budget on both legs: the lane adds the explicit route match and
one authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI.
