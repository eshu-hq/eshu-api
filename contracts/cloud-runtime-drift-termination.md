# Cloud-runtime-drift relay-leg termination

Wave-4 Drift 2, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#117`). `POST /api/v0/cloud/runtime-drift/findings`
and `GET /api/v0/investigations/drift/packet` terminate dest-side
inside the drift wave gate, ahead of the generic proxy.

## Pin

`CloudRuntimeDriftReadAdmission v0`, recorded at the W8b tier-1 mint
(`d81bd12802`, unwired, arbiter-approved per the W8b ruling). The lane
consumes only this narrow pin: the bounded active-finding page plus
the total active count for the same bounded filters — the exact 2
re-declared methods, with `Run`/`RunSingle`, content reads, snapshot,
ledger, stored-summary, management, inventory, selector, and
reachability legs sealed OUT. Citing `TerraformDriftReadAdmission v0`
or `IaCReadAdmission v0` as coverage for either route here is void:
neither re-declares the provider-aggregated `MultiCloudRuntimeDriftStore`
surface. The lane cites no other pin, and no other family may cite
this pin as coverage. There is deliberately no `WithReadStore`
constructor on this pin: dest serves the reads through the existing
Postgres adapter the way `cmd/api` wiring builds it today; any other
backend pairing needs a new arbiter ruling. The grant checks
(scoped-empty short-circuits, the direct scope grant comparison), the
fixed failure messages, the packet bounds, the truth envelope, and
`QueryProfile` stay handler behavior, not port shape — core judges
them on the forwarded call.

## Census (per route per method)

| Method | `POST /api/v0/cloud/runtime-drift/findings` | `GET /api/v0/investigations/drift/packet` |
|---|---|---|
| `ListActiveMultiCloudRuntimeDriftFindings` | bounded readback page | packet build (only store read on the leg) |
| `CountActiveMultiCloudRuntimeDriftFindings` | truncation total | — (no total on the packet; OUT by construction) |

Reachability notes, recorded not silently assumed: the packet leg
short-circuits before any store read on a scoped out-of-grant scope
(refusal packet, no store call) and on a missing scope or unknown
provider (contract error, no store call); the count read is reachable
only from the findings leg.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeDriftSharedKey`) runs first, then
  the call crosses the versioned port transport (`forwardToCore` on
  the native lane) with method, path, query, body, negotiation
  headers, and credential intact; core bytes come back unmodified.
  Byte parity with the generic relay is pinned by
  `TestDriftTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 found and scoped-empty zero
  page, 200 built and refusal packets, 400 validation/malformed,
  capability and read-model 501s, fixed-message 500s, retryable 503
  with `Retry-After`, gateway 504) in both negotiated shapes, for
  both routes and every driven body/query shape.
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, or 500 for these classes
  (`TestDriftWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestDriftWaveFrozen`: the wave is exactly the two pinned drift
  routes; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `w8aWave`: the W8a pins cover their
  own 16 rows only. Consuming this pin for any other route is a
  NO-GO (arbiter W8b ruling).
- `TestDriftWaveAheadOfRelay` (+ slash spellings): shared-key calls
  file their dispatch on the native lane ahead of the generic
  proxy, with request bodies and packet queries forwarded intact
  across the driven shapes. Removing the `registerDriftWave` call
  returns the routes to the generic relay, which this test pins as
  RED (relay row, no native row).
- `TestDriftAuthorizerCountersPinned`: one allow judgment per
  terminated call on its route family (`iac` for the findings POST,
  `evidence` for the packet GET); the deny branch stays observable
  (`TestDriftDenyBranchShapePinned` pins its 403 shape with zero
  core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe across
  both legs (one forward per caller).
- Latency: `BenchmarkDriftTerminatedRead` (one leg per wave route)
  vs `BenchmarkDriftRelayRead` (still-relayed GET and POST, on the
  generic proxy); both legs do one server-side fetch (see
  `docs/drift-latency.md` for the measured run). The relay legs ride
  routes no wave table covers (`GET /api/v0/dependencies` and `POST
  /api/v0/code/bundles`), so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 207 -> 209,
  disclosed absences 33 -> 35.

## Spec posture

The two drift routes mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in two core
files — `go/internal/query/openapi/paths/cloud/runtime_drift.go`
(the `/cloud/runtime-drift/findings` entry) and
`go/internal/query/openapi/paths/evidence/investigations.go` (the
`/investigations/drift/packet` entry) — so, like the gap-04 Compare,
W6a Code-flow, W4 search, and W8a relay-leg lanes, the routes are
disclosed spec absences (pinned by
`TestSpec_DriftStaysDisclosedAbsent`), not a port. A follow-up
spec-port lane claims both keys when it ports the two entries
entry-shaped with their referenced schemas; until then the served
document is unchanged by this lane. The drift-packet absence moves
here from `TestSpec_D4DisclosedOmissions` with this lane: it is no
longer relay-only, it is disclosed-absent.

## Rollback

Revert-to-relay: remove the `s.registerDriftWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the two drift routes return to the generic relay
behind RequireAuth. The spec absences stay disclosed either way (the
routes leave the mounted set with the lane); no data migrates, and no
other family moves.
