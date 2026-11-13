# Drift Wave-4 latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#117`, Wave-4
Drift. The terminated reader does the same single server-side fetch
as the generic relay plus the wave gate and the family authorizer.
The stub core answers from memory — the numbers isolate dest-side
cost, not core or network time.

## Measured (2026-11-13, `bash scripts/measure-drift-latency.sh`)

- `BenchmarkDriftTerminatedRead/findings-post`: ~90 us/op.
- `BenchmarkDriftRelayRead/post-relay`: ~91 us/op.
- `BenchmarkDriftTerminatedRead/packet-get`: ~86 us/op.
- `BenchmarkDriftRelayRead/get-relay`: ~86 us/op.

Ratios 1.00–1.00, inside the 1.50 budget
(`scripts/measure-drift-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same
single-fetch shape: no latency regression from the gate plus
authorizer. Absolute deployment numbers ride the pinned-core-image
leg at flip prep.
