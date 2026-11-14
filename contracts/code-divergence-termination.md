# Code-divergence relay-leg termination

Divergence 3, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#125`). `POST /api/v0/code/divergence/findings`,
`POST /api/v0/code/divergence/investigate`, and `POST
/api/v0/code/divergence/report` terminate dest-side inside the
divergence wave gate, ahead of the generic proxy.

## Pin

`CodeDivergenceReadAdmission v0`, recorded in
`contracts/code-divergence-read-admission.md` at the divergence mint
(`d584e38`, the verified core merge of `eshu-hq/eshu#10650`). The
lane consumes only this narrow pin: the five staying content reads
(`DivergenceGroupStats`, `DivergenceMembers`,
`DivergenceMembersByEntityID`, `DriftedFindingStats`,
`DriftedFindingRows`) plus the bounded graph traversal (`Run`) the
wrapper-bypass and convention-outlier tracks read through — the
exact 6 re-declared methods, with `RunSingle`, point/list reads,
file-content legs, ledger/summary/snapshot surface, and every other
`GraphQuery` leg sealed OUT. The lane cites no other pin, and no
other family may cite this pin as coverage.

## Census (per route per method)

| Method | `POST /api/v0/code/divergence/findings` | `POST /api/v0/code/divergence/investigate` | `POST /api/v0/code/divergence/report` |
|---|---|---|---|
| `DivergenceGroupStats` | phase-one rank over every selected kind | — (point lookup, OUT by construction) | per-kind scan windows; wrapper nominators |
| `DivergenceMembers` | window hydration, incl. wrapper nominations through the exact column | exact/renamed point lookup by fingerprint | content-kind windows plus wrapper nominations |
| `DivergenceMembersByEntityID` | wrapper qualification and outlier cohort hydration | wrapper-target and outlier-cohort point lookups | both graph tracks |
| `DriftedFindingStats` | drifted stat stream | — (fact row read directly, OUT by construction) | drifted counts |
| `DriftedFindingRows` | drifted window hydration | drifted point lookup by writer finding id | drifted content kind |
| `Run` | wrapper-bypass one-hop qualification and convention-outlier cohort sweep | both graph tracks behind the point lookup | both graph tracks behind the rollup (timeouts degrade to counted suppressions) |

Reachability notes, recorded not silently assumed: the investigate
leg is a point lookup and runs no stat stream; the drifted point
lookup reads the fact row directly; the report rollup degrades a
timed-out graph track to a counted suppression with its kind
truncated instead of failing the other four kinds, while the
findings page keeps the loud deadline.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeDivergenceSharedKey`) runs
  first, then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeDivergenceTerminatedMatchesRelayByteForByte` across the
  documented outcomes (200 found pages and scoped-empty zero pages,
  200 addressed findings with bounded next steps, 200 rollups with
  counts and top slices, 400
  validation/selector denials, 403 grant-denied, 404 unknown
  fingerprint, capability 501s, fixed-message 500s,
  configuration-state 503, retryable 503 with `Retry-After`, gateway
  504) in both negotiated shapes, for all three routes and every
  driven body shape (kind-selected and paged findings pages,
  exact/drifted/qualified point lookups, default and top-bounded
  rollups).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer eshu_sc_...`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeDivergenceWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestCodeDivergenceWaveFrozen`: the wave is exactly the three
  pinned divergence POSTs; nothing else may enter it silently. The
  table is a NEW wave, never an extension of `codeGraphWave`: the
  graph pin covers its own 4 rows only. Consuming this pin for any
  other route is a NO-GO (arbiter supplemental ruling).
- `TestCodeDivergenceWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  kind-selected, paged, point-lookup, and rollup shapes. Removing
  the `registerCodeDivergenceWave` call returns the routes to the
  generic relay, which this test pins as RED (relay row, no native
  row).
- `TestCodeDivergenceAuthorizerCountersPinned`: one allow judgment
  on the `code_divergence` family per terminated call; the deny
  branch stays observable
  (`TestCodeDivergenceDenyBranchShapePinned` pins its 403 shape with
  zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation without retry, and a 12-caller parallel
  probe across the three legs (one forward per caller).
- Latency: `BenchmarkCodeDivergenceTerminatedRead` (one leg per
  wave route) vs `BenchmarkCodeDivergenceRelayRead` (still-relayed
  POST, on the generic proxy); every leg does one server-side fetch
  (see `docs/code-divergence-latency.md` for the measured run). The
  relay leg rides `POST /api/v0/code/bundles` — no existing latency
  leg covers a terminated route, so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 213 -> 216,
  disclosed absences 39 -> 42.

## Spec posture

The three divergence POSTs mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in one core
file — `go/internal/query/openapi/paths/code/divergence.go` (the
`/code/divergence/findings`, `/investigate`, and `/report`
entries). Porting an entry without its referenced schemas would
break the verbatim-port rule `scripts/check-parity.sh` gates — so,
like the gap-04 Compare, W6a Code-flow, W4 search, G-a Code-graph,
and Wave-4 Drift relay-leg lanes, the routes are disclosed spec
absences (pinned by
`TestSpec_CodeDivergenceStaysDisclosedAbsent`), not a port. A
follow-up spec-port lane claims the three keys when it ports the
entries entry-shaped with their referenced schemas at a pinned base;
until then the served document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeDivergenceWave(mux)`
call in `routes()` (or the wave/gate/terminate/test files with this
contract), and the three divergence POSTs return to the generic
relay behind RequireAuth. The spec absences stay disclosed either
way (the routes leave the mounted set with the lane); no data
migrates, and no other family moves.
