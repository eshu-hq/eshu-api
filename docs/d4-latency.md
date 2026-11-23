# D4 read-latency evidence

D4 latency record for the Native-next reads termination lane (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The census assigns
the satellite this gate: core keeps the contract-registry and
static-contract gates; this repo proves the terminated reads are not
slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before leg: the explicit relay (`BenchmarkD4RelayRead`),
  foreign-bearer GET (`/api/v0/dependencies`) + POST
  (`/api/v0/impact/change-surface`) on their wave-mounted routes
  through the full production chain. The legs compare different routes
  by necessity (the D4 paths now terminate, and both relay routes
  have since terminated natively for the shared key — dependencies in
  W8b-narrow, change-surface in the impact lane's Wave-3, so the
  shared-key shape would run terminated-vs-terminated);
  both do exactly one server-side fetch, so the 1.50 bound stays a real
  pathology check.
- After leg: the terminated D4 reads (`BenchmarkD4TerminatedRead`),
  shared-key identity GET (`/api/v0/repositories`) + infra query POST
  (`/api/v0/infra/resources/search`) through the full production chain
  with the wave gate and family authorizer.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d4-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when terminated exceeds relay by
  more than 50% on any leg. The threshold is generous on purpose: both
  legs do exactly one server-side fetch, so any real pathology (a
  double fetch, a retry, a body re-read) exceeds it loudly while
  scheduling noise does not.

## Relay-route disclosure

The D4 wave terminated the relay legs the D2/D3 lanes used
(`GET /api/v0/repositories`, `POST /api/v0/repositories/search`), so
all three lanes relay via foreign-bearer explicit relay on the
wave-mounted routes `GET /api/v0/dependencies` (Dependencies family)
and `POST /api/v0/impact/change-surface` (Impact family) — both have
since terminated natively for the shared key (dependencies in
W8b-narrow, change-surface in the impact lane's Wave-3), so the
shared-key shape would run terminated-vs-terminated. The D2/D3
benchmark relay legs and their measure scripts moved to the same
routes in this lane with the move disclosed in each script header,
so every lane gates terminated-vs-relay again instead of
terminated-vs-terminated. The D2/D3 doc files keep their historical
numbers as recorded; CI enforces the live ratio.

## Numbers (local, 2026-11-06; relay leg re-pointed 2026-11-15)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Terminated (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| GET | 82421 | 82632 | 1.00 | ≤ 1.50 |
| POST | 86116 | 87401 | 0.99 | ≤ 1.50 |

Re-measured 2026-11-15 after the W6 scan lane re-pointed the relay
leg from the terminated code bundles POST to the impact
change-surface POST (then the relay leg; change-surface has since
terminated natively in the impact lane's Wave-3, and the leg now
rides foreign-bearer explicit relay).

No measurable overhead: the lane adds the gate branch and one
authorizer call around the same single fetch. A repeat run measured
GET 0.99 and POST 0.91 — the same shape. Absolute deployment numbers
against a pinned core image ride flip prep (D11); this gate runs
hermetically on every PR and in CI
(`.github/workflows/read-latency.yml`).
