# Code search W4 latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#113`, W4.
The terminated reader does the same single server-side fetch as the
generic relay plus the wave gate and the family authorizer. The stub
core answers from memory — the numbers isolate dest-side cost, not
core or network time.

## Measured (2026-11-12, `bash scripts/measure-code-search-latency.sh`)

- `BenchmarkCodeSearchTerminatedPOST/code-search`: ~93 µs/op.
- `BenchmarkCodeSearchTerminatedPOST/symbols-search`: ~94 µs/op.
- `BenchmarkCodeSearchRelayPOST/bundles-relay`: ~94 µs/op.

Ratios 1.00–1.01, inside the 1.50 budget
(`scripts/measure-code-search-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same
single-fetch shape: no latency regression from the gate plus
authorizer. Absolute deployment numbers ride the pinned-core-image
leg at flip prep.
