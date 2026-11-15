# W5 entity/impact/story/packet latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#97`, W5
entity/impact/story/packet. The terminated reader does the same single
server-side fetch as the generic relay plus the wave gate and the
family authorizer. The stub core answers from memory — the numbers
isolate dest-side cost, not core or network time.

## Measured (2026-11-14, `bash scripts/measure-w5-latency.sh`)

- `BenchmarkEntityTerminatedRead/entities-resolve`: ~100 us/op vs
  `BenchmarkEntityRelayRead/post-relay`: ~98 us/op — ratio 1.02.
- `BenchmarkEntityTerminatedRead/workloads-wl_1-story`: ~91 us/op vs
  `BenchmarkEntityRelayRead/hold-get-relay`: ~91 us/op — ratio 1.01.
- `BenchmarkImpactTerminatedRead/trace-deployment-chain`: ~101 us/op
  vs `post-relay`: ~98 us/op — ratio 1.03.
- `BenchmarkStoryTerminatedRead/story-get`: ~91 us/op vs
  `BenchmarkEntityRelayRead/get-relay`: ~92 us/op — ratio 0.99.
- `BenchmarkPacketTerminatedRead/packet-get`: ~93 us/op vs `get-relay`:
  ~92 us/op — ratio 1.01.
- `BenchmarkImpactRelayRead/hold-change-surface`: ~99 us/op vs
  `post-relay`: ~98 us/op — ratio 1.01.
- `BenchmarkImpactRelayRead/hold-change-surface-investigate`:
  ~100 us/op vs `post-relay`: ~98 us/op — ratio 1.02.

Ratios 0.99–1.03, inside the 1.50 budget
(`scripts/measure-w5-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same single-fetch
shape: no latency regression from the gate plus authorizer. Absolute
deployment numbers ride the pinned-core-image leg at flip prep.
