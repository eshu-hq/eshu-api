# Wave-1 Freshness read-latency evidence

Wave-1 Freshness reads termination lane latency record (Refs
`eshu-hq/eshu#9669`, D13 tracker `eshu-hq/eshu#10491`, satellite
`eshu-hq/eshu-api#72`). The census assigns the satellite this gate:
core keeps the contract-registry and static-contract gates; this repo
proves the terminated reads are not slower than the relay they
replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkFreshnessRelayRead`),
  shared-key GET (`/api/v0/dependencies`) through the full production
  chain. The legs compare different routes by necessity (the Wave-1
  paths now terminate); every leg does exactly one server-side fetch,
  so the 1.50 bound stays a real pathology check.
- After legs: the terminated Wave-1 reads
  (`BenchmarkFreshnessTerminatedRead`), shared-key GETs
  (`/api/v0/freshness/changed-since`,
  `/api/v0/freshness/generations`,
  `/api/v0/freshness/services/changed-since` with representative
  filter queries) through the full production chain with the wave gate
  and family authorizer.
- Every leg hits the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-freshness-latency.sh` runs each leg
  `-count=5`, takes the median ns/op, and fails when terminated
  exceeds relay by more than 50% on any leg. The threshold is generous
  on purpose: every leg does exactly one server-side fetch, so any
  real pathology (a double fetch, a retry, a body re-read) exceeds it
  loudly while scheduling noise does not.

## Relay-route disclosure

The Wave-1 lane terminates no earlier relay leg, so the lane relays
against the route the D2-D4 lanes moved to:
`GET /api/v0/dependencies` (Dependencies family, still on the generic
proxy). The Wave-1 benchmark relay leg and its measure script use the
same route with the use disclosed in the script header, so every lane
gates terminated-vs-relay again instead of terminated-vs-terminated.

## Numbers (local, 2026-11-09 Wave-1 lane)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Changed-since GET | 84606 | 82844 | 1.02 | ≤ 1.50 |
| Generations GET | 82667 | 82844 | 1.00 | ≤ 1.50 |
| Service changed-since GET | 85530 | 82844 | 1.03 | ≤ 1.50 |

Within budget on all three legs: the lane adds the gate branch and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this evidence
runs hermetically via .github/workflows/read-latency.yml on
lane-affecting PRs and weekly, and locally with the script.
