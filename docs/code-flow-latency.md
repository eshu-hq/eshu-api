# Code-flow W6a latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#98`, W6a.
The terminated reader does the same single server-side fetch as the
generic relay plus the wave gate and the family authorizer. The stub
core answers from memory — the numbers isolate dest-side cost, not
core or network time.

## Measured (2026-11-12, `go test ./api/ -bench BenchmarkCodeFlow`)

- `BenchmarkCodeFlowTerminatedPOST/cfg-summary`: ~106–114 µs/op.
- `BenchmarkCodeFlowTerminatedPOST/taint-path`: ~108–112 µs/op.
- `BenchmarkCodeFlowRelayPOST/bundles-relay`: ~113–120 µs/op.

The terminated legs ride at or below the relay cost on the same
single-fetch shape: no latency regression from the gate plus
authorizer. Absolute deployment numbers ride the pinned-core-image
leg at flip prep.
