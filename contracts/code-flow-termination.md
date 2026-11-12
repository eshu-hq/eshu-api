# Code-flow W6a relay-leg termination

W6a Code-flow 4, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#98`). `POST /api/v0/code/flow/cfg-summary`,
`/pdg-summary`, `/reaching-def`, `/taint-path` terminate dest-side
inside the W6a wave gate, ahead of the generic proxy.

## Pin

`CodeFlowReadAdmission v0`, recorded in
`contracts/code-flow-read-admission.md` at the core merge
(`eshu-hq/eshu#10552`, source `e504e328a7`). The lane consumes only
this narrow pin: the four routes' bounded reads ride `ListCodeFlow`
(Kind `cfg_summary`, `pdg_summary`, `reaching_def`, `taint_path`).
The lane cites no shared-port pin, and no other family may cite this
pin as coverage. The grant check (plus the scoped repository
selector), the unsupported-language short-circuit, the fixed failure
messages (`"code-flow read failed"`, `"code-flow store is
unavailable"`), and QueryProfile stay handler behavior, not port
shape — core judges them on the forwarded call.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeFlowSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeFlowTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 bounded evidence, 400
  repo-required and malformed, 403 grant-denied, 501 unsupported
  profile, fixed-message 500, store-unavailable 503, graph-deadline
  504, retryable 503 with `Retry-After`, unfollowed redirect) in both
  negotiated shapes, for all four routes.
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, or 500 for these classes
  (`TestCodeFlowWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestCodeFlowWaveFrozen`: the wave is exactly the four pinned
  Code-flow POSTs; nothing else may enter it silently.
- `TestCodeFlowWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact.
- `TestCodeFlowAuthorizerCountersPinned`: one allow judgment on the
  `code_flow` family per terminated call; the deny branch stays
  observable (`TestCodeFlowDenyBranchShapePinned` pins its 403
  shape with zero core contact).
- Latency: `BenchmarkCodeFlowTerminatedPOST` vs
  `BenchmarkCodeFlowRelayPOST` bound the gate + authorizer overhead;
  both legs do one server-side fetch (see `docs/code-flow-latency.md`
  for the measured run).
- Spec scope: the routes are mounted native operations; mounted
  180 -> 184, disclosed absences 11 -> 15.

## Spec posture

The Code-flow POSTs mount natively, but their Decision 5(c) fragment
stays core-side for this lane: core shapes
`go/internal/query/openapi/paths/code/flow.go` as a four-key block
whose entry-shaped port belongs to the follow-up spec-port lane — and
a re-shaped satellite subset would break the verbatim-port rule
`scripts/check-parity.sh` gates. The routes are therefore disclosed
spec absences (same posture the setup and Compare lanes record),
pinned by `TestSpec_CodeFlowStaysDisclosedAbsent`. A follow-up
spec-port lane claims the keys (plus the `CodeFlowRequest` /
`CodeFlowResponse` schemas) when it ports the fragment entry-shaped;
until then the served document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeFlowWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the four Code-flow POSTs return to the generic relay
behind RequireAuth; the spec absences stay disclosed either way. The
document stays unchanged, no data migrates, and no other family
moves.
