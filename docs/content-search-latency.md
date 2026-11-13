# W4 Content search latency evidence

W4 Content search termination lane latency record (Refs
`eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#111`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the terminated search reads are
not slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the generic byte-relay (`BenchmarkContentSearchRelayPOST`),
  shared-key POST (`/api/v0/code/bundles`) through the full production
  chain. The legs compare different routes by necessity (the 4 search
  paths now terminate; the still-relayed code bundles POST no wave table
  covers still relays); every leg does exactly one server-side fetch, so
  the 1.50 bound stays a real pathology check.
- After legs: the terminated W4 search reads
  (`BenchmarkContentSearchTerminatedPOST`), shared-key POSTs
  (`/api/v0/content/files/search`,
  `/api/v0/content/entities/search`,
  `/api/v0/content/dockerfile-installs/search`,
  `/api/v0/content/php-config/search` with representative scoped and
  unscoped bodies) through the full production chain with the wave gate
  and family authorizer.
- Every leg hits the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-content-search-latency.sh` runs each leg
  `-count=5`, takes the median ns/op, and fails when terminated
  exceeds relay by more than 50% on any leg. The threshold is generous
  on purpose: every leg does exactly one server-side fetch, so any
  real pathology (a double fetch, a retry, a body re-read) exceeds it
  loudly while scheduling noise does not.

## Relay-route disclosure

The W4 lane terminates the search legs, so the lane relays against a
still-relayed code route that no wave table covers:
`POST /api/v0/code/bundles` (Code family, still relayed).
Same-method legs, so every lane gates terminated-vs-relay again
instead of terminated-vs-terminated.

## Numbers (local, 2026-11-12 W4 lane)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Files-search POST | 92521 | 91376 | 1.01 | ≤ 1.50 |
| Entities-search POST | 95571 | 91376 | 1.05 | ≤ 1.50 |
| Dockerfile-search POST | 92121 | 91376 | 1.01 | ≤ 1.50 |
| PHP-config-search POST | 90640 | 91376 | 0.99 | ≤ 1.50 |

Within budget on all four legs: the lane adds the gate branch and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI.
