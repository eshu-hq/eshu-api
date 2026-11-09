# D7 clone-latency evidence

D7 latency record for the admin-four tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkD7RelayWrite`), POST
  `/api/v0/code/bundles` through the full production chain. The
  code-bundles route stays on the generic relay (the D3/D4/Leaf B
  lanes pin it as their relay POST leg); the Compare POST it replaces
  terminated in its gap-04 lane.
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

The relay leg is POST `/api/v0/code/bundles`: it is the still-relaying
write the D3/D4/Leaf B lanes pin as their relay POST leg. Both legs do
exactly one server-side fetch, so the 1.50 bound stays a real
pathology check.

## Numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Dead-letter POST | 85242 | 93171 | 0.91 | ≤ 1.50 |
| Reindex POST | 84142 | 93171 | 0.90 | ≤ 1.50 |

Within budget on both legs: the lane adds the explicit route match and
one authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI. An earlier run on the same
host and code measured 168043/168119 (1.00) and 163105/168119 (0.97):
shared-host scheduling noise moves the absolutes while both legs move
together, so the ratio stays comfortably inside the budget either way.
