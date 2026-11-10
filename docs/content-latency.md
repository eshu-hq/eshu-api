# Wave-2 Content point-read latency evidence

Wave-2 Content point reads termination lane latency record (Refs
`eshu-hq/eshu#9669`, D13 tracker `eshu-hq/eshu#10491`, satellite
`eshu-hq/eshu-api#72`). The census assigns the satellite this gate:
core keeps the contract-registry and static-contract gates; this repo
proves the terminated reads are not slower than the relay they
replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkContentRelayRead`),
  shared-key POST (`/api/v0/content/files/search`) through the full
  production chain. The legs compare different routes by necessity
  (the wave paths now terminate; the out-of-pin file search POST
  still relays); every leg does exactly one server-side fetch, so the
  1.50 bound stays a real pathology check.
- After legs: the terminated Wave-2 reads
  (`BenchmarkContentTerminatedRead`), shared-key POSTs
  (`/api/v0/content/files/read`,
  `/api/v0/content/files/lines`,
  `/api/v0/content/entities/read` with representative bodies,
  including a line-range slicing body) through the full production
  chain with the wave gate and family authorizer.
- Every leg hits the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-content-latency.sh` runs each leg
  `-count=5`, takes the median ns/op, and fails when terminated
  exceeds relay by more than 50% on any leg. The threshold is generous
  on purpose: every leg does exactly one server-side fetch, so any
  real pathology (a double fetch, a retry, a body re-read) exceeds it
  loudly while scheduling noise does not.

## Relay-route disclosure

The Wave-2 lane terminates the point reads but not the search legs,
so the lane relays against the out-of-pin search route that stays on
the generic proxy: `POST /api/v0/content/files/search` (Content
family, still relayed). Same-method legs, so every lane gates
terminated-vs-relay again instead of terminated-vs-terminated.

## Numbers (local, 2026-11-10 Wave-2 lane)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Files-read POST | 91531 | 93587 | 0.98 | ≤ 1.50 |
| Files-lines POST | 92692 | 93587 | 0.99 | ≤ 1.50 |
| Entities-read POST | 96961 | 93587 | 1.04 | ≤ 1.50 |

Within budget on all three legs: the lane adds the gate branch and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI.
