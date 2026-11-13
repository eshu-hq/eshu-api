# Dead-code termination latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#124`,
dead-code 3. The terminated reader does the same single server-side
fetch as the generic relay plus the wave gate and the family
authorizer. The stub core answers from memory — the numbers isolate
dest-side cost, not core or network time.

## Measured (2026-11-13, `bash scripts/measure-dead-code-latency.sh`)

- `BenchmarkDeadCodeTerminatedRead/scan-post`: ~328 us/op.
- `BenchmarkDeadCodeTerminatedRead/cross-repo-post`: ~369 us/op.
- `BenchmarkDeadCodeTerminatedRead/investigate-post`: ~366 us/op.
- `BenchmarkDeadCodeRelayRead/post-relay`: ~340 us/op.

Ratios 0.96–1.09, inside the 1.50 budget
(`scripts/measure-dead-code-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same single-fetch
shape: no latency regression from the gate plus authorizer. Absolute
deployment numbers ride the pinned-core-image leg at flip prep.
