# D9 clone-latency evidence

D9 latency record for the OIDC-login tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before legs: the generic byte-relay (`BenchmarkD8RelayWrite` for
  writes, `BenchmarkD8RelayRead` for reads) through the full
  production chain. The write-relay leg is the Compare POST
  (`/api/v0/compare/environments`), which stays on the generic relay
  by D6 disclosure (no pin covers its shared graph/content ports);
  the read-relay leg is GET `/api/v0/dependencies`, which stays on
  the generic relay.
- After legs: the D9 clones — the keyless login-start GET bare and
  with the full provider-IDs-plus-return_to query
  (`BenchmarkD9PublicLogin`), and the keyless callback GET carrying
  state + code (`BenchmarkD9PublicCallback`) — through the full
  production chain with the explicit route match and telemetry.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d9-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when a clone leg exceeds relay by
  more than 50%. The threshold is generous on purpose: both legs do
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The write-relay leg is the Compare POST (the still-relaying write the
D6 lane disclosed); the read-relay leg is GET `/api/v0/dependencies`
(the still-relaying read the D6 lane disclosed). Both legs do exactly
one server-side fetch, so the 1.50 bound stays a real pathology check.

## Numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Login-start GET, bare | 76645 | 85922 | 0.89 | ≤ 1.50 |
| Login-start GET, full query | 76351 | 85922 | 0.89 | ≤ 1.50 |
| Callback GET, state + code | 81259 | 85922 | 0.95 | ≤ 1.50 |
| Login-start GET vs relay POST | 76351 | 104978 | 0.73 | ≤ 1.50 |
| Callback GET vs relay POST | 81259 | 104978 | 0.77 | ≤ 1.50 |

Within budget on all five legs: the lane adds the explicit route
match around the same single fetch, with no gate and no authorizer on
the public carve-out. Absolute deployment numbers against a pinned
core image ride flip prep (D11); this gate runs hermetically on every
PR and in CI.
