# Code-divergence termination latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#125`,
divergence 3. The terminated reader does the same single
server-side fetch as the generic relay plus the wave gate and the
family authorizer. The stub core answers from memory — the numbers
isolate dest-side cost, not core or network time.

## Measured (2026-11-13, `bash scripts/measure-code-divergence-latency.sh`)

- `BenchmarkCodeDivergenceTerminatedRead/findings-post`: ~103 us/op.
- `BenchmarkCodeDivergenceTerminatedRead/investigate-post`: ~94 us/op.
- `BenchmarkCodeDivergenceTerminatedRead/report-post`: ~94 us/op.
- `BenchmarkCodeDivergenceRelayRead/post-relay`: ~104 us/op.

Ratios 0.90–0.99, inside the 1.50 budget
(`scripts/measure-code-divergence-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same
single-fetch shape: no latency regression from the gate plus
authorizer. Absolute deployment numbers ride the pinned-core-image
leg at flip prep.
