# Code-graph Wave-7 G-a latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#121`, Wave-7
G-a. The terminated reader does the same single server-side fetch
as the generic relay plus the wave gate and the family authorizer.
The stub core answers from memory — the numbers isolate dest-side
cost, not core or network time.

## Measured (2026-11-13, `bash scripts/measure-code-graph-latency.sh`)

- `BenchmarkCodeGraphTerminatedRead/call-chain-post`: ~95 us/op.
- `BenchmarkCodeGraphTerminatedRead/compare-post`: ~95 us/op.
- `BenchmarkCodeGraphTerminatedRead/metrics-post`: ~94 us/op.
- `BenchmarkCodeGraphTerminatedRead/callers-post`: ~94 us/op.
- `BenchmarkCodeGraphRelayRead/post-relay`: ~108 us/op.

Ratios 0.87–0.88, inside the 1.50 budget
(`scripts/measure-code-graph-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same
single-fetch shape: no latency regression from the gate plus
authorizer. Absolute deployment numbers ride the pinned-core-image
leg at flip prep.
