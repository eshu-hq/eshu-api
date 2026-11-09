# Compare relay-leg latency evidence

Compare latency record for the gap-04 relay-leg termination lane
(Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#72`). The
census assigns the satellite this gate: core keeps the
contract-registry and static-contract gates; this repo proves the
terminated Compare POST is not slower than the relay it replaces.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkCompareRelayPOST`),
  shared-key POST (`/api/v0/code/bundles`) through the full
  production chain. The legs compare different routes by necessity
  (the Compare POST now terminates); every leg does exactly one
  server-side fetch, so the 1.50 bound stays a real pathology check.
- After leg: the terminated Compare POST
  (`BenchmarkCompareTerminatedPOST`), shared-key POST
  (`/api/v0/compare/environments`) with the parity request body
  through the full production chain with the wave gate and family
  authorizer.
- Every leg hits the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-compare-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when terminated exceeds relay by
  more than 50%. The threshold is generous on purpose: every leg does
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The relay leg is POST `/api/v0/code/bundles`: it is the
still-relaying write the D3/D4/Leaf B lanes pin as their relay POST
leg. The Compare benchmark relay leg and its measure script use the
same route with the use disclosed in the script header, so the lane
gates terminated-vs-relay, not terminated-vs-terminated.

## Numbers (local, 2026-11-09)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Compare POST | 92078 | 92832 | 0.99 | ≤ 1.50 |

Within budget: the terminated forward costs the same single fetch as
the relay, plus the shared wave gate and family authorizer the other
terminated lanes already pay.

Absolute deployment numbers against a pinned core image ride flip
prep (D11); this gate runs hermetically on every PR and in CI.
