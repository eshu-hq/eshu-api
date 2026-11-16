# W5 entity/impact/story/packet latency

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#97`, W5
entity/impact/story/packet. The terminated reader does the same single
server-side fetch as the generic relay plus the wave gate and the
family authorizer. The stub core answers from memory — the numbers
isolate dest-side cost, not core or network time.

## Measured (2026-11-16, `bash scripts/measure-w5-latency.sh`, Wave-3)

- `BenchmarkEntityTerminatedRead/entities-resolve`: ~94 us/op vs
  `BenchmarkImpactRelayRead/hold-change-surface`: ~92 us/op — ratio 1.02.
- `BenchmarkEntityTerminatedRead/workloads-wl_1-story`: ~88 us/op vs
  `BenchmarkImpactRelayRead/hold-change-surface-investigate`: ~93 us/op — ratio 0.95.
- `BenchmarkImpactTerminatedRead/trace-deployment-chain`: ~94 us/op
  vs `post-relay`: ~92 us/op — ratio 1.02.
- `BenchmarkStoryTerminatedRead/story-get`: ~88 us/op vs
  `hold-change-surface-investigate`: ~94 us/op — ratio 0.93.
- `BenchmarkPacketTerminatedRead/packet-get`: ~90 us/op vs `hold-change-surface-investigate`:
  ~94 us/op — ratio 0.96.
- `BenchmarkImpactRelayRead/hold-change-surface`: ~92 us/op vs
  `post-relay`: ~92 us/op — ratio 1.00.
- `BenchmarkImpactRelayRead/hold-change-surface-investigate`:
  ~93 us/op vs `post-relay`: ~92 us/op — ratio 1.01.

Wave-3 removed the last shared-key relay legs, so the comparators ride
the explicit relay lane via foreign-bearer callers (same single-fetch
shape): entity and story GETs compare against the change-surface
relay legs, terminated legs against post-relay. Ratios 0.93–1.02,
inside the 1.50 budget (`scripts/measure-w5-latency.sh`, wired into
`.github/workflows/read-latency.yml`).

The terminated legs ride at the relay cost on the same single-fetch
shape: no latency regression from the gate plus authorizer. Absolute
deployment numbers ride the pinned-core-image leg at flip prep.
