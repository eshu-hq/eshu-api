# W8a satellite termination

Satellite termination of the 16 W8a reads (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#100`, D13 tracker
`eshu-hq/eshu#10491`). The wave table lives in `api/w8a_wave.go`;
the serving contract in `api/w8a_terminate.go`; the router
registration in `api/router.go` (`registerW8aWave`, ahead of the
generic proxy). The Compare POST (`POST
/api/v0/compare/environments`) is NOT in this wave: it terminated in
its gap-04 lane and W8a only re-verifies it (see
`contracts/compare-termination.md`).

## Pins

Seven deliberate core pins, each recorded byte-identical at the core
HEAD `eshu-hq/eshu#e504e328a7` and re-verified in-lane with zero
drift since its merge (no commits touching any pin file between its
merge and `e504e328a7`):

- `SecretsIAMReadAdmission v0`, recorded in
  `contracts/secretsiam-read-admission.md` at the core merge
  (`eshu-hq/eshu#10537`): covers the 5 secrets/IAM rows. The S3
  external-principal grant leg stays OUT and unpinned; core blends
  it on the forwarded call from the handler's own `GrantPosture`
  handle (nil-safe: the section omits instead of failing the
  rollup).
- `StatusGovernanceReadAdmission v0`, recorded in
  `contracts/status-governance-read-admission.md` at the core merge
  (`eshu-hq/eshu#10538`): covers the governance row's Summary leg;
  the snapshot leg rides the already-recorded `StatusReadAdmission
  v0`.
- `StatusHostedReadinessReadAdmission v0`, recorded in
  `contracts/status-hosted-readiness-read-admission.md` at the core
  merge (`eshu-hq/eshu#10538`): covers the hosted-readiness row's
  graph legs (repository count, workload-leftovers detection); the
  snapshot leg rides `StatusReadAdmission v0`. The `GraphBackend`
  selector and the Detect algorithm stay config/callee, OUT.
- `IaCReadAdmission v0` (`contracts/iac-read-admission.md`,
  `eshu-hq/eshu#10455`): covers the 5 IaC rows. The unused pin
  methods on these routes (the `Run`/`Summary`/Reachability trio
  legs the routes do not admit through) stay unconsumed.
- `MetricsReadAdmission v0`
  (`contracts/metrics-read-admission.md`, `eshu-hq/eshu#10440`):
  covers the metrics row; the code-graph-metrics POST stays outside
  the pin.
- `SemanticSearchReadAdmission v0`
  (`contracts/semantic-search-read-admission.md`,
  `eshu-hq/eshu#10443`): covers the semantic row.
- `TerraformDriftReadAdmission v0`
  (`contracts/terraform-drift-read-admission.md`,
  `eshu-hq/eshu#10442`): covers the terraform findings row.

The lane consumes only these narrow pins and cites no shared-port
pin; no other family may cite these pins as coverage. Recording a
pin whole never authorizes terminating routes beyond the 16: `POST
/api/v0/cloud/runtime-drift/findings` and the IaC legs outside the
W3 IaC-6 slice, the index-status Neo4j-count legs, the
impact packet, the suppressions mutation, and the
code-graph-metrics POST stay relay (or on their tier-C lane) with
per-route reasons, pinned by
`TestW8aExcludedRoutesStayRelay`. (`GET /api/v0/iac/resources`
terminated natively in the W3 lane on its supplemental SearchActive
pin — see `contracts/iac-read-admission.md`.) No direct-store serving (no
`*WithReadStore`/`*WithQueryer` satisfier) rides this lane: the
transport is the proven forward model only (dest caller gate +
family authorizer + bounded no-retry/no-redirect `forwardToCore`,
credential/body/negotiation intact, core re-judges authz atomically,
core bytes unmodified).

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeW8aSharedKey`) runs first, then
  the call crosses the versioned port transport (`forwardToCore` on
  the native lane) with method, path, query, body, negotiation
  headers, and credential intact; core bytes come back unmodified.
  Byte parity with the generic relay is pinned by
  `TestW8aGetTerminatedMatchesRelayByteForByte` and
  `TestW8aPostTerminatedMatchesRelayByteForByte` across the
  fragment-documented outcomes (200, 400, 403, 404, fixed-message
  500, core 503 permanent, retryable 503 with `Retry-After`, 504,
  unfollowed redirect) in both negotiated envelopes, plus
  `TestW8aRouteSpecificParity` for the route-specific legs
  (posture-summary grant section present AND graph-read-error
  503/504, semantic `pending_search_vector` cause, terraform
  double-guard page AND capability 403, metrics allow-list 403,
  IaC scoped-empty pages, governance Summary 503 shape,
  hosted-readiness count AND unknown-backend legs,
  answer-narration override on AND off).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, or 500 for these classes
  (`TestW8aWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: every route
  stays inside RequireAuth, and keyless callers keep the 401.
- Exactly one family authorizer allow per terminated call
  (`TestW8aAuthorizerCountersPinned`); the deny branch stays
  observable (`TestW8aDenyBranchShapePinned` pins its 403 shape with
  zero core contact). Exactly one core call per terminated call —
  no retry, no second fetch (the parity and forwarding proofs assert
  single upstream hits; the latency gate bounds the overhead at
  1.50x of the relay).

## D0 / wiring evidence

- `TestW8aWaveFrozen`: the wave is exactly the 16 pinned W8a
  reads; nothing else may enter it silently.
- `TestW8aWaveAheadOfRelay` (+ slash spellings): shared-key calls
  file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact.
- `TestW8aTerminated_ForwardsCallerBytes`: method, path, query,
  negotiation, credential, and body cross intact with the
  correlation ID echoed.
- `TestW8aAuthorizerCountersPinned`: one allow judgment on the
  called family per terminated call, none on neighbors.
- Latency: `BenchmarkW8aTerminatedRead` per wave family vs
  `BenchmarkW8aRelayRead`, gated by
  `scripts/measure-w8a-latency.sh` (fail past 1.50x).
- Spec scope: 16 newly mounted operations, 16 new disclosed
  absences (mounted 180 -> 196, absent 11 -> 27, pinned by
  `TestSpecScope_Frozen`).
- Core-pin frozen tests enforced core-side at their merges and
  re-gated satellite-side at `e504e328a7` by `check-parity.sh`
  (method-set + wire-shapes per family); the recorded-pin contract
  tests ride the same parity gate.

## Spec posture

The 16 W8a routes mount natively, but their Decision 5(c)
fragments stay core-side for this lane: each fragment is shaped as
a mid-list entry of its own assembled document, so a byte-identical
satellite port cannot concatenate mid-list — and a re-shaped
satellite subset would break the verbatim-port rule
`scripts/check-parity.sh` gates. The routes are therefore disclosed
spec absences (the same posture the Compare gap-04 lane records),
pinned by `TestSpec_W8aStaysDisclosedAbsent`. A follow-up
spec-port lane claims the keys when core re-shapes a fragment or
the lane ports it entry-shaped; until then the served document is
unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerW8aWave(mux)` call in
`routes()` (or this lane's files) and re-add the 16 routes to the
relay disclosures; the routes return to the generic relay behind
RequireAuth. No data migrates and no other family moves.
