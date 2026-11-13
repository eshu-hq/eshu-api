# Divergence relay-leg termination

Divergence 3, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#125`). `POST /api/v0/code/divergence/findings`, `POST
/api/v0/code/divergence/investigate`, and `POST
/api/v0/code/divergence/report` terminate dest-side inside the
divergence wave gate, ahead of the generic proxy.

## Pin

`CodeDivergenceReadAdmission v0`, recorded in
`contracts/code-divergence-read-admission.md` at the divergence mint
(`d584e38`, the verified core merge of `eshu-hq/eshu#10650`). The lane
consumes only this narrow pin: the five content reads (group stats,
members, members-by-entity, drifted stats, drifted rows) plus the
bounded graph traversal (`Run`) behind the wrapper-bypass and
convention-outlier tracks — the exact 6 re-declared methods, with
`RunSingle`, search, list, file-content, ledger/summary/snapshot
surface, and every other `GraphQuery`/content-store leg sealed OUT.
The lane cites no other pin, and no other family may cite this pin as
coverage. Citing `CodeGraphReadAdmission v0`,
`CodeSearchReadAdmission v0`, or `ContentSearchReadAdmission v0` as
coverage here is void: those pins seal the divergence graph and
content legs out explicitly. There is deliberately no `WithReadStore`
constructor on this pin: dest serves the reads through the existing
graph reader plus the guarded Postgres content reader, the same pair
`cmd/api` wiring builds today; any other backend pairing needs a new
arbiter ruling. The grant checks, the capability short-circuit, the
repository selector, the fixed failure messages, suppression assembly,
and `QueryProfile` stay handler behavior, not port shape — core judges
them on the forwarded call.

Scope note: the merged pin file comments scope the W-wave consumer to
findings ONLY. The frozen 6-method set is shared by all three routes
(investigate/report use strict subsets — see
`contracts/code-divergence-read-admission.md`), so this lane cites the
pin for all three wave rows on the shared method set with that
disclosure; arbiter reconcile of the comment scope plus box-1 APPROVED
remain the promotion gates.

## Census (per route per method)

| Method | `POST /api/v0/code/divergence/findings` | `POST /api/v0/code/divergence/investigate` | `POST /api/v0/code/divergence/report` |
|---|---|---|---|
| `DivergenceGroupStats` | phase-one stat scan | — (OUT by construction) | per-kind stat scan |
| `DivergenceMembers` | window hydration | point lookup | per-kind hydration |
| `DivergenceMembersByEntityID` | wrapper + outlier hydration | wrapper-target + outlier-cohort hydration | wrapper + outlier hydration |
| `DriftedFindingStats` | drifted stat stream | — (OUT by construction) | drifted counts |
| `DriftedFindingRows` | drifted window hydration | drifted point lookup | drifted hydration |
| `Run` | wrapper evidence + outlier sweep | wrapper qualification + outlier selection | wrapper qualification + whole outlier sweep |

Reachability notes, recorded not silently assumed: the investigate leg
runs no stat scan by construction; the report leg caps wrapper
qualification at 200 families and hydration at 500 groups per kind;
`RunSingle` is OUT everywhere by construction.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeDivergenceSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestDivergenceTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 findings/empty/truncated
  pages, 200 finding-plus-steps, 200 rollup, 404 not-found,
  400 validation/selector, 403 grant-denied, 501 capability,
  fixed-message 500s, backend-unavailable 503, retryable 503 with
  `Retry-After`, gateway 504) in both negotiated shapes, for all three
  routes and every driven body shape (all-families, repo-scoped,
  single-family, point lookup, target lookup, default rollup,
  top-capped rollup).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer [REDACTED]` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestDivergenceWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestDivergenceWaveFrozen`: the wave is exactly the three pinned
  divergence POSTs; nothing else may enter it silently. The table is a
  NEW wave, never an extension of `codeGraphWave`: the G-a pin covers
  its own 4 rows only. Consuming this pin for the search, list,
  file-content, or graph-entity surfaces is a NO-GO (arbiter
  SUPPLEMENTAL-PIN-OWED ruling).
- `TestDivergenceWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact across all-families,
  repo-scoped, single-family, point-lookup, target-lookup, and rollup
  shapes. Removing the `registerDivergenceWave` call returns the routes
  to the generic relay, which this test pins as RED (relay row, no
  native row).
- `TestDivergenceAuthorizerCountersPinned`: one allow judgment on the
  `code_divergence` family per terminated call; the deny branch stays
  observable (`TestDivergenceDenyBranchShapePinned` pins its 403 shape
  with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe across
  the three legs (one forward per caller).
- Latency: `BenchmarkDivergenceTerminatedRead` (one leg per wave
  route) vs `BenchmarkDivergenceRelayRead` (still-relayed POST, on the
  generic proxy); every leg does one server-side fetch (see
  `docs/divergence-latency.md` for the measured run). The relay leg
  rides `POST /api/v0/code/bundles` — no existing latency leg covers
  a terminated route, so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragment stays core-side for this lane; mounted 213 -> 216,
  disclosed absences 39 -> 42 (rebase note: owner PR #132 moves the
  same base counts for the dead-code 3 — the first lane to merge shifts
  this lane's counts to 216 -> 219 / 42 -> 45 on rebase).
- Decision 2: NOT OWED in this lane. There is no tier-C mutation
  survivor to split: all three wave rows are reads behind the same
  6-method pin, and the frozen method-set test holds the
  ledger/summary/snapshot exclusions core-side. No parity split is
  proven because none is owed.

## Spec posture

The three divergence POSTs mount natively, but their Decision 5(c)
fragment stays core-side for this lane. All three entries live in one
core file — `go/internal/query/openapi/paths/code/divergence.go`
(the `/code/divergence/findings`, `/code/divergence/investigate`,
and `/code/divergence/report` entries). Porting an entry without its
referenced blocks would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, Wave-7 G-a, dead-code, and Wave-4 Drift
relay-leg lanes, the routes are disclosed spec absences (pinned by
`TestSpec_DivergenceStaysDisclosedAbsent`), not a port. A follow-up
spec-port lane claims the three keys when it ports the entries
entry-shaped with their referenced schemas; until then the served
document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerDivergenceWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the three divergence POSTs return to the generic relay
behind RequireAuth. The spec absences stay disclosed either way (the
routes leave the mounted set with the lane); no data migrates, and no
other family moves.
