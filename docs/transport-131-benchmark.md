# Relay transport pool benchmark — eshu-hq/eshu-api#131

Measurement-first note for the 2-idle-connection theory. Stopping rule
from the issue: no transport change lands unless this benchmark proves a
material cost.

## Setup

- Harness: `api/dispatch_transport_test.go` (beside `dispatch_test.go`).
- Stub core: 50ms handler, 200 `{"ok":true}`.
- Load: 64 concurrent requests per burst through the production
  `Handler` (`GET /api/v0/repositories`, generic relay leg).
- Connection counting: `ConnState`/`StateNew` on the stub core server.
- Legs: default client (`dispatchClient()`, nil Transport → Go default,
  `MaxIdleConnsPerHost = 2`) vs tuned client (same timeout + redirect
  rule, `MaxIdleConnsPerHost = 64`).
- Host: linux/arm64, loopback, `go1.26.11`. Rerun with:
  `go test ./api/ -run=NONE -bench=RelayTransportPool -benchtime=8x -count=3`

## Numbers

Calibration test (`TestRelayTransportPoolCalibration`, `-race` clean):
burst 1 opens 64 connections, burst 2 opens 62 new (only the 2-idle
pool survives). Reproduces the issue's probe exactly (64 then 62).

Benchmark, steady state (1 warmup burst excluded), 3 × 8 bursts per leg:

| leg | new conns / burst | p50 (ms) | p99 (ms) |
|---|---|---|---|
| default pool 2 | 62.00, 62.00, 62.00 | 51.54, 51.29, 51.43 | 52.89, 52.30, 52.19 |
| tuned pool 64 | 0, 0, 0 | 50.56, 50.72, 50.53 | 51.39, 51.65, 51.13 |

## Reading

- Pool churn is proven and large: 62 vs 0 new connections per 64-burst.
- Latency cost is small and consistent in direction, 3/3 runs:
  about +0.8ms p50 and +1.0ms p99 on a 50ms handler (~2%).
- A ~1ms loopback delta does not explain relay 503 flakes: 503s come
  from refused/dial-timeout paths, not from 1ms of setup. This gives no
  measured basis to implicate the pool in #94 (closed via hygiene fix);
  per the issue, #94 stays closed.

## Recommendation

Close #131 with these numbers; land no transport change. If an arbiter
judges 62 fresh handshakes per burst worth fixing for core-side accept
load (not client latency), the fix is `tunedDispatchClient`-shaped: an
explicit cloned Transport with a documented pool size — plus the
keep-alive-reuse, core-restart, and `ForceAttemptHTTP2` tests the issue
requires. That work is not started here.
