# D8 clone-latency evidence

D8 latency record for the session-resolver tier-C clone lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the tier-C clones are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before legs: the explicit relay (`BenchmarkD8RelayWrite` for
  writes, `BenchmarkD8RelayRead` for reads) through the full
  production chain. The write-relay leg is foreign-bearer POST
  `/api/v0/impact/change-surface` on its wave-mounted route
  (change-surface has since terminated natively in the impact lane's
  Wave-3, so the shared-key shape would run
  terminated-vs-terminated);
  the read-relay leg is foreign-bearer GET `/api/v0/dependencies`
  on its wave-mounted route (dependencies has since terminated
  natively in W8b-narrow for the shared key).
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

The write-relay leg is POST `/api/v0/impact/change-surface` via
foreign-bearer explicit relay (the route the W6 scan lane re-pointed
when it terminated the last code POST; change-surface has since
terminated natively in the impact lane's Wave-3); the read-relay leg
is GET `/api/v0/dependencies` via foreign-bearer explicit relay
(dependencies has since terminated natively in W8b-narrow). Both legs
do exactly one server-side fetch, so the 1.50 bound stays a real
pathology check.

## Numbers (local, 2026-11-07; relay leg re-pointed 2026-11-15)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Browser-session create POST | 84788 | 86816 | 0.98 | ≤ 1.50 |
| API-token create POST | 85082 | 86816 | 0.98 | ≤ 1.50 |
| Public login POST | 81310 | 83557 | 0.97 | ≤ 1.50 |
| Session-list GET | 82498 | 84040 | 0.98 | ≤ 1.50 |
| Profile GET | 82948 | 84040 | 0.99 | ≤ 1.50 |

Re-measured 2026-11-15 after the W6 scan lane re-pointed the relay
leg from the terminated code bundles POST to the impact
change-surface POST (then the relay leg; change-surface has since
terminated natively in the impact lane's Wave-3, and the leg now
rides foreign-bearer explicit relay) (create-POST rows unchanged).

Within budget on all five legs: the lane adds the explicit route
match, the credential-class gate, and one authorizer call around the
same single fetch. Absolute deployment numbers against a pinned core
image ride flip prep (D11); this gate runs hermetically on every PR
and in CI.
