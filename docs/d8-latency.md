# D8 clone-latency evidence

D8 latency record for the session-resolver tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before legs: the generic byte-relay (`BenchmarkD8RelayWrite` for
  writes, `BenchmarkD8RelayRead` for reads) through the full
  production chain. The write-relay leg is POST
  `/api/v0/code/bundles`, which stays on the generic relay (the
  D3/D4/Leaf B lanes pin it as their relay POST leg; the Compare POST
  it replaces terminated in its gap-04 lane);
  the read-relay leg is GET `/api/v0/dependencies`, which stays on
  the generic relay.
- After legs: the D8 clones — shared-key browser-session create and
  API-token create POSTs (`BenchmarkD8NativeWrite`), the keyless
  public login POST (`BenchmarkD8PublicWrite`), and the shared-key
  session-list and profile GETs (`BenchmarkD8NativeRead`) — through
  the full production chain with the explicit route match, the
  credential-class gate, and the family authorizer.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d8-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when a clone leg exceeds relay by
  more than 50%. The threshold is generous on purpose: both legs do
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The write-relay leg is POST `/api/v0/code/bundles` (the still-relaying
write the D3/D4/Leaf B lanes pin as their relay POST leg); the
read-relay leg is GET `/api/v0/dependencies`
(the still-relaying read the D6 lane disclosed). Both legs do exactly
one server-side fetch, so the 1.50 bound stays a real pathology check.

## Numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Browser-session create POST | 84788 | 86816 | 0.98 | ≤ 1.50 |
| API-token create POST | 85082 | 86816 | 0.98 | ≤ 1.50 |
| Public login POST | 87797 | 86816 | 1.01 | ≤ 1.50 |
| Session-list GET | 86085 | 86408 | 1.00 | ≤ 1.50 |
| Profile GET | 86216 | 86408 | 1.00 | ≤ 1.50 |

Within budget on all five legs: the lane adds the explicit route
match, the credential-class gate, and one authorizer call around the
same single fetch. Absolute deployment numbers against a pinned core
image ride flip prep (D11); this gate runs hermetically on every PR
and in CI.
