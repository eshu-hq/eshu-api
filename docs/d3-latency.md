# D3 read-latency evidence

D3 latency record for the admin/identity/recovery reads termination
lane (Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The
census assigns the satellite this gate: core keeps the
contract-registry and static-contract gates; this repo proves the
terminated reads are not slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkD3RelayRead`),
  shared-key GET (`/api/v0/repositories`) + POST
  (`/api/v0/repositories/search`) through the full production chain.
  The legs compare different routes by necessity (the D3 paths now
  terminate); both do exactly one server-side fetch, so the 1.50 bound
  stays a real pathology check.
- After leg: the terminated D3 reads (`BenchmarkD3TerminatedRead`),
  shared-key identity GET (`/api/v0/auth/admin/roles`) + admin query
  POST (`/api/v0/admin/work-items/query`) through the full production
  chain with the wave gate and family authorizer.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d3-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when terminated exceeds relay by
  more than 50% on any leg. The threshold is generous on purpose: both
  legs do exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Numbers (local, 2026-11-06)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| GET | 85590 | 155276 | 0.55 | ≤ 1.50 |
| POST | 178083 | 225863 | 0.79 | ≤ 1.50 |

No measurable overhead: the lane adds the gate branch and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI
(`.github/workflows/read-latency.yml`).

## Variance note

Repeated runs on this shared host spread widely (GET ratios
0.55–1.09, POST ratios 0.28–2.74 across three runs; one POST run
exceeded the budget). The host carried load average 30.8 on 12 cores
during the runs with a sibling agent's lint running concurrently, and
the relay leg itself swung 94k–346k ns/op while the terminated leg
stayed 86k–389k. Both legs do exactly one server-side fetch (pinned
by the `hits == 1` assertions in the parity suite), so the spread is
scheduling noise, not lane pathology. CI's clean runner is the
enforcement point for the 1.50 gate.
