# Leaf B read-latency evidence

D2 latency record for the Native-now termination lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the terminated wave is not
slower than the relay it replaces.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkLeafBRelayRead`),
  shared-key GET + POST through the full production chain.
- After leg: the terminated wave (`BenchmarkLeafBTerminatedRead`),
  shared-key GET (`/api/v0/capabilities`) + POST (`/api/v0/ask`)
  through the full production chain with the wave gate and family
  authorizer.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-leafb-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when terminated exceeds relay by
  more than 50% on any leg. The threshold is generous on purpose: both
  legs do exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Numbers (local, 2026-11-06)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| GET | 91268 | 92650 | 0.99 | ≤ 1.50 |
| POST | 99994 | 96971 | 1.03 | ≤ 1.50 |

No measurable overhead: the lane adds the gate branch and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI
(`.github/workflows/read-latency.yml`).
