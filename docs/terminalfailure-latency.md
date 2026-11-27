# Terminal-failure clone-latency evidence

Terminal-failure latency record for the collector-failure
acknowledgement tier C-a clone lane (Refs `eshu-hq/eshu#9669`, part of
`eshu-hq/eshu#6707`). The census assigns the satellite this gate: core
keeps the contract-registry and static-contract gates; this repo proves
the terminal-failure tier-C clone is not slower than the relay it
replaces.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the explicit relay
  (`BenchmarkTerminalFailureRelayWrite`), foreign-bearer POST
  `/api/v0/impact/change-surface` on its wave-mounted route through
  the full production chain. The change-surface route has since
  terminated natively in the impact lane's Wave-3 (so the shared-key
  shape would run terminated-vs-terminated); the Compare POST it
  replaces terminated in its gap-04 lane.
- After leg: the terminal-failure clone
  (`BenchmarkTerminalFailureNativeWrite`), shared-key acknowledge
  POST through the full production chain with the explicit route
  match and family authorizer. The leg forwards the representative
  operator JSON body; the request is rebuilt per iteration with Body
  and ContentLength reset because the server consumes the body.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-terminalfailure-latency.sh` runs each leg
  `-count=5`, takes the median ns/op, and fails when the clone leg
  exceeds relay by more than 50%. The threshold is generous on
  purpose: both legs do exactly one server-side fetch, so any real
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
| Acknowledge POST | 53170 | 51068 | 1.04 | ≤ 1.50 |

Within budget: the lane adds the explicit route match and one
authorizer call around the same single fetch. Absolute deployment
numbers against a pinned core image ride flip prep (D11); this gate
runs hermetically on every PR and in CI. Script wall time on this
host: ~2s.
