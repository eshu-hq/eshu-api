# W5 entity/impact/story/packet latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#97`, W5
entity/impact/story/packet. The terminated reader does the same single
server-side fetch as the generic relay plus the wave gate and the
family authorizer. The stub core answers from memory — the numbers
isolate dest-side cost, not core or network time.

## Measured (2026-11-16, `bash scripts/measure-w5-latency.sh`, Wave-2)

- `BenchmarkEntityTerminatedRead/entities-resolve`: ~93 us/op vs
  `BenchmarkEntityRelayRead/post-relay`: ~91 us/op — ratio 1.02.
- `BenchmarkEntityTerminatedRead/workloads-wl_1-story`: ~88 us/op vs
  `post-relay`: ~92 us/op — ratio 0.95.
- `BenchmarkImpactTerminatedRead/trace-deployment-chain`: ~95 us/op
  vs `post-relay`: ~91 us/op — ratio 1.05.
- `BenchmarkStoryTerminatedRead/story-get`: ~87 us/op vs
  `post-relay`: ~92 us/op — ratio 0.94.
- `BenchmarkPacketTerminatedRead/packet-get`: ~89 us/op vs `post-relay`:
  ~92 us/op — ratio 0.96.
- `BenchmarkImpactRelayRead/hold-change-surface`: ~93 us/op vs
  `post-relay`: ~91 us/op — ratio 1.03.
- `BenchmarkImpactRelayRead/hold-change-surface-investigate`:
  ~94 us/op vs `post-relay`: ~91 us/op — ratio 1.04.

Wave-2 moved the entity comparator off the deleted hold-get-relay and
get-relay legs: with no entity GET left on relay, the entity GET
comparators share the post-relay reference (same single-fetch shape).
Ratios 0.94–1.05, inside the 1.50 budget
(`scripts/measure-w5-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same single-fetch
shape: no latency regression from the gate plus authorizer. Absolute
deployment numbers ride the pinned-core-image leg at flip prep.
