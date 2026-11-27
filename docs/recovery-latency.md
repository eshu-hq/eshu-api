# Recovery clone-latency evidence

Recovery latency record for the admin-recovery tier C-c2 clone lane
(Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census
assigns the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the recovery tier-C clones are
not slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the explicit relay
  (`BenchmarkRecoveryRelayWrite`), foreign-bearer POST
  `/api/v0/impact/change-surface` on its wave-mounted route through
  the full production chain. The change-surface route has since
  terminated natively in the impact lane's Wave-3 (so the shared-key
  shape would run terminated-vs-terminated); the Compare POST it
  replaces terminated in its gap-04 lane.
- After legs: the recovery clones
  (`BenchmarkRecoveryNativeWrite`), shared-key replay, reopen,
  refinalize, and recover-generations POSTs through the full
  production chain with the explicit route match and family
  authorizer. Each leg forwards its representative caller JSON body
  (the dispatch-pin wire shapes); the request is rebuilt per iteration
  with Body and ContentLength reset because the server consumes the
  body.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-recovery-latency.sh` runs each leg
  `-count=5`, takes the median ns/op, and fails when a clone leg
  exceeds relay by more than 50%. The threshold is generous on
  purpose: every leg does exactly one server-side fetch, so any real
  pathology (a double fetch, a retry, a body re-read) exceeds it
  loudly while scheduling noise does not.

## Relay-route disclosure

The relay leg is POST `/api/v0/impact/change-surface` via
foreign-bearer explicit relay: the W6 scan lane re-pointed it there
when it terminated the last code POST. Both legs do exactly one
server-side fetch, so the 1.50 bound stays a real pathology check.

## Numbers (local, 2026-11-27)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Replay POST | 50625 | 50566 | 1.00 | ≤ 1.50 |
| Reopen POST | 51766 | 50566 | 1.02 | ≤ 1.50 |
| Refinalize POST | 50401 | 50566 | 1.00 | ≤ 1.50 |
| Recover-generations POST | 50004 | 50566 | 0.99 | ≤ 1.50 |

Within budget on all four legs: the lane adds the explicit route
match and one authorizer call around the same single fetch. Absolute
deployment numbers against a pinned core image ride flip prep (D11);
this gate runs hermetically on every PR and in CI. Script wall time on
this host: ~4s.
