# D6 read-latency evidence

D6 latency record for the Status + SupplyChain reads termination lane
(Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census
assigns the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the terminated reads are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkD6RelayRead`),
  shared-key GET (`/api/v0/dependencies`) through the full production
  chain. The legs compare different routes by necessity (the D6 paths
  now terminate); every leg does exactly one server-side fetch, so the
  1.50 bound stays a real pathology check.
- After legs: the terminated D6 reads (`BenchmarkD6TerminatedRead`),
  shared-key Status GET (`/api/v0/status/pipeline`) + SupplyChain GET
  (`/api/v0/supply-chain/impact/findings`) through the full production
  chain with the wave gate and family authorizer, plus the phase-2
  legs (`BenchmarkD6ExtraTerminatedRead`): shared-key GETs
  (`/api/v0/status/operations`, `/api/v0/index-status`,
  `/api/v0/status/index`) through the same gate and authorizer.
- Every leg hits the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d6-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when terminated exceeds relay by
  more than 50% on any leg. The threshold is generous on purpose: every
  leg does exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Relay-route disclosure

The D6 wave terminates no earlier relay leg, so the lane relays
against the route the D2-D4 lanes moved to:
`GET /api/v0/dependencies` (Dependencies family, still on the generic
proxy). The D6 benchmark relay leg and its measure script use the same
route with the use disclosed in the script header, so every lane gates
terminated-vs-relay again instead of terminated-vs-terminated.

## Numbers (local, 2026-11-06 base lane; phase-2 legs 2026-11-08)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Status GET | 103801 | 88663 | 1.17 | ≤ 1.50 |
| Supply GET | 113935 | 88663 | 1.29 | ≤ 1.50 |

Within budget on both legs: the lane adds the gate branch and one
authorizer call around the same single fetch. Ratios above 1.0 on this
run are shared-host scheduling noise — a single-count probe on the
same host measured terminated 86517/87958 vs relay 90115 (ratios below
1.0). Absolute deployment numbers against a pinned core image ride
flip prep (D11); this gate runs hermetically on every PR and in CI.

Phase-2 legs (same method, same host class, 2026-11-08):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Extra operations GET | 81477 | 81287 | 1.00 | ≤ 1.50 |
| Extra index-status GET | 80561 | 81287 | 0.99 | ≤ 1.50 |
| Extra status-index GET | 81883 | 81287 | 1.01 | ≤ 1.50 |

Within budget on all three phase-2 legs: the terminated forward costs
the same single fetch as the relay, plus the shared wave gate and
family authorizer the base lane already pays.
